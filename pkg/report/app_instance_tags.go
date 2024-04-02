package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	tags "github.com/replicatedhq/replicated-sdk/pkg/tags"
	tagstypes "github.com/replicatedhq/replicated-sdk/pkg/tags/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

func SendAppInstanceTags(ctx context.Context, clientset kubernetes.Interface, sdkStore store.Store, tdata tagstypes.InstanceTagData) error {
	if err := tags.Save(ctx, clientset, sdkStore.GetNamespace(), tdata); err != nil {
		return errors.Wrap(err, "failed to save instance tags")
	}
	if util.IsAirgap() {
		return nil
	}
	return SendOnlineAppInstanceTags(sdkStore, tdata)
}

func SendOnlineAppInstanceTags(sdkStore store.Store, tdata tagstypes.InstanceTagData) error {
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
		Data tagstypes.InstanceTagData `json:"data"`
	}{
		Data: tdata,
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
		return errors.Wrap(err, "failed to execute post request")
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
		return errors.Errorf("unexpected result from post request: %d", resp.StatusCode)
	}

	return nil
}
