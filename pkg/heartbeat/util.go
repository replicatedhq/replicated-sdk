package heartbeat

import (
	"net/http"
	"strconv"

	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

func InjectHeartbeatInfoHeaders(req *http.Request, heartbeatInfo *types.HeartbeatInfo) {
	if heartbeatInfo == nil {
		return
	}

	req.Header.Set("X-Replicated-K8sVersion", heartbeatInfo.K8sVersion)
	req.Header.Set("X-Replicated-AppStatus", heartbeatInfo.AppStatus)
	req.Header.Set("X-Replicated-ClusterID", heartbeatInfo.ClusterID)
	req.Header.Set("X-Replicated-InstanceID", heartbeatInfo.InstanceID)

	if heartbeatInfo.ChannelID != "" {
		req.Header.Set("X-Replicated-DownstreamChannelID", heartbeatInfo.ChannelID)
	} else if heartbeatInfo.ChannelName != "" {
		req.Header.Set("X-Replicated-DownstreamChannelName", heartbeatInfo.ChannelName)
	}

	req.Header.Set("X-Replicated-DownstreamChannelSequence", strconv.FormatInt(heartbeatInfo.ChannelSequence, 10))
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
