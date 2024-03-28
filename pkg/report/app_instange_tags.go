package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

func SendAppInstanceTags(clientset kubernetes.Interface, sdkStore store.Store, data map[string]string) error {
	if util.IsAirgap() {
		return SendAirgapAppInstanceTags(clientset, sdkStore, data)
	}
	return SendOnlineAppInstanceTags(sdkStore, data)
}

func SendAirgapAppInstanceTags(clientset kubernetes.Interface, sdkStore store.Store, data map[string]string) error {
	report := &AppInstanceTagsReport{
		Events: []AppInstanceTagsReportEvent{
			{
				ReportedAt: time.Now().UTC().UnixMilli(),
				LicenseID:  sdkStore.GetLicense().Spec.LicenseID,
				InstanceID: sdkStore.GetAppID(),
				Data:       data,
			},
		},
	}

	if err := AppendReport(clientset, sdkStore.GetNamespace(), report); err != nil {
		return errors.Wrap(err, "failed to append custom app metrics report")
	}

	return nil
}

func SendOnlineAppInstanceTags(sdkStore store.Store, data map[string]string) error {
	license := sdkStore.GetLicense()

	endpoint := sdkStore.GetReplicatedAppEndpoint()
	if endpoint == "" {
		endpoint = license.Spec.Endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return errors.Wrap(err, "failed to parse endpoint")
	}

	hostname := u.Hostname()
	if u.Port() != "" {
		hostname = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
	}

	url := fmt.Sprintf("%s://%s/application/instance-tags", u.Scheme, hostname)

	payload := struct {
		Data map[string]string `json:"data"`
	}{
		Data: data,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "marshal data")
	}

	req, err := util.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return errors.Wrap(err, "call newrequest")
	}

	req.SetBasicAuth(license.Spec.LicenseID, license.Spec.LicenseID)
	req.Header.Set("Content-Type", "application/json")

	instanceData := GetInstanceData(sdkStore)
	InjectInstanceDataHeaders(req, instanceData)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		if len(body) > 0 {
			return util.ActionableError{Message: string(body)}
		}
		return errors.Errorf("unexpected result from get request: %d", resp.StatusCode)
	}

	return nil
}
