package license

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
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
	License      licensewrapper.LicenseWrapper
}

func GetLicenseByID(licenseID string, endpoint string) (licensewrapper.LicenseWrapper, error) {
	if endpoint == "" {
		endpoint = defaultReplicatedAppEndpoint
	}
	url := fmt.Sprintf("%s/license", endpoint)

	licenseData, err := getLicenseFromAPI(url, licenseID)
	if err != nil {
		return licensewrapper.LicenseWrapper{}, errors.Wrap(err, "failed to get license from api")
	}

	return licenseData.License, nil
}

func GetLatestLicense(wrapper licensewrapper.LicenseWrapper, endpoint string) (*LicenseData, error) {
	if endpoint == "" {
		endpoint = wrapper.GetEndpoint()
	}
	url := fmt.Sprintf("%s/license/%s", endpoint, wrapper.GetAppSlug())

	licenseData, err := getLicenseFromAPI(url, wrapper.GetLicenseID())
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

	resp, err := util.HttpClient().Do(req)
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

func LicenseIsExpired(wrapper licensewrapper.LicenseWrapper) (bool, error) {
	entitlements := wrapper.GetEntitlements()
	if entitlements == nil {
		return false, nil
	}

	ent, found := entitlements["expires_at"]
	if !found {
		return false, nil
	}

	valueType := ent.GetValueType()
	if valueType != "" && valueType != "String" {
		return false, errors.Errorf("expires_at must be type String: %s", valueType)
	}

	expiresAtValue := ent.GetValue().StrVal
	if expiresAtValue == "" {
		return false, nil
	}

	parsed, err := time.Parse(time.RFC3339, expiresAtValue)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse expiration time")
	}
	return parsed.Before(time.Now()), nil
}

func GetLatestLicenseFields(wrapper licensewrapper.LicenseWrapper, endpoint string) (types.LicenseFields, error) {
	if endpoint == "" {
		endpoint = wrapper.GetEndpoint()
	}
	url := fmt.Sprintf("%s/license/fields", endpoint)

	req, err := util.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.SetBasicAuth(wrapper.GetLicenseID(), wrapper.GetLicenseID())

	instanceData := report.GetInstanceData(store.GetStore())
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := util.HttpClient().Do(req)
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

func GetLatestLicenseField(wrapper licensewrapper.LicenseWrapper, endpoint string, fieldName string) (*types.LicenseField, error) {
	if endpoint == "" {
		endpoint = wrapper.GetEndpoint()
	}
	url := fmt.Sprintf("%s/license/field/%s", endpoint, fieldName)

	req, err := util.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.SetBasicAuth(wrapper.GetLicenseID(), wrapper.GetLicenseID())

	instanceData := report.GetInstanceData(store.GetStore())
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := util.HttpClient().Do(req)
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
