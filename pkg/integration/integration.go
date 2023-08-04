package integration

import (
	"context"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	replicatedSDKSecretName = "replicated-sdk"
	integrationMockDataKey  = "integration-mock-data"
	integrationEnabledKey   = "integration-enabled"
)

var replicatedSDKSecretLock = sync.Mutex{}

func IsEnabled(ctx context.Context, clientset kubernetes.Interface, namespace string, license *kotsv1beta1.License) (bool, error) {
	if license == nil || license.Spec.LicenseType != "dev" {
		return false, nil
	}

	replicatedSDKSecretLock.Lock()
	defer replicatedSDKSecretLock.Unlock()

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, replicatedSDKSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get replicated-sdk secret")
	}

	v, ok := secret.Data[integrationEnabledKey]
	if !ok || len(v) == 0 {
		return true, nil
	}

	enabled, _ := strconv.ParseBool(string(v))
	return enabled, nil
}
