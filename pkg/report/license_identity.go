package report

import "github.com/replicatedhq/replicated-sdk/pkg/installationtoken"

// Offline reports can leave the cluster in a support bundle. Record the
// stable installation ID, never the credential needed to authenticate it.
func reportLicenseIdentity(credential string) (licenseID, installationID string) {
	if token, err := installationtoken.Parse(credential); err == nil {
		return "", token.InstallationID
	}
	return credential, ""
}
