package integration

import (
	"context"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	replicatedSecretName             = "replicated"
	replicatedIntegrationMockDataKey = "REPLICATED_INTEGRATION_MOCK_DATA"
	replicatedIntegrationEnabledKey  = "REPLICATED_INTEGRATION_ENABLED"
)

var replicatedSecretLock = sync.Mutex{}

func IsEnabled(ctx context.Context, clientset kubernetes.Interface, namespace string, license *kotsv1beta1.License) (bool, error) {
	if license == nil || license.Spec.LicenseType != "dev" {
		return false, nil
	}

	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get replicated secret")
	}

	v, ok := secret.Data[replicatedIntegrationEnabledKey]
	if !ok {
		return true, nil
	}

	enabled, _ := strconv.ParseBool(string(v))
	return enabled, nil
}

func createReplicatedSecret(ctx context.Context, clientset kubernetes.Interface, namespace string, data map[string][]byte) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      replicatedSecretName,
			Namespace: namespace,
		},
		Data: data,
	}

	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret replicated")
	}

	return nil
}
