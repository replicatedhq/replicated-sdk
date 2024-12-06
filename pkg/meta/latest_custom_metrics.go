package meta

import (
	"context"
	"maps"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	customMetricsSecretKey replicatedMetadataSecretKey = "latest-custom-metrics"
)

func SyncCustomAppMetrics(ctx context.Context, clientset kubernetes.Interface, namespace string, inboundMetrics map[string]interface{}, overwrite bool) (map[string]interface{}, error) {
	existing := map[string]interface{}{}

	err := get(ctx, clientset, namespace, customMetricsSecretKey, &existing)
	if err != nil && errors.Cause(err) != ErrReplicatedMetadataNotFound {
		return nil, errors.Wrapf(err, "failed to get custom metrics data")
	}

	modified := mergeCustomAppMetrics(existing, inboundMetrics, overwrite)

	if err := save(ctx, clientset, namespace, customMetricsSecretKey, modified); err != nil {
		return nil, errors.Wrap(err, "failed to save custom metrics")
	}

	return modified, nil
}

func mergeCustomAppMetrics(existingMetrics map[string]interface{}, inboundMetrics map[string]interface{}, overwrite bool) map[string]interface{} {
	if existingMetrics == nil {
		existingMetrics = map[string]interface{}{}
	}

	if inboundMetrics == nil {
		inboundMetrics = map[string]interface{}{}
	}

	if overwrite {
		return inboundMetrics
	}

	if len(inboundMetrics) == 0 || maps.Equal(existingMetrics, inboundMetrics) {
		return existingMetrics
	}

	for k, v := range inboundMetrics {
		if v == nil {
			delete(existingMetrics, k)
			continue
		}

		existingMetrics[k] = v
	}

	return existingMetrics
}
