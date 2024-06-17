package meta

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/meta/types"
	"k8s.io/client-go/kubernetes"
)

const (
	instanceTagSecretKey replicatedMetadataSecretKey = "instance-tag-data"
)

func SaveInstanceTag(ctx context.Context, clientset kubernetes.Interface, namespace string, tdata types.InstanceTagData) error {
	return save(ctx, clientset, namespace, instanceTagSecretKey, tdata)
}

func GetInstanceTag(ctx context.Context, clientset kubernetes.Interface, namespace string) (*types.InstanceTagData, error) {
	t := types.InstanceTagData{}

	if err := get(ctx, clientset, namespace, instanceTagSecretKey, &t); err != nil {
		return nil, errors.Wrapf(err, "failed to get instance tag data")
	}

	return &t, nil

}
