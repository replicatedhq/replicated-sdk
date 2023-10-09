package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

func SendCustomAppMetricsData(sdkStore store.Store, license *kotsv1beta1.License, data map[string]interface{}) error {
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

	heartbeatInfo := heartbeat.GetHeartbeatInfo(sdkStore)
	heartbeat.InjectHeartbeatInfoHeaders(req, heartbeatInfo)

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
