package helm

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"helm.sh/helm/v3/pkg/action"
)

var cfg *action.Configuration

func init() {
	if !IsHelmManaged() {
		return // not running in a helm environment
	}

	cfg = new(action.Configuration)
	if err := cfg.Init(k8sutil.KubernetesConfigFlags, GetReleaseNamespace(), GetHelmDriver(), logger.Debugf); err != nil {
		panic(errors.Wrap(err, "failed to init helm action config"))
	}
}
