package license

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

const (
	defaultReplicatedAppEndpoint = "https://replicated.app"
)

type LicenseData struct {
	LicenseBytes []byte
	License      *kotsv1beta1.License
}

func GetLicenseByID(licenseID string, endpoint string) (*kotsv1beta1.License, error) {
	if endpoint == "" {
		endpoint = defaultReplicatedAppEndpoint
	}
	url := fmt.Sprintf("%s/license", endpoint)

	licenseData, err := getLicenseFromAPI(url, licenseID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get license from api")
	}

	return licenseData.License, nil
}

func GetLatestLicense(license *kotsv1beta1.License, endpoint string) (*LicenseData, error) {
	if endpoint == "" {
		endpoint = license.Spec.Endpoint
	}
	url := fmt.Sprintf("%s/license/%s", endpoint, license.Spec.AppSlug)

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

	instanceData := report.GetInstanceData(store.GetStore())
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load response")
	}

	if resp.StatusCode >= 400 {
		return nil, errors.Errorf("unexpected result from get request: %d, data: %s", resp.StatusCode, body)
	}

	license, err := LoadLicenseFromBytes(body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load license from bytes")
	}

	data := &LicenseData{
		LicenseBytes: body,
		License:      license,
	}
	return data, nil
}

func LicenseIsExpired(license *kotsv1beta1.License) (bool, error) {
	val, found := license.Spec.Entitlements["expires_at"]
	if !found {
		return false, nil
	}
	if val.ValueType != "" && val.ValueType != "String" {
		return false, errors.Errorf("expires_at must be type String: %s", val.ValueType)
	}
	if val.Value.StrVal == "" {
		return false, nil
	}

	partsed, err := time.Parse(time.RFC3339, val.Value.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse expiration time")
	}
	return partsed.Before(time.Now()), nil
}

func GetLatestLicenseFields(license *kotsv1beta1.License, endpoint string) (types.LicenseFields, error) {
	if endpoint == "" {
		endpoint = license.Spec.Endpoint
	}
	url := fmt.Sprintf("%s/license/fields", endpoint)

	req, err := util.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.SetBasicAuth(license.Spec.LicenseID, license.Spec.LicenseID)

	instanceData := report.GetInstanceData(store.GetStore())
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load response")
	}

	if resp.StatusCode >= 400 {
		return nil, errors.Errorf("unexpected result from get request: %d, data: %s", resp.StatusCode, body)
	}

	var licenseFields types.LicenseFields
	if err := json.Unmarshal(body, &licenseFields); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal license fields")
	}

	return licenseFields, nil
}

func GetLatestLicenseField(license *kotsv1beta1.License, endpoint string, fieldName string) (*types.LicenseField, error) {
	if endpoint == "" {
		endpoint = license.Spec.Endpoint
	}
	url := fmt.Sprintf("%s/license/field/%s", endpoint, fieldName)

	req, err := util.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.SetBasicAuth(license.Spec.LicenseID, license.Spec.LicenseID)

	instanceData := report.GetInstanceData(store.GetStore())
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load response")
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("unexpected result from get request: %d, data: %s", resp.StatusCode, body)
	}

	var licenseField types.LicenseField
	if err := json.Unmarshal(body, &licenseField); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal license fields")
	}

	return &licenseField, nil
}
