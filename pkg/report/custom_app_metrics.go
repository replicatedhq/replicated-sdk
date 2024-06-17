package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

func SendCustomAppMetrics(clientset kubernetes.Interface, sdkStore store.Store, data map[string]interface{}) error {
	// Logic here

	// Get current tags data from secret
	currentTags := map[string]interface{}{}

	data = SyncCustomAppMetrics(currentTags, data, false)

	// Save data

	if util.IsAirgap() {
		return SendAirgapCustomAppMetrics(clientset, sdkStore, data)
	}
	return SendOnlineCustomAppMetrics(sdkStore, data)
}

func SyncCustomAppMetrics(existingMetrics map[string]interface{}, inboundMetrics map[string]interface{}, overwrite bool) map[string]interface{} {
	if overwrite {
		return inboundMetrics
	}

	if len(inboundMetrics) == 0 || maps.Equal(existingMetrics, inboundMetrics) {
		return existingMetrics
	}

	for k, v := range inboundMetrics {
		if v == nil {
			delete(existingMetrics, k)
			continue
		}

		existingMetrics[k] = v
	}

	return existingMetrics
}

func SendAirgapCustomAppMetrics(clientset kubernetes.Interface, sdkStore store.Store, data map[string]interface{}) error {
	report := &CustomAppMetricsReport{
		Events: []CustomAppMetricsReportEvent{
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

func SendOnlineCustomAppMetrics(sdkStore store.Store, data map[string]interface{}) error {
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

	url := fmt.Sprintf("%s://%s/application/custom-metrics", u.Scheme, hostname)

	payload := struct {
		Data map[string]interface{} `json:"data"`
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
