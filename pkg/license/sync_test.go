package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/license/cache"
	"github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const syncTestNamespace = "test-ns"

// testFixture bundles the moving parts every Sync* test needs: a fake
// kubernetes clientset (with the SDK Deployment so cache writes can resolve
// owner references), an initialized in-memory store, and a reset hook for
// the package-level once-per-pod warning.
type testFixture struct {
	clientset kubernetes.Interface
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()

	store.InitInMemory(store.InitInMemoryStoreOptions{
		Namespace: syncTestNamespace,
	})
	t.Cleanup(func() { store.SetStore(nil) })

	resetStaleWarnOnce(t)

	clientset := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "replicated",
			Namespace: syncTestNamespace,
			UID:       apimachinerytypes.UID("deadbeef-0000-0000-0000-000000000000"),
		},
	})

	return &testFixture{clientset: clientset}
}

// resetStaleWarnOnce makes the once-per-pod warning fire fresh in each
// test. The package-level sync.Once otherwise leaks state across tests.
func resetStaleWarnOnce(t *testing.T) {
	t.Helper()
	staleWarnOnce = sync.Once{}
}

// licenseServer returns an httptest server that serves validLicenseYAML
// from any path (so the same server backs GetLicenseByID and
// GetLatestLicense regardless of URL shape).
func licenseServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(validLicenseYAML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// brokenServer returns an httptest server that 503s every request,
// simulating an unreachable replicated.app.
func brokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream broken", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fieldsServer returns an httptest server that serves a fixed
// LicenseFields payload from any path.
func fieldsServer(t *testing.T, fields types.LicenseFields) *httptest.Server {
	t.Helper()
	body, err := json.Marshal(fields)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---- SyncLicenseByID ----

func TestSyncLicenseByID_UpstreamSuccess_WritesThroughToCache(t *testing.T) {
	f := newTestFixture(t)
	srv := licenseServer(t)

	data, source, err := SyncLicenseByID(context.Background(), f.clientset, syncTestNamespace, "license-id", srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceUpstream, source)
	require.NotEmpty(t, data.LicenseBytes)

	cached, err := cache.Read(context.Background(), f.clientset, syncTestNamespace)
	require.NoError(t, err, "successful upstream call must write through to cache")
	require.Equal(t, data.LicenseBytes, cached.LicenseBytes)
}

func TestSyncLicenseByID_UpstreamFailure_CacheHit_ReturnsCached(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	require.NoError(t, cache.WriteLicense(context.Background(), f.clientset, syncTestNamespace, []byte(validLicenseYAML)))

	data, source, err := SyncLicenseByID(context.Background(), f.clientset, syncTestNamespace, "license-id", srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceCache, source)
	require.Equal(t, []byte(validLicenseYAML), data.LicenseBytes)
}

func TestSyncLicenseByID_UpstreamFailure_CacheMiss_ReturnsWrappedError(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	_, source, err := SyncLicenseByID(context.Background(), f.clientset, syncTestNamespace, "license-id", srv.URL)
	require.Error(t, err)
	require.Equal(t, SourceUpstream, source, "no fallback available, source must reflect the failed attempt")
	require.Contains(t, err.Error(), "license cache miss")
}

// ---- SyncLatestLicense ----

func TestSyncLatestLicense_UpstreamSuccess_WritesThroughToCache(t *testing.T) {
	f := newTestFixture(t)
	srv := licenseServer(t)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	data, source, err := SyncLatestLicense(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceUpstream, source)

	cached, err := cache.Read(context.Background(), f.clientset, syncTestNamespace)
	require.NoError(t, err)
	require.Equal(t, data.LicenseBytes, cached.LicenseBytes)
}

func TestSyncLatestLicense_UpstreamFailure_CacheHit_ReturnsCached(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	require.NoError(t, cache.WriteLicense(context.Background(), f.clientset, syncTestNamespace, []byte(validLicenseYAML)))

	data, source, err := SyncLatestLicense(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceCache, source)
	require.Equal(t, []byte(validLicenseYAML), data.LicenseBytes)
}

func TestSyncLatestLicense_UpstreamFailure_CacheMiss_ReturnsWrappedError(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	_, source, err := SyncLatestLicense(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.Error(t, err)
	require.Equal(t, SourceUpstream, source)
	require.Contains(t, err.Error(), "license cache miss")
}

// ---- SyncLatestLicenseFields ----

func TestSyncLatestLicenseFields_UpstreamSuccess_WritesThroughToCache(t *testing.T) {
	f := newTestFixture(t)
	expected := types.LicenseFields{
		"my_field": types.LicenseField{Title: "My Field", Value: "v1"},
	}
	srv := fieldsServer(t, expected)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	// In production a license is always cached before fields are
	// (bootstrapCritical → bootstrapBackground ordering), so seed that
	// state before exercising the fields write-through.
	require.NoError(t, cache.WriteLicense(context.Background(), f.clientset, syncTestNamespace, []byte(validLicenseYAML)))

	got, source, err := SyncLatestLicenseFields(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceUpstream, source)
	require.Equal(t, "v1", got["my_field"].Value)

	cached, err := cache.Read(context.Background(), f.clientset, syncTestNamespace)
	require.NoError(t, err)
	require.Equal(t, "v1", cached.LicenseFields["my_field"].Value)
}

func TestSyncLatestLicenseFields_UpstreamFailure_CacheHit_ReturnsCached(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	cached := types.LicenseFields{
		"my_field": types.LicenseField{Title: "Cached", Value: "stale"},
	}
	// Seed the cache with both a license (so Read returns success) and fields.
	require.NoError(t, cache.WriteLicense(context.Background(), f.clientset, syncTestNamespace, []byte(validLicenseYAML)))
	require.NoError(t, cache.WriteLicenseFields(context.Background(), f.clientset, syncTestNamespace, cached))

	got, source, err := SyncLatestLicenseFields(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.NoError(t, err)
	require.Equal(t, SourceCache, source)
	require.Equal(t, "stale", got["my_field"].Value)
}

func TestSyncLatestLicenseFields_UpstreamFailure_NoCachedFields_ReturnsWrappedError(t *testing.T) {
	f := newTestFixture(t)
	srv := brokenServer(t)

	wrapper, err := LoadLicenseFromBytes([]byte(validLicenseYAML))
	require.NoError(t, err)

	// License cached but fields never written — fields fallback must miss.
	require.NoError(t, cache.WriteLicense(context.Background(), f.clientset, syncTestNamespace, []byte(validLicenseYAML)))

	_, source, err := SyncLatestLicenseFields(context.Background(), f.clientset, syncTestNamespace, wrapper, srv.URL)
	require.Error(t, err)
	require.Equal(t, SourceUpstream, source)
	require.Contains(t, err.Error(), "no cached license fields available")
}
