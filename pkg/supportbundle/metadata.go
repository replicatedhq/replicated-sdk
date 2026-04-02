package supportbundle

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	SupportBundleMetadataSecretName = "replicated-support-metadata"
)

var metadataLock = sync.Mutex{}

// SaveMetadata saves support bundle metadata key:value pairs directly as top-level keys
// in the replicated-support-metadata secret's data field.
// If overwrite is true, the existing data is replaced entirely. If false, the provided keys are merged.
func SaveMetadata(ctx context.Context, clientset kubernetes.Interface, namespace string, data map[string]string, overwrite bool) error {
	if store.GetStore().GetReadOnlyMode() {
		return errors.New("support bundle metadata is unavailable in read-only mode")
	}

	metadataLock.Lock()
	defer metadataLock.Unlock()

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SupportBundleMetadataSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return errors.Errorf("secret %s not found in namespace %s", SupportBundleMetadataSecretName, namespace)
		}
		return errors.Wrap(err, "failed to get support bundle metadata secret")
	}

	if overwrite {
		existingSecret.Data = make(map[string][]byte, len(data))
		for k, v := range data {
			existingSecret.Data[k] = []byte(v)
		}
	} else {
		if existingSecret.Data == nil {
			existingSecret.Data = map[string][]byte{}
		}
		for k, v := range data {
			existingSecret.Data[k] = []byte(v)
		}
	}

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update support bundle metadata secret")
	}

	return nil
}
