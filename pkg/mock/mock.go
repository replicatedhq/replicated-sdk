package mock

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	replicatedSecretName  = "replicated"
	replicatedMockDataKey = "REPLICATED_MOCK_DATA"
)

var (
	mock *Mock
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
	CurrentRelease    *MockRelease  `json:"currentRelease"`
	DeployedReleases  []MockRelease `json:"deployedReleases"`
	AvailableReleases []MockRelease `json:"availableReleases"`
}

type MockRelease struct {
	VersionLabel         string `json:"versionLabel"`
	ChannelID            string `json:"channelID"`
	ChannelName          string `json:"channelName"`
	ChannelSequence      int    `json:"channelSequence"`
	ReleaseSequence      int    `json:"releaseSequence"`
	IsRequired           bool   `json:"isRequired"`
	ReleaseNotes         string `json:"releaseNotes"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func (m *Mock) GetCurrentRelease() (bool, *MockRelease, error) {
	mockData, err := m.getMockData()
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return false, nil, nil
	}

	return true, mockData.CurrentRelease, nil
}

func (m *Mock) GetAvailableReleases() (bool, []MockRelease, error) {
	mockData, err := m.getMockData()
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return false, nil, nil
	}

	return true, mockData.AvailableReleases, nil
}

func (m *Mock) GetAllReleases() (bool, []MockRelease, error) {
	mockData, err := m.getMockData()
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to get mock data")
	} else if mockData == nil {
		return false, nil, nil
	}

	releases := []MockRelease{}
	releases = append(releases, mockData.DeployedReleases...)
	releases = append(releases, mockData.AvailableReleases...)

	return true, releases, nil
}

func (m *Mock) getMockData() (*MockData, error) {
	secret, err := m.clientset.CoreV1().Secrets(m.namespace).Get(context.TODO(), replicatedSecretName, metav1.GetOptions{})
	if err != nil {
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
