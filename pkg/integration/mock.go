package integration

import (
	"context"
	_ "embed"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/integration/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"gopkg.in/yaml.v2"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	//go:embed data/default_mock_data.yaml
	defaultMockDataYAML []byte
)

func GetMockData(ctx context.Context, clientset kubernetes.Interface, namespace string) (*types.MockData, error) {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, util.GetReplicatedSecretName(), metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get replicated secret")
	}
	if err == nil {
		b := secret.Data[integrationMockDataKey]
		if len(b) != 0 {
			var mockData types.MockData
			if err := yaml.Unmarshal(b, &mockData); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal mock data")
			}
			return &mockData, nil
		}
	}

	return GetDefaultMockData(ctx)
}

func GetDefaultMockData(ctx context.Context) (*types.MockData, error) {
	var mockData types.MockData
	if err := yaml.Unmarshal(defaultMockDataYAML, &mockData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal default mock data")
	}
	return &mockData, nil
}

func SetMockData(ctx context.Context, clientset kubernetes.Interface, namespace string, mockData types.MockData) error {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	b, err := yaml.Marshal(mockData)
	if err != nil {
		return errors.Wrap(err, "failed to marshal mock data")
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, util.GetReplicatedSecretName(), metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get replicated secret")
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	secret.Data[integrationMockDataKey] = b
	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update replicated secret")
	}

	return nil
}
