package license

import (
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
)

// InstallationMetadata comes from the installation token in the signed KOTS
// license. Custom entitlements must not establish installation identity.
type InstallationMetadata struct {
	InstallationID       string
	CredentialGeneration int64
	Present              bool
}

func InstallationMetadataFromLicense(wrapper licensewrapper.LicenseWrapper) InstallationMetadata {
	if token, err := installationtoken.Parse(wrapper.GetLicenseID()); err == nil {
		return InstallationMetadata{InstallationID: token.InstallationID, CredentialGeneration: token.Generation, Present: true}
	}
	return InstallationMetadata{}
}
