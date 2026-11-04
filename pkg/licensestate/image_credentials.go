package licensestate

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
)

// licenseDockerConfig formats the installation token for image pulls.
//
// The registry hosts are configuration, not license data: the installer knows
// whether this installation pulls from the Replicated proxy, a customer's own
// registry, or an in-cluster airgap registry, and supplies them. Rotation
// changes the credential and nothing else, so the same hosts are rewritten with
// the new token. Deriving hosts from the license instead would only ever
// produce Replicated proxy domains, and could not express the other two cases.
func licenseDockerConfig(candidate *State, registryDomains []string) ([]byte, error) {
	if candidate == nil || len(candidate.CandidateLicense) == 0 {
		return nil, errors.New("candidate license is required for Docker config")
	}
	wrapper, err := sdklicense.LoadLicenseFromBytes(candidate.CandidateLicense)
	if err != nil {
		return nil, errors.Wrap(err, "parse candidate license for Docker config")
	}
	if wrapper.GetLicenseID() == "" {
		return nil, errors.New("candidate license has no registry credential")
	}
	domains, err := normalizeRegistryDomains(registryDomains)
	if err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return nil, errors.New("no registry domains are configured for image credentials")
	}
	type auth struct {
		Auth string `json:"auth"`
	}
	config := struct {
		Auths map[string]auth `json:"auths"`
	}{Auths: map[string]auth{}}
	encoded := base64.StdEncoding.EncodeToString([]byte("LICENSE_ID:" + wrapper.GetLicenseID()))
	for _, domain := range domains {
		config.Auths[domain] = auth{Auth: encoded}
	}
	credentials, err := json.Marshal(config)
	if err != nil {
		return nil, errors.Wrap(err, "marshal Docker config response")
	}
	return credentials, nil
}

// normalizeRegistryDomains rejects anything that is not a bare host. These
// strings are written into a credential file, so a path, query, or embedded
// userinfo is a configuration error rather than something to interpret.
func normalizeRegistryDomains(configured []string) ([]string, error) {
	seen := map[string]bool{}
	domains := []string{}
	for _, domain := range configured {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		u, err := url.Parse("https://" + domain)
		if err != nil || u.Host != domain || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.Errorf("configured registry domain is not a valid host")
		}
		if !seen[domain] {
			seen[domain] = true
			domains = append(domains, domain)
		}
	}
	return domains, nil
}
