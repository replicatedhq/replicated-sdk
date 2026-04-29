package apiserver

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cenkalti/backoff/v4"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/stretchr/testify/require"
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

	_, err := loadAndVerifyLicense(APIServerParams{
		LicenseBytes: []byte("not a license"),
	})
	require.Error(t, err, "expected an error when LicenseBytes is malformed")

	var perm *backoff.PermanentError
	require.ErrorAs(t, err, &perm, "expected backoff.Permanent")
	require.Zero(t, bt.dials.Load(), "production-mode license parse must not dial upstream")
}

func TestLoadAndVerifyLicense_ProductionMode_DoesNotDialUpstream(t *testing.T) {
	bt := withBlockingTransport(t)

	// We don't care about the success/failure outcome here — only that
	// LicenseBytes-mode doesn't reach for the network. Even with bytes
	// that fail signature verification, no upstream call is permitted.
	_, _ = loadAndVerifyLicense(APIServerParams{
		LicenseBytes: []byte(validLicenseYAML),
	})
	require.Zero(t, bt.dials.Load(), "production-mode license load+verify must not dial upstream")
}

// resetAirgapOverride restores the package-level airgap override after a
// test that flipped it. Without this, the override leaks into subsequent
// tests in the package and silently disables their upstream gates.
func resetAirgapOverride(t *testing.T) {
	t.Helper()
	t.Cleanup(util.ResetAirgapOverride)
}

func devLicenseWrapper() licensewrapper.LicenseWrapper {
	return licensewrapper.LicenseWrapper{
		V1: &kotsv1beta1.License{
			Spec: kotsv1beta1.LicenseSpec{LicenseType: "dev"},
		},
	}
}

func prodLicenseWrapper() licensewrapper.LicenseWrapper {
	return licensewrapper.LicenseWrapper{
		V1: &kotsv1beta1.License{
			Spec: kotsv1beta1.LicenseSpec{LicenseType: "prod"},
		},
	}
}

func TestApplyDevOfflineGuard_OffByDefault_NoOp(t *testing.T) {
	resetAirgapOverride(t)
	require.NoError(t, applyDevOfflineGuard(prodLicenseWrapper(), false))
	require.False(t, util.IsAirgap(), "devOffline=false must not flip airgap override")
}

func TestApplyDevOfflineGuard_ProdLicense_Rejected(t *testing.T) {
	resetAirgapOverride(t)
	err := applyDevOfflineGuard(prodLicenseWrapper(), true)
	require.Error(t, err)

	var perm *backoff.PermanentError
	require.ErrorAs(t, err, &perm, "non-dev license + devOffline must be a permanent failure")
	require.False(t, util.IsAirgap(), "rejected guard must not have flipped the airgap override")
}

func TestApplyDevOfflineGuard_DevLicense_FlipsAirgap(t *testing.T) {
	resetAirgapOverride(t)
	require.False(t, util.IsAirgap(), "precondition: airgap should be off before guard runs")

	require.NoError(t, applyDevOfflineGuard(devLicenseWrapper(), true))
	require.True(t, util.IsAirgap(), "devOffline=true + dev license must flip the airgap override")
}

// TestApplyDevOfflineGuard_NoUpstreamDial defends the contract that the
// guard itself never reaches for the network. The guard only inspects
// already-parsed wrapper fields and writes a process-local atomic; if a
// future refactor accidentally introduced an upstream call here it would
// undermine the entire purpose of the offline opt-in.
func TestApplyDevOfflineGuard_NoUpstreamDial(t *testing.T) {
	resetAirgapOverride(t)
	bt := withBlockingTransport(t)

	require.NoError(t, applyDevOfflineGuard(devLicenseWrapper(), true))
	require.Zero(t, bt.dials.Load(), "devOffline guard must not dial upstream")
}
