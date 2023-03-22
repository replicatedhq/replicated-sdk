package reporting

import (
	"net/http"
	"strconv"

	"github.com/replicatedhq/kots-sdk/pkg/reporting/types"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

func InjectReportingInfoHeaders(req *http.Request, reportingInfo *types.ReportingInfo) {
	if reportingInfo == nil {
		return
	}

	req.Header.Set("X-Replicated-K8sVersion", reportingInfo.K8sVersion)
	req.Header.Set("X-Replicated-AppStatus", reportingInfo.AppStatus)
	req.Header.Set("X-Replicated-ClusterID", reportingInfo.ClusterID)
	req.Header.Set("X-Replicated-InstanceID", reportingInfo.InstanceID)

	if reportingInfo.ChannelID != "" {
		req.Header.Set("X-Replicated-DownstreamChannelID", reportingInfo.ChannelID)
	} else if reportingInfo.ChannelName != "" {
		req.Header.Set("X-Replicated-DownstreamChannelName", reportingInfo.ChannelName)
	}

	req.Header.Set("X-Replicated-DownstreamChannelSequence", strconv.FormatInt(reportingInfo.ChannelSequence, 10))
}

func canReport(license *kotsv1beta1.License) bool {
	if util.IsAirgap() {
		return false
	}
	if util.IsDevEnv() && !util.IsDevLicense(license) {
		// don't send reports from our dev env to our production services even if this is a production license
		return false
	}
	return true
}
