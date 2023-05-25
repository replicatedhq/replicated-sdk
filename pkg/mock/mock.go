package mock

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	replicatedSecretName  = "replicated"
	replicatedMockDataKey = "REPLICATED_MOCK_DATA"

	CurrentReleaseMockKey    = "currentRelease"
	DeployedReleasesMockKey  = "deployedReleases"
	AvailableReleasesMockKey = "availableReleases"
)

var (
	mock                 *Mock
	replicatedSecretLock = sync.Mutex{}
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
	CurrentRelease    *MockRelease  `json:"currentRelease,omitempty"`
	DeployedReleases  []MockRelease `json:"deployedReleases,omitempty"`
	AvailableReleases []MockRelease `json:"availableReleases,omitempty"`
}

type MockRelease struct {
	VersionLabel         string `json:"versionLabel"`
	ChannelID            string `json:"channelID"`
	ChannelName          string `json:"channelName"`
	IsRequired           bool   `json:"isRequired"`
	ReleaseNotes         string `json:"releaseNotes"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func (m *Mock) HasMockData(mockType string) (bool, error) {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(context.TODO(), replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get secret replicated-dev")
	}

	b := secret.Data[replicatedMockDataKey]
	if len(b) == 0 {
		return false, nil
	}

	mockDataMap := make(map[string]interface{})
	if err := json.Unmarshal(b, &mockDataMap); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal mock data")
	}

	_, exists := mockDataMap[mockType]
	return exists, nil
}

func (m *Mock) GetCurrentRelease() (*MockRelease, error) {
	mockData, err := m.GetMockData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.CurrentRelease, nil
}

func (m *Mock) GetAvailableReleases() ([]MockRelease, error) {
	mockData, err := m.GetMockData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.AvailableReleases, nil
}

func (m *Mock) GetDeployedReleases() ([]MockRelease, error) {
	mockData, err := m.GetMockData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return nil, nil
	}

	return mockData.DeployedReleases, nil
}

func (m *Mock) SetMockData(mockData MockData) error {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	b, err := json.Marshal(mockData)
	if err != nil {
		return errors.Wrap(err, "failed to marshal mock data")
	}

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(context.TODO(), replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			data := map[string][]byte{
				replicatedMockDataKey: b,
			}
			err = m.createReplicatedSecret(data)
			if err != nil {
				return errors.Wrap(err, "failed to create secret replicated")
			}
			return nil
		}

		return errors.Wrap(err, "failed to get secret replicated-dev")
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	secret.Data[replicatedMockDataKey] = b
	_, err = m.clientset.CoreV1().Secrets(m.namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update secret replicated-dev")
	}

	return nil
}

func (m *Mock) GetMockData() (*MockData, error) {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(context.TODO(), replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get secret replicated-dev")
	}

	b := secret.Data[replicatedMockDataKey]
	if len(b) == 0 {
		return nil, nil
	}

	var mockData MockData
	if err := json.Unmarshal(b, &mockData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal mock data")
	}

	return &mockData, nil
}

func (m *Mock) DeleteMockData() error {
	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(context.TODO(), replicatedSecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "failed to get secret replicated-dev")
	}

	delete(secret.Data, replicatedMockDataKey)
	_, err = m.clientset.CoreV1().Secrets(m.namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to update secret replicated-dev")
	}
	return nil
}

func (m *Mock) createReplicatedSecret(data map[string][]byte) error {
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

	_, err := m.clientset.CoreV1().Secrets(m.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret replicated")
	}

	return nil
}
