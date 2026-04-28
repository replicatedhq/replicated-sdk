package license

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/license/cache"
	"github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

// LicenseSource identifies whether a Sync* result came from the live
// upstream or from the local cache. Handlers use this to decide whether
// to set the X-Replicated-License-Cache: stale response header.
type LicenseSource int

const (
	SourceUpstream LicenseSource = iota
	SourceCache
)

// staleWarnOnce ensures the cache-fallback warning log fires at most once
// per pod lifetime. Subsequent fallbacks are silent to avoid log spam
// during a sustained replicated.app outage.
var staleWarnOnce sync.Once

// warnStale logs the cache-fallback the first time it fires in a given
// pod. The original upstream error is included to give operators a clue
// about why the upstream call failed.
func warnStale(upstreamErr error) {
	staleWarnOnce.Do(func() {
		logger.Warnf("license_cache_fallback: serving cached license because upstream is unavailable: %v", upstreamErr)
	})
}

// SyncLicenseByID fetches a license by ID from the upstream Vendor Portal.
// On success, the license bytes are written through to the cache and
// SourceUpstream is returned.
//
// On upstream failure, the cache is consulted: if a previously-cached
// license is present and parses correctly, it is returned with
// SourceCache. If the cache is missing or unusable, the original upstream
// error is returned wrapped (so callers can inspect it).
//
// This wrapper is the entry point for the integration-mode boot path.
// Production-mode boots use the chart-embedded license bytes and never
// reach here.
func SyncLicenseByID(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	licenseID string,
	endpoint string,
) (*LicenseData, LicenseSource, error) {
	data, upstreamErr := GetLicenseByID(licenseID, endpoint)
	if upstreamErr == nil {
		if writeErr := cache.WriteLicense(ctx, clientset, namespace, data.LicenseBytes); writeErr != nil {
			logger.Infof("license cache: write-through after GetLicenseByID failed, continuing: %v", writeErr)
		}
		return data, SourceUpstream, nil
	}

	cached, cacheErr := cache.Read(ctx, clientset, namespace)
	if cacheErr != nil {
		return nil, SourceUpstream, errors.Wrap(upstreamErr, "upstream failed and license cache miss")
	}

	parsed, parseErr := LoadLicenseFromBytes(cached.LicenseBytes)
	if parseErr != nil {
		return nil, SourceUpstream, errors.Wrapf(upstreamErr, "upstream failed and cached license could not be parsed: %v", parseErr)
	}

	warnStale(upstreamErr)
	return &LicenseData{
		LicenseBytes: cached.LicenseBytes,
		License:      parsed,
	}, SourceCache, nil
}

// SyncLatestLicense refreshes the license against the upstream and writes
// through to the cache on success. On upstream failure, falls back to the
// cached license; on cache miss, returns the original upstream error.
//
// The returned LicenseData.LicenseBytes is what gets persisted on the
// success path. Cache hits return the bytes that were written by the most
// recent successful upstream call.
func SyncLatestLicense(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	wrapper licensewrapper.LicenseWrapper,
	endpoint string,
) (*LicenseData, LicenseSource, error) {
	data, upstreamErr := GetLatestLicense(wrapper, endpoint)
	if upstreamErr == nil {
		if writeErr := cache.WriteLicense(ctx, clientset, namespace, data.LicenseBytes); writeErr != nil {
			logger.Infof("license cache: write-through after GetLatestLicense failed, continuing: %v", writeErr)
		}
		return data, SourceUpstream, nil
	}

	cached, cacheErr := cache.Read(ctx, clientset, namespace)
	if cacheErr != nil {
		return nil, SourceUpstream, errors.Wrap(upstreamErr, "upstream failed and license cache miss")
	}

	parsed, parseErr := LoadLicenseFromBytes(cached.LicenseBytes)
	if parseErr != nil {
		return nil, SourceUpstream, errors.Wrapf(upstreamErr, "upstream failed and cached license could not be parsed: %v", parseErr)
	}

	warnStale(upstreamErr)
	return &LicenseData{
		LicenseBytes: cached.LicenseBytes,
		License:      parsed,
	}, SourceCache, nil
}

// SyncLatestLicenseFields refreshes the license-fields map against the
// upstream and writes through to the cache on success. On upstream
// failure, falls back to the cached fields; on cache miss, returns the
// original upstream error.
//
// Note: GetLatestLicenseField (singular) is intentionally NOT wrapped.
// Its handler already falls back to the in-memory store on upstream
// failure, and the in-memory store is repopulated from the cache during
// bootstrap. Wrapping it would duplicate that fallback path without
// adding value.
func SyncLatestLicenseFields(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	wrapper licensewrapper.LicenseWrapper,
	endpoint string,
) (types.LicenseFields, LicenseSource, error) {
	fields, upstreamErr := GetLatestLicenseFields(wrapper, endpoint)
	if upstreamErr == nil {
		if writeErr := cache.WriteLicenseFields(ctx, clientset, namespace, fields); writeErr != nil {
			logger.Infof("license cache: write-through after GetLatestLicenseFields failed, continuing: %v", writeErr)
		}
		return fields, SourceUpstream, nil
	}

	cached, cacheErr := cache.Read(ctx, clientset, namespace)
	if cacheErr != nil {
		return nil, SourceUpstream, errors.Wrap(upstreamErr, "upstream failed and license cache miss")
	}
	if cached.LicenseFields == nil {
		return nil, SourceUpstream, errors.Wrap(upstreamErr, "upstream failed and no cached license fields available")
	}

	warnStale(upstreamErr)
	return cached.LicenseFields, SourceCache, nil
}
