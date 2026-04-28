package apiserver

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// blockingTransport is a RoundTripper that fails any HTTP request and
// records that a dial was attempted. Tests use this in place of
// http.DefaultTransport to assert that a code path is fully local.
type blockingTransport struct {
	dials atomic.Int32
}

func (b *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b.dials.Add(1)
	return nil, &neverDialError{host: req.URL.Host}
}

type neverDialError struct{ host string }

func (e *neverDialError) Error() string {
	return "blockingTransport: outbound network access denied to " + e.host
}

// withBlockingTransport swaps http.DefaultTransport for the duration of the
// test and returns a counter the test can inspect.
func withBlockingTransport(t *testing.T) *blockingTransport {
	t.Helper()
	bt := &blockingTransport{}
	orig := http.DefaultTransport
	http.DefaultTransport = bt
	t.Cleanup(func() { http.DefaultTransport = orig })
	return bt
}

func TestLoadAndVerifyLicense_InvalidLicenseBytes_ReturnsPermanent(t *testing.T) {
	bt := withBlockingTransport(t)
	clientset := fake.NewSimpleClientset()

	_, err := loadAndVerifyLicense(APIServerParams{
		LicenseBytes: []byte("not a license"),
	}, clientset)
	require.Error(t, err, "expected an error when LicenseBytes is malformed")

	var perm *backoff.PermanentError
	require.ErrorAs(t, err, &perm, "expected backoff.Permanent")
	require.Zero(t, bt.dials.Load(), "production-mode license parse must not dial upstream")
}

func TestLoadAndVerifyLicense_ProductionMode_DoesNotDialUpstream(t *testing.T) {
	bt := withBlockingTransport(t)
	clientset := fake.NewSimpleClientset()

	// We don't care about the success/failure outcome here — only that
	// LicenseBytes-mode doesn't reach for the network. Even with bytes
	// that fail signature verification, no upstream call is permitted.
	_, _ = loadAndVerifyLicense(APIServerParams{
		LicenseBytes: []byte(validLicenseYAML),
	}, clientset)
	require.Zero(t, bt.dials.Load(), "production-mode license load+verify must not dial upstream")
}
