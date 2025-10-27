package license

import (
	"github.com/pkg/errors"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
)


// VerifySignature verifies a license wrapper using the appropriate algorithm
func VerifySignature(wrapper licensetypes.LicenseWrapper) (licensetypes.LicenseWrapper, error) {
	if wrapper.V1 != nil {
		// Use kotskinds built-in validation for v1beta1 licenses
		_, err := wrapper.V1.ValidateLicense()
		if err != nil {
			return licensetypes.LicenseWrapper{}, err
		}
		// ValidateLicense() verifies all signatures and field integrity
		// Return the original wrapper since the license is already verified
		return wrapper, nil
	}

	if wrapper.V2 != nil {
		// Use kotskinds built-in validation for v1beta2 licenses
		_, err := wrapper.V2.ValidateLicense()
		if err != nil {
			return licensetypes.LicenseWrapper{}, err
		}
		// ValidateLicense() verifies all signatures and field integrity
		// Return the original wrapper since the license is already verified
		return wrapper, nil
	}

	return licensetypes.LicenseWrapper{}, errors.New("license wrapper is empty")
}







