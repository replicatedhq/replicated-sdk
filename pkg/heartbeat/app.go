package heartbeat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

func SendAppHeartbeat(sdkStore store.Store) error {
	license := sdkStore.GetLicense()

	canReport, err := canReport(license)
	if err != nil {
		return errors.Wrap(err, "failed to check if can report")
	}
	if !canReport {
		return nil
	}

	heartbeatInfo := GetHeartbeatInfo(sdkStore)

	marshalledRS, err := json.Marshal(heartbeatInfo.ResourceStates)
	if err != nil {
		return errors.Wrap(err, "failed to marshal resource states")
	}
	reqPayload := map[string]interface{}{
		"resource_states": string(marshalledRS),
	}
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request payload")
	}

	postReq, err := util.NewRequest("POST", fmt.Sprintf("%s/kots_metrics/license_instance/info", license.Spec.Endpoint), bytes.NewBuffer(reqBody))
	if err != nil {
		return errors.Wrap(err, "failed to create http request")
	}
	postReq.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID)))))
	postReq.Header.Set("Content-Type", "application/json")

	InjectHeartbeatInfoHeaders(postReq, heartbeatInfo)

	resp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		return errors.Wrap(err, "failed to post request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("Unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func GetHeartbeatInfo(sdkStore store.Store) *types.HeartbeatInfo {
	r := types.HeartbeatInfo{
		ClusterID:       sdkStore.GetReplicatedID(),
		InstanceID:      sdkStore.GetAppID(),
		ChannelID:       sdkStore.GetChannelID(),
		ChannelName:     sdkStore.GetChannelName(),
		ChannelSequence: sdkStore.GetChannelSequence(),
		AppStatus:       string(sdkStore.GetAppStatus().State),
		ResourceStates:  sdkStore.GetAppStatus().ResourceStates,
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Debugf("failed to get clientset: %v", err.Error())
	} else {
		k8sVersion, err := k8sutil.GetK8sVersion(clientset)
		if err != nil {
			logger.Debugf("failed to get k8s version: %v", err.Error())
		} else {
			r.K8sVersion = k8sVersion
		}

		if distribution := GetDistribution(clientset); distribution != types.UnknownDistribution {
			r.K8sDistribution = distribution.String()
		}
	}

	return &r
}
