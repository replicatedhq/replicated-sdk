package license

import (
	"os"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	kotsscheme "github.com/replicatedhq/kotskinds/client/kotsclientset/scheme"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	kotsscheme.AddToScheme(scheme.Scheme)
}

func LoadLicenseFromPath(licenseFilePath string) (licensetypes.LicenseWrapper, error) {
	licenseData, err := os.ReadFile(licenseFilePath)
	if err != nil {
		return licensetypes.LicenseWrapper{}, errors.Wrap(err, "failed to read license file")
	}

	return LoadLicenseFromBytes(licenseData)
}

func LoadLicenseFromBytes(data []byte) (licensetypes.LicenseWrapper, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, gvk, err := decode([]byte(data), nil, nil)
	if err != nil {
		return licensetypes.LicenseWrapper{}, errors.Wrap(err, "failed to decode license data")
	}

	if gvk.Group != "kots.io" || (gvk.Version != "v1beta1" && gvk.Version != "v1beta2") || gvk.Kind != "License" {
		return licensetypes.LicenseWrapper{}, errors.Errorf("unexpected GVK: %s", gvk.String())
	}

	// Return wrapper with appropriate version populated
	switch gvk.Version {
	case "v1beta1":
		v1License, ok := obj.(*kotsv1beta1.License)
		if !ok {
			return licensetypes.LicenseWrapper{}, errors.New("failed to cast to v1beta1.License")
		}
		return licensetypes.LicenseWrapper{V1: v1License}, nil

	case "v1beta2":
		v2License, ok := obj.(*kotsv1beta2.License)
		if !ok {
			return licensetypes.LicenseWrapper{}, errors.New("failed to cast to v1beta2.License")
		}
		return licensetypes.LicenseWrapper{V2: v2License}, nil

	default:
		return licensetypes.LicenseWrapper{}, errors.Errorf("unsupported license version: %s", gvk.Version)
	}
}
