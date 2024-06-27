package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleOptionsRequest(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func EnforceMockAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !store.GetStore().IsDevLicense() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func RequireValidLicenseIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		licenseID := r.Header.Get("authorization")
		if licenseID == "" {
			response := types.ErrorResponse{Error: "missing authorization header"}
			JSON(w, http.StatusUnauthorized, response)
			return
		}

		if store.GetStore().GetLicense().Spec.LicenseID != licenseID {
			response := types.ErrorResponse{Error: "license ID is not valid"}
			JSON(w, http.StatusUnauthorized, response)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Code for the cache middleware
type CacheEntry struct {
	RequestBody  []byte
	ResponseBody []byte
	StatusCode   int
	Expiry       time.Time
}

type cache struct {
	store map[string]CacheEntry
	mu    sync.RWMutex
}

func NewCache() *cache {
	return &cache{
		store: map[string]CacheEntry{},
	}
}

func (c *cache) Get(key string) (CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.store[key]
	if !found || time.Now().After(entry.Expiry) {
		return CacheEntry{}, false
	}
	return entry, true
}

func (c *cache) Set(key string, entry CacheEntry, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up expired entries
	for k, v := range c.store {
		if time.Now().After(v.Expiry) {
			delete(c.store, k)
		}
	}

	entry.Expiry = time.Now().Add(duration)
	c.store[key] = entry
}

type responseRecorder struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	StatusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.StatusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}

func CacheMiddleware(next http.HandlerFunc, duration time.Duration) http.HandlerFunc {
	// Each handler has its own cache to reduce contention for the in-memory store
	cache := NewCache()

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error(errors.Wrap(err, "cache middleware - failed to read request body"))
			http.Error(w, "cache middleware: unable to read request body", http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		hash := sha256.Sum256([]byte(r.Method + "::" + r.URL.Path))

		key := fmt.Sprintf("%x\n", hash)

		if entry, found := cache.Get(key); found && IsSamePayload(entry.RequestBody, body) {
			logger.Infof("cache middleware: serving cached payload for method: %s path: %s ttl: %s ", r.Method, r.URL.Path, time.Until(entry.Expiry).Round(time.Second).String())
			JSONCached(w, entry.StatusCode, json.RawMessage(entry.ResponseBody))
			return
		}

		recorder := &responseRecorder{ResponseWriter: w, Body: &bytes.Buffer{}}
		next(recorder, r)

		// Save only successful responses in the cache
		if recorder.StatusCode < 200 || recorder.StatusCode >= 300 {
			return
		}

		cache.Set(key, CacheEntry{
			StatusCode:   recorder.StatusCode,
			RequestBody:  body,
			ResponseBody: recorder.Body.Bytes(),
		}, duration)

	}
}

func IsSamePayload(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	if len(a) == 0 {
		a = []byte(`{}`)
	}

	if len(b) == 0 {
		b = []byte(`{}`)
	}

	var aPayload, bPayload map[string]interface{}
	if err := json.Unmarshal(a, &aPayload); err != nil {
		logger.Error(errors.Wrap(err, "failed to unmarshal payload A"))
		return false
	}
	if err := json.Unmarshal(b, &bPayload); err != nil {
		logger.Error(errors.Wrap(err, "failed to unmarshal payload B"))
		return false
	}
	return maps.Equal(aPayload, bPayload)
}
