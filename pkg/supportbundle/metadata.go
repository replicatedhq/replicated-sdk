package supportbundle

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	SupportBundleMetadataSecretName = "replicated-support-metadata"
	supportBundleMetadataKey        = "support-bundle-metadata"
)

var metadataLock = sync.Mutex{}

// SaveMetadata saves support bundle metadata key:value pairs to the replicated-support-metadata secret.
// If overwrite is true, the existing metadata is replaced entirely. If false, the provided keys are merged.
func SaveMetadata(ctx context.Context, clientset kubernetes.Interface, namespace string, data map[string]string, overwrite bool) error {
	metadataLock.Lock()
	defer metadataLock.Unlock()

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SupportBundleMetadataSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return errors.Errorf("secret %s not found in namespace %s", SupportBundleMetadataSecretName, namespace)
		}
		return errors.Wrap(err, "failed to get support bundle metadata secret")
	}

	finalData := data
	if !overwrite {
		// Merge with existing data
		existing := map[string]string{}
		if existingSecret.Data != nil {
			if existingBytes, ok := existingSecret.Data[supportBundleMetadataKey]; ok && len(existingBytes) > 0 {
				if err := json.Unmarshal(existingBytes, &existing); err != nil {
					return errors.Wrap(err, "failed to unmarshal existing support bundle metadata")
				}
			}
		}
		for k, v := range data {
			existing[k] = v
		}
		finalData = existing
	}

	encodedData, err := json.Marshal(finalData)
	if err != nil {
		return errors.Wrap(err, "failed to marshal support bundle metadata")
	}

	if existingSecret.Data == nil {
		existingSecret.Data = map[string][]byte{}
	}
	existingSecret.Data[supportBundleMetadataKey] = encodedData

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update support bundle metadata secret")
	}

	return nil
}
