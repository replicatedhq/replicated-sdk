package meta

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	customMetricsSecretKey replicatedMetadataSecretKey = "latest-custom-metrics"
)

func SaveLatestCustomMetrics(ctx context.Context, clientset kubernetes.Interface, namespace string, customMetrics map[string]interface{}) error {
	return save(ctx, clientset, namespace, customMetricsSecretKey, customMetrics)
}

func GetLatestCustomMetrics(ctx context.Context, clientset kubernetes.Interface, namespace string) (map[string]interface{}, error) {
	cm := map[string]interface{}{}

	err := get(ctx, clientset, namespace, customMetricsSecretKey, &cm)
	if err != nil && errors.Cause(err) != ErrReplicatedMetadataSecretNotFound {
		return nil, errors.Wrapf(err, "failed to get custom metrics data")
	}

	if errors.Cause(err) == ErrReplicatedMetadataSecretNotFound {
		if err := SaveLatestCustomMetrics(ctx, clientset, namespace, cm); err != nil {
			return nil, errors.Wrap(err, "failed to create custom metrics secret data")
		}
		return cm, nil
	}

	return cm, nil
}
