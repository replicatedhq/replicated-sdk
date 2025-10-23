package integration

import (
	"context"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	integrationMockDataKey = "integration-mock-data"
	integrationEnabledKey  = "integration-enabled"
)

var replicatedSecretLock = sync.Mutex{}

func IsEnabled(ctx context.Context, clientset kubernetes.Interface, namespace string, wrapper licensetypes.LicenseWrapper) (bool, error) {
	if (wrapper.V1 == nil && wrapper.V2 == nil) || wrapper.GetLicenseType() != "dev" {
		return false, nil
	}

	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, util.GetReplicatedSecretName(), metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get replicated secret")
	}

	v, ok := secret.Data[integrationEnabledKey]
	if !ok || len(v) == 0 {
		return true, nil
	}

	enabled, _ := strconv.ParseBool(string(v))
	return enabled, nil
}
