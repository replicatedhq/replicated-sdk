package report

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	tags "github.com/replicatedhq/replicated-sdk/pkg/tags"
	tagstypes "github.com/replicatedhq/replicated-sdk/pkg/tags/types"
	"k8s.io/client-go/kubernetes"
)

func SendAppInstanceTags(ctx context.Context, clientset kubernetes.Interface, sdkStore store.Store, tdata tagstypes.InstanceTagData) error {
	if err := tags.Save(ctx, clientset, sdkStore.GetNamespace(), tdata); err != nil {
		return errors.Wrap(err, "failed to save instance tags")
	}
	return SendInstanceData(clientset, sdkStore)
}
