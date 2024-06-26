package handlers

import (
	"bytes"
	"io"
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
	Expiry       time.Time
}

type cache struct {
	store map[string]CacheEntry
	mu    sync.RWMutex
}

func NewCache() *cache {
	return &cache{
		store: make(map[string]CacheEntry),
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

	entry.Expiry = time.Now().Add(duration)
	c.store[key] = entry
}

type responseRecorder struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}

func CacheMiddleware(next http.Handler, duration time.Duration, statusCode int) http.HandlerFunc {
	cache := NewCache()

	return func(w http.ResponseWriter, r *http.Request) {

		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error(errors.Wrap(err, "cache middleware failed ready request body"))
			http.Error(w, "cache middleware: unable to read request body", http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		cacheKey := r.Method + "::" + r.URL.Path + "::" + string(body)

		if entry, found := cache.Get(cacheKey); found {
			logger.Infof("cache middleware: serving from cache for method %s path %s", r.Method, r.URL.Path)
			JSON(w, statusCode, entry.ResponseBody)
			return
		}

		recorder := &responseRecorder{ResponseWriter: w, Body: &bytes.Buffer{}}
		next.ServeHTTP(recorder, r)

		cache.Set(cacheKey, CacheEntry{
			RequestBody:  body,
			ResponseBody: recorder.Body.Bytes(),
		}, duration)
	}
}
