package license

import (
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

// LoadLicenseFromPath loads a license from a file path.
// This function now wraps the licensewrapper package implementation.
func LoadLicenseFromPath(licenseFilePath string) (licensewrapper.LicenseWrapper, error) {
	return licensewrapper.LoadLicenseFromPath(licenseFilePath)
}

// LoadLicenseFromBytes deserializes license YAML/JSON bytes into a LicenseWrapper.
// This function now wraps the licensewrapper package implementation.
func LoadLicenseFromBytes(data []byte) (licensewrapper.LicenseWrapper, error) {
	return licensewrapper.LoadLicenseFromBytes(data)
}
