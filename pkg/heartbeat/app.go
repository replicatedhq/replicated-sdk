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

	reqPayload, err := buildHeartbeatRequestPayload(heartbeatInfo)
	if err != nil {
		return errors.Wrap(err, "failed to build request payload")
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

func buildHeartbeatRequestPayload(heartbeatInfo *types.HeartbeatInfo) (*types.HeartbeatRequestPayload, error) {
	marshalledRS, err := json.Marshal(heartbeatInfo.ResourceStates)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal resource states")
	}

	marshalledKPD, err := json.Marshal(heartbeatInfo.K8sProviderData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal k8s provider data")
	}

	fmt.Println("sending heartbeat with provider data:", string(marshalledKPD))

	reqPayload := types.HeartbeatRequestPayload{
		ResourceStates:  string(marshalledRS),
		K8sProviderData: string(marshalledKPD),
	}

	return &reqPayload, nil
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

	// get kubernetes cluster version
	k8sVersion, err := k8sutil.GetK8sVersion()
	if err != nil {
		logger.Debugf("failed to get k8s version: %v", err.Error())
	} else {
		r.K8sVersion = k8sVersion
	}

	if distribution := GetDistribution(); distribution != types.UnknownDistribution {
		r.K8sDistribution = distribution.String()
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Debugf("failed to get k8s clientset: %v", err.Error())
	} else {
		r.K8sProviderData = GetK8sProviderData(clientset)
	}

	return &r
}
