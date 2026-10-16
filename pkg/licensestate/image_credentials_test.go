package licensestate

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/stretchr/testify/require"
)

func dockerConfigAuths(t *testing.T, config []byte) map[string]string {
	t.Helper()
	var docker struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(config, &docker))
	auths := map[string]string{}
	for domain, auth := range docker.Auths {
		decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
		require.NoError(t, err)
		auths[domain] = string(decoded)
	}
	return auths
}

// Rotation changes the credential and nothing else. The configured hosts are
// the installer's, so a customer or airgap registry keeps working across
// rotations exactly as it did at install.
func TestImageCredentialsRewriteConfiguredHostsWithTheSuccessorToken(t *testing.T) {
	state, oldToken := portalTestState(t)
	configured := []string{"registry-v2.localhost:8000", "byo-registry.customer.example.com"}

	initial := &State{CandidateLicense: state.ActiveLicense}
	first, err := licenseDockerConfig(initial, configured)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"registry-v2.localhost:8000":        "LICENSE_ID:" + oldToken,
		"byo-registry.customer.example.com": "LICENSE_ID:" + oldToken,
	}, dockerConfigAuths(t, first))

	successor, err := installationtoken.New(state.InstallationID, 3)
	require.NoError(t, err)
	newToken, err := successor.Encode()
	require.NoError(t, err)
	state.ActiveImageCredentials = first
	state.CandidateLicense = []byte(strings.ReplaceAll(string(state.ActiveLicense), oldToken, newToken))

	rotated, err := licenseDockerConfig(state, configured)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"registry-v2.localhost:8000":        "LICENSE_ID:" + newToken,
		"byo-registry.customer.example.com": "LICENSE_ID:" + newToken,
	}, dockerConfigAuths(t, rotated))
}

func TestLicenseDockerConfigRequiresConfiguredDomains(t *testing.T) {
	state, _ := portalTestState(t)
	_, err := licenseDockerConfig(&State{CandidateLicense: state.ActiveLicense}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no registry domains are configured")
}

func TestNormalizeRegistryDomainsRejectsAnythingThatIsNotAHost(t *testing.T) {
	for _, tc := range []struct{ name, domain string }{
		{"path", "registry.example.com/repository"},
		{"userinfo", "user@registry.example.com"},
		{"embedded newline", "registry.\nexample.com"},
		{"query", "registry.example.com?a=b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeRegistryDomains([]string{tc.domain})
			require.Error(t, err)
		})
	}
}

func TestNormalizeRegistryDomainsDeduplicatesBareHosts(t *testing.T) {
	// Surrounding whitespace is a config artifact, not part of the host.
	domains, err := normalizeRegistryDomains([]string{
		"proxy.replicated.com", " proxy.replicated.com ", "registry.replicated.com\n", "  ",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"proxy.replicated.com", "registry.replicated.com"}, domains)
}

func TestNormalizeRegistryDomainsRejectsASchemeRatherThanStrippingIt(t *testing.T) {
	// The installer supplies bare hosts; a URL means the caller is confused.
	_, err := normalizeRegistryDomains([]string{"https://proxy.replicated.com"})
	require.Error(t, err)
}
