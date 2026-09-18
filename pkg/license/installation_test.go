package license

import (
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/stretchr/testify/require"
)

func TestInstallationIdentityComesOnlyFromInstallationToken(t *testing.T) {
	token, err := installationtoken.New("portal-installation", 7)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	for _, test := range []struct {
		name, licenseID string
		present         bool
	}{
		{"Installation", encoded, true},
		{"Legacy", "existing-license-id", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Vendor-defined fields are not a second source of installation ID.
			wrapper, err := LoadLicenseFromBytes([]byte("apiVersion: kots.io/v1beta1\nkind: License\nmetadata:\n  name: test\nspec:\n  licenseID: " + test.licenseID + "\n  entitlements:\n    replicated_sdk_installation_id:\n      value: another-installation\n      valueType: String\n    replicated_sdk_credential_generation:\n      value: 99\n      valueType: Integer\n"))
			require.NoError(t, err)
			metadata := InstallationMetadataFromLicense(wrapper)
			require.Equal(t, test.present, metadata.Present)
			if test.present {
				require.Equal(t, "portal-installation", metadata.InstallationID)
				require.Equal(t, int64(7), metadata.CredentialGeneration)
			} else {
				require.Empty(t, metadata.InstallationID)
				require.Zero(t, metadata.CredentialGeneration)
			}
		})
	}
}
