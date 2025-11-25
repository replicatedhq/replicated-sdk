package license

import (
	"github.com/pkg/errors"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

// VerifySignature verifies a license wrapper using the appropriate algorithm
func VerifySignature(wrapper licensewrapper.LicenseWrapper) (licensewrapper.LicenseWrapper, error) {
  if wrapper.IsEmpty() {
    return licensewrapper.LicenseWrapper{}, errors.New("license wrapper is empty")
  }
  return wrapper, wrapper.VerifySignature()
}







