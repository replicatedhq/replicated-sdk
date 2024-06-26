package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_IsSamePayload(t *testing.T) {
	tests := []struct {
		name     string
		payloadA []byte
		payloadB []byte
		expect   bool
	}{
		{
			name:     "should return true for empty nil payloads",
			payloadA: nil,
			payloadB: nil,
			expect:   true,
		},
		{
			name:     "should return true despite one payload being nil",
			payloadA: []byte{},
			payloadB: nil,
			expect:   true,
		},
		{
			name:     "should tolerate empty non-nil byte payloads",
			payloadA: []byte{},
			payloadB: []byte{},
			expect:   true,
		},
		{
			name:     "should return false for different payloads where one payload is empty",
			payloadA: []byte{},
			payloadB: []byte(`{"numPeople": 10}`),
			expect:   false,
		},
		{
			name:     "should return false for different payloads",
			payloadA: []byte(`{"numProjects": 2000}`),
			payloadB: []byte(`{"numPeople": 10}`),
			expect:   false,
		},
		{
			name:     "should return true for same payloads",
			payloadA: []byte(`{"numPeople": 10}`),
			payloadB: []byte(`{"numPeople": 10}`),
			expect:   true,
		},
		{
			name:     "should return true for same payload despite differences in key ordering and spacing",
			payloadA: []byte(`{"numProjects": 2000, "numPeople":     10     }`),
			payloadB: []byte(`{"numPeople": 10, "numProjects":    2000}`),
			expect:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSamePayload(tt.payloadA, tt.payloadB)
			require.Equal(t, tt.expect, got)
		})
	}

}
func Test_cache(t *testing.T) {
	tests := []struct {
		name     string
		assertFn func(*testing.T, *cache)
	}{
		{
			name: "cache should be able to set and get KV pair with valid ttl",
			assertFn: func(t *testing.T, c *cache) {
				now := time.Now()

				entry := CacheEntry{
					RequestBody:  []byte("request body"),
					ResponseBody: []byte("response body"),
					StatusCode:   http.StatusOK,
				}
				c.Set("cache-key", entry, 1*time.Minute)

				cachedEntry, exists := c.Get("cache-key")

				require.True(t, exists)
				require.Equal(t, entry.RequestBody, cachedEntry.RequestBody)
				require.Equal(t, entry.ResponseBody, cachedEntry.ResponseBody)
				require.Equal(t, entry.StatusCode, cachedEntry.StatusCode)

				// TTL should be valid
				require.Equal(t, true, cachedEntry.Expiry.After(now))
			},
		},
		{
			name: "cache get should return false for non-existent key",
			assertFn: func(t *testing.T, c *cache) {
				_, exists := c.Get("cache-key-does-not-exist")
				require.False(t, exists)
			},
		},
		{
			name: "cache get should return false for expired cache entry",
			assertFn: func(t *testing.T, c *cache) {

				entry := CacheEntry{
					RequestBody:  []byte("request body"),
					ResponseBody: []byte("response body"),
					StatusCode:   http.StatusOK,
				}
				c.Set("cache-key", entry, 5*time.Millisecond)

				time.Sleep(10 * time.Millisecond)

				_, exists := c.Get("cache-key")

				require.False(t, exists)

			},
		},
		{
			name: "cache set should delete expired cache entries",
			assertFn: func(t *testing.T, c *cache) {

				entry := CacheEntry{
					RequestBody:  []byte("request body"),
					ResponseBody: []byte("response body"),
					StatusCode:   http.StatusOK,
				}

				c.Set("first-cache-key", entry, 10*time.Millisecond)
				_, exists := c.Get("first-cache-key")
				require.True(t, exists)

				time.Sleep(20 * time.Millisecond)

				c.Set("second-cache-key", entry, 1*time.Minute)
				_, exists = c.Get("first-cache-key")
				require.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCache()
			tt.assertFn(t, c)
		})
	}
}

func newTestRequest(method, url string, body []byte) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	return req, rec
}

func Test_CacheMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]interface{}{"message": "Hello, World!"})
	})

	duration := 1 * time.Minute
	cache := NewCache()
	cachedHandler := CacheMiddleware(cache, duration).Middleware(handler)

	/* First request should not be served from cache */
	req, recorder := newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT rate limited

	/* Second request should be served from cache since the payload it the same */
	req, recorder = newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "true", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should exist because the response is served from cache
	require.Equal(t, "true", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should exist because the response is rate limited

	/* Third request should not be served from cache since the payload is different */
	req, recorder = newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 1111}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT served from cache

}

func Test_CacheMiddleware_Expiry(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]interface{}{"message": "Hello, World!"})
	})

	duration := 100 * time.Millisecond
	cache := NewCache()
	cachedHandler := CacheMiddleware(cache, duration).Middleware(handler)

	/* First request should not be served from cache */
	req, recorder := newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT served from cache

	/* Second request should be served from cache since the payload it the same and under the expiry time */
	req, recorder = newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "true", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should exist because the response is served from cache
	require.Equal(t, "true", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should exist because the response is rate limited

	time.Sleep(110 * time.Millisecond)

	/* Third request should not be served from cache due to expiry */
	req, recorder = newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `{"message":"Hello, World!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT rate limited

}

func Test_CacheMiddleware_DoNotCacheErroredPayload(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Something went wrong!"})
	})

	duration := 1 * time.Minute
	cache := NewCache()
	cachedHandler := CacheMiddleware(cache, duration).Middleware(handler)

	/* First request should not be served from cache */
	req, recorder := newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, `{"error":"Something went wrong!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT served from cache

	/* Second request should not be served from cache - err'ed payloads are not cached */
	req, recorder = newTestRequest("POST", "/custom-metric", []byte(`{"data": {"numProjects": 2000}}`))
	cachedHandler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, `{"error":"Something went wrong!"}`, recorder.Body.String())
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Served-From-Cache")) // Header should NOT exist because the response is NOT served from cache
	require.Equal(t, "", recorder.Header().Get("X-Replicated-Rate-Limited"))      // Header should NOT exist because the response is NOT rate limited

}
