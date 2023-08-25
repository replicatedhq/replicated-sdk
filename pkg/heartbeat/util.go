package heartbeat

import (
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
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

	if heartbeatInfo.K8sDistribution != "" {
		req.Header.Set("X-Replicated-K8sDistribution", heartbeatInfo.K8sDistribution)
	}
}

func canReport(license *kotsv1beta1.License) (bool, error) {
	if helm.IsHelmManaged() {
		replicatedHelmRevision := helm.GetReleaseRevision()

		helmRelease, err := helm.GetRelease(helm.GetReleaseName())
		if err != nil {
			return false, errors.Wrap(err, "failed to get release")
		}

		// don't report from replicated instances that are not associated with the current helm release revision.
		// this can happen during a helm upgrade/rollback when a rolling update of the replicated deployment is in progress.
		if replicatedHelmRevision != helmRelease.Version {
			logger.Debugf("not reporting from replicated instance with helm revision %d because current helm release revision is %d\n", replicatedHelmRevision, helmRelease.Version)
			return false, nil
		}
	}

	if util.IsAirgap() {
		return false, nil
	}

	if util.IsDevEnv() && !util.IsDevLicense(license) {
		// don't send reports from our dev env to our production services even if this is a production license
		return false, nil
	}

	return true, nil
}
