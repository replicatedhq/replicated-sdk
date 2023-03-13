package replicatedapp

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kots-sdk/pkg/kotsutil"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

type LicenseData struct {
	LicenseBytes []byte
	License      *kotsv1beta1.License
}

func GetLatestLicense(license *kotsv1beta1.License) (*LicenseData, error) {
	url := fmt.Sprintf("%s/license/%s", license.Spec.Endpoint, license.Spec.AppSlug)

	licenseData, err := getLicenseFromAPI(url, license.Spec.LicenseID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get license from api")
	}

	return licenseData, nil
}

func getLicenseFromAPI(url string, licenseID string) (*LicenseData, error) {
	req, err := util.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.SetBasicAuth(licenseID, licenseID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load response")
	}

	if resp.StatusCode >= 400 {
		return nil, errors.Errorf("unexpected result from get request: %d, data: %s", resp.StatusCode, body)
	}

	license, err := kotsutil.LoadLicenseFromBytes(body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load license from bytes")
	}

	data := &LicenseData{
		LicenseBytes: body,
		License:      license,
	}
	return data, nil
}
