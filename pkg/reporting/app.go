package reporting

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kots-sdk/pkg/k8sutil"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/reporting/types"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/util"
)

func SendAppInfo() error {
	license := store.GetStore().GetLicense()
	if !canReport(license) {
		return nil
	}

	reportingInfo := GetReportingInfo()

	marshalledRS, err := json.Marshal(reportingInfo.ResourceStates)
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

	InjectReportingInfoHeaders(postReq, reportingInfo)

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

func GetReportingInfo() *types.ReportingInfo {
	r := types.ReportingInfo{
		ClusterID:       store.GetStore().GetKotsSDKID(),
		InstanceID:      store.GetStore().GetAppID(),
		ChannelID:       store.GetStore().GetChannelID(),
		ChannelName:     store.GetStore().GetChannelName(),
		ChannelSequence: store.GetStore().GetChannelSequence(),
		AppStatus:       string(store.GetStore().GetAppStatus().State),
		ResourceStates:  store.GetStore().GetAppStatus().ResourceStates,
	}

	// get kubernetes cluster version
	k8sVersion, err := k8sutil.GetK8sVersion()
	if err != nil {
		logger.Debugf("failed to get k8s version: %v", err.Error())
	} else {
		r.K8sVersion = k8sVersion
	}

	return &r
}
