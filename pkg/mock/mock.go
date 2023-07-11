package mock

import (
	"context"
	_ "embed"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	replicatedSecretName     = "replicated"
	replicatedMockDataKey    = "REPLICATED_MOCK_DATA"
	replicatedMockEnabledKey = "REPLICATED_MOCK_ENABLED"

	CurrentReleaseMockKey    = "currentRelease"
	DeployedReleasesMockKey  = "deployedReleases"
	AvailableReleasesMockKey = "availableReleases"
)

var (
	mock                 *Mock
	replicatedSecretLock = sync.Mutex{}

	//go:embed default_mock_data.yaml
	defaultMockDataYAML []byte
)

type Mock struct {
	clientset kubernetes.Interface
	namespace string
}

func InitMock(clientset kubernetes.Interface, namespace string) {
	mock = &Mock{
		clientset: clientset,
		namespace: namespace,
	}
}

func MustGetMock() *Mock {
	if mock == nil {
		panic("mock not initialized")
	}
	return mock
}

type MockData struct {
	HelmChartURL      string        `json:"helmChartURL,omitempty" yaml:"helmChartURL,omitempty"`
	CurrentRelease    *MockRelease  `json:"currentRelease,omitempty" yaml:"currentRelease,omitempty"`
	DeployedReleases  []MockRelease `json:"deployedReleases,omitempty" yaml:"deployedReleases,omitempty"`
	AvailableReleases []MockRelease `json:"availableReleases,omitempty" yaml:"availableReleases,omitempty"`
}

type MockRelease struct {
	VersionLabel         string `json:"versionLabel" yaml:"versionLabel"`
	ReleaseNotes         string `json:"releaseNotes" yaml:"releaseNotes"`
	CreatedAt            string `json:"createdAt" yaml:"createdAt"`
	DeployedAt           string `json:"deployedAt" yaml:"deployedAt"`
	HelmReleaseName      string `json:"helmReleaseName" yaml:"helmReleaseName"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision" yaml:"helmReleaseRevision"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace" yaml:"helmReleaseNamespace"`
}

func (m *Mock) UseMockData(ctx context.Context) (bool, error) {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(ctx, replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get replicated secret")
	}

	v := secret.Data[replicatedMockEnabledKey]
	if len(v) == 0 {
		return false, nil
	}

	enabled, _ := strconv.ParseBool(string(v))
	return enabled, nil
}

func (m *Mock) GetHelmChartURL(ctx context.Context) (string, error) {
	mockData, err := m.GetMockData(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return "", nil
	}

	return mockData.HelmChartURL, nil
}

func (m *Mock) GetCurrentRelease(ctx context.Context) (*MockRelease, error) {
	mockData, err := m.GetMockData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.CurrentRelease, nil
}

func (m *Mock) GetAvailableReleases(ctx context.Context) ([]MockRelease, error) {
	mockData, err := m.GetMockData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.AvailableReleases, nil
}

func (m *Mock) GetDeployedReleases(ctx context.Context) ([]MockRelease, error) {
	mockData, err := m.GetMockData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.DeployedReleases, nil
}

func (m *Mock) SetMockData(ctx context.Context, mockData MockData) error {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	b, err := yaml.Marshal(mockData)
	if err != nil {
		return errors.Wrap(err, "failed to marshal mock data")
	}

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(ctx, replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			data := map[string][]byte{
				replicatedMockDataKey: b,
			}
			err = m.createReplicatedSecret(ctx, data)
			if err != nil {
				return errors.Wrap(err, "failed to create secret replicated")
			}
			return nil
		}

		return errors.Wrap(err, "failed to get replicated secret")
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	secret.Data[replicatedMockDataKey] = b
	_, err = m.clientset.CoreV1().Secrets(m.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update replicated secret")
	}

	return nil
}

func (m *Mock) GetMockData(ctx context.Context) (*MockData, error) {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	content := defaultMockDataYAML

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(ctx, replicatedSecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get replicated secret")
	}
	if err == nil {
		b := secret.Data[replicatedMockDataKey]
		if len(b) != 0 {
			content = b
		}
	}

	var mockData MockData
	if err := yaml.Unmarshal(content, &mockData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal mock data")
	}

	return &mockData, nil
}

func (m *Mock) createReplicatedSecret(ctx context.Context, data map[string][]byte) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      replicatedSecretName,
			Namespace: m.namespace,
		},
		Data: data,
	}

	_, err := m.clientset.CoreV1().Secrets(m.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret replicated")
	}

	return nil
}
