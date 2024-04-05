package report

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/tags"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

var instanceDataMtx sync.Mutex

func SendInstanceData(clientset kubernetes.Interface, sdkStore store.Store) error {
	license := sdkStore.GetLicense()

	canReport, err := canReport(clientset, sdkStore.GetNamespace(), license)
	if err != nil {
		return errors.Wrap(err, "failed to check if can report")
	}
	if !canReport {
		return nil
	}

	// make sure events are reported in order
	instanceDataMtx.Lock()
	defer func() {
		time.Sleep(1 * time.Second)
		instanceDataMtx.Unlock()
	}()

	instanceData := GetInstanceData(sdkStore)

	if util.IsAirgap() {
		return SendAirgapInstanceData(clientset, sdkStore.GetNamespace(), license.Spec.LicenseID, instanceData)
	}

	return SendOnlineInstanceData(license, instanceData)
}

func SendAirgapInstanceData(clientset kubernetes.Interface, namespace string, licenseID string, instanceData *types.InstanceData) error {
	event := InstanceReportEvent{
		ReportedAt:                time.Now().UTC().UnixMilli(),
		LicenseID:                 licenseID,
		InstanceID:                instanceData.InstanceID,
		ClusterID:                 instanceData.ClusterID,
		UserAgent:                 buildversion.GetUserAgent(),
		AppStatus:                 instanceData.AppStatus,
		K8sVersion:                instanceData.K8sVersion,
		K8sDistribution:           instanceData.K8sDistribution,
		DownstreamChannelID:       instanceData.ChannelID,
		DownstreamChannelName:     instanceData.ChannelName,
		DownstreamChannelSequence: instanceData.ChannelSequence,
	}

	if instanceData.ResourceStates != nil {
		marshalledRS, err := json.Marshal(instanceData.ResourceStates)
		if err != nil {
			return errors.Wrap(err, "failed to marshal resource states")
		}
		event.ResourceStates = string(marshalledRS)
	}

	marshalledTags, err := json.Marshal(instanceData.Tags)
	if err != nil {
		return errors.Wrap(err, "failed to marshal tags")
	}
	event.Tags = string(marshalledTags)

	report := &InstanceReport{
		Events: []InstanceReportEvent{event},
	}

	if err := AppendReport(clientset, namespace, report); err != nil {
		return errors.Wrap(err, "failed to append instance report")
	}

	return nil
}

func SendOnlineInstanceData(license *v1beta1.License, instanceData *types.InstanceData) error {
	// build the request body
	reqPayload := map[string]interface{}{}
	if err := InjectInstanceDataPayload(reqPayload, instanceData); err != nil {
		return errors.Wrap(err, "failed to inject instance data payload")
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

	InjectInstanceDataHeaders(postReq, instanceData)

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

func GetInstanceData(sdkStore store.Store) *types.InstanceData {
	r := types.InstanceData{
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

		if tdata, err := tags.Get(context.TODO(), clientset, sdkStore.GetNamespace()); err != nil {
			logger.Debugf("failed to get instance tag data: %v", err.Error())
		} else {
			r.Tags = *tdata
		}
	}

	return &r
}
