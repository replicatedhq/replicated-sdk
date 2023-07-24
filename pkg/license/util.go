package license

import (
	"os"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsscheme "github.com/replicatedhq/kotskinds/client/kotsclientset/scheme"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	kotsscheme.AddToScheme(scheme.Scheme)
}

func LoadLicenseFromPath(licenseFilePath string) (*kotsv1beta1.License, error) {
	licenseData, err := os.ReadFile(licenseFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read license file")
	}

	return LoadLicenseFromBytes(licenseData)
}

func LoadLicenseFromBytes(data []byte) (*kotsv1beta1.License, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, gvk, err := decode([]byte(data), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode license data")
	}

	if gvk.Group != "kots.io" || gvk.Version != "v1beta1" || gvk.Kind != "License" {
		return nil, errors.Errorf("unexpected GVK: %s", gvk.String())
	}

	return obj.(*kotsv1beta1.License), nil
}
