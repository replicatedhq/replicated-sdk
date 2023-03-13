package reporting

import (
	"encoding/base64"
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

	postReq, err := util.NewRequest("POST", fmt.Sprintf("%s/kots_metrics/license_instance/info", license.Spec.Endpoint), nil)
	if err != nil {
		return errors.Wrap(err, "failed to create http request")
	}
	postReq.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID)))))
	postReq.Header.Set("Content-Type", "application/json")

	reportingInfo := GetReportingInfo()
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
		ClusterID:  store.GetStore().GetKotsSDKID(),
		InstanceID: store.GetStore().GetAppID(),
	}

	di, err := getDownstreamInfo()
	if err != nil {
		logger.Debugf("failed to get downstream info: %v", err.Error())
	}
	if di != nil {
		r.Downstream = *di
	}

	// get kubernetes cluster version
	k8sVersion, err := k8sutil.GetK8sVersion()
	if err != nil {
		logger.Debugf("failed to get k8s version: %v", err.Error())
	} else {
		r.K8sVersion = k8sVersion
	}

	// get app status
	r.AppStatus = string(store.GetStore().GetAppStatus().State)

	return &r
}

func getDownstreamInfo() (*types.DownstreamInfo, error) {
	di := types.DownstreamInfo{}

	di.ChannelID = store.GetStore().GetChannelID()
	di.ChannelName = store.GetStore().GetChannelName()
	di.ChannelSequence = store.GetStore().GetChannelSequence()
	di.Status = string(store.GetStore().GetAppStatus().State)

	return &di, nil
}
