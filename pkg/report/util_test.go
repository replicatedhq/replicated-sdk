package report

import (
	"testing"

	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/report/types"
	tagstypes "github.com/replicatedhq/replicated-sdk/pkg/tags/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInjectInstanceDataPayload(t *testing.T) {
	instanceData := &types.InstanceData{
		AppStatus: "ready",
		ResourceStates: appstatetypes.ResourceStates{
			{
				Kind:      "Deployment",
				Name:      "test-deployment",
				Namespace: "test-namespace",
				State:     appstatetypes.StateDegraded,
			},
			{
				Kind:      "Service",
				Name:      "test-service",
				Namespace: "test-namespace",
				State:     appstatetypes.StateUnavailable,
			},
		},
	}

	reqPayload := make(map[string]interface{})

	err := InjectInstanceDataPayload(reqPayload, instanceData)
	assert.NoError(t, err)

	expectedPayload := map[string]interface{}{
		"resource_states": `[{"kind":"Deployment","name":"test-deployment","namespace":"test-namespace","state":"degraded"},{"kind":"Service","name":"test-service","namespace":"test-namespace","state":"unavailable"}]`,
	}
	assert.Equal(t, expectedPayload, reqPayload)

	// test nil instance data
	reqPayload = make(map[string]interface{})

	err = InjectInstanceDataPayload(reqPayload, nil)
	assert.NoError(t, err)

	expectedPayload = map[string]interface{}{}
	assert.Equal(t, expectedPayload, reqPayload)

	// test empty app status
	reqPayload = make(map[string]interface{})

	err = InjectInstanceDataPayload(reqPayload, &types.InstanceData{
		AppStatus: "",
	})
	assert.NoError(t, err)

	expectedPayload = map[string]interface{}{}
	assert.Equal(t, expectedPayload, reqPayload)
}

func TestGetInstanceDataHeaders(t *testing.T) {
	instanceData := &types.InstanceData{
		AppStatus:       "ready",
		ClusterID:       "cluster-123",
		InstanceID:      "instance-456",
		ChannelID:       "channel-789",
		ChannelSequence: 42,
		K8sVersion:      "v1.20.2+k3s1",
		K8sDistribution: "k3s",
		Tags:            tagstypes.InstanceTagData{Force: true, Tags: map[string]string{"key": "value"}},
	}

	headers := GetInstanceDataHeaders(instanceData)

	expectedHeaders := map[string]string{
		"X-Replicated-K8sVersion":                "v1.20.2+k3s1",
		"X-Replicated-AppStatus":                 "ready",
		"X-Replicated-ClusterID":                 "cluster-123",
		"X-Replicated-InstanceID":                "instance-456",
		"X-Replicated-DownstreamChannelID":       "channel-789",
		"X-Replicated-DownstreamChannelSequence": "42",
		"X-Replicated-K8sDistribution":           "k3s",
		"X-Replicated-InstanceTagData":           "eyJmb3JjZSI6dHJ1ZSwidGFncyI6eyJrZXkiOiJ2YWx1ZSJ9fQ==",
	}
	assert.Equal(t, expectedHeaders, headers)

	// nil instance data
	nilHeaders := GetInstanceDataHeaders(nil)
	assert.Empty(t, nilHeaders)

	// empty app status
	emptyAppStatusHeaders := GetInstanceDataHeaders(&types.InstanceData{
		AppStatus: "",
	})
	_, appStatusOk := emptyAppStatusHeaders["X-Replicated-AppStatus"]
	assert.False(t, appStatusOk)
}

func TestCanReport(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		clientset *fake.Clientset
		namespace string
		want      bool
		wantErr   bool
	}{
		{
			name: "one pod, one replicaset, revision matches deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "1", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
				k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "one pod, one replicaset, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "2", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
				k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
		{
			name: "one pod, two replicasets, revision matches deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "2", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				k8sutil.CreateTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset-bar", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "one pod, two replicasets, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "2", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				k8sutil.CreateTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset-foo", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
		{
			name: "two pods, two replicasets, revision matches deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod-bar",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "2", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				k8sutil.CreateTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				k8sutil.CreateTestPod("test-pod-foo", "test-namespace", "test-replicaset-foo", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestPod("test-pod-bar", "test-namespace", "test-replicaset-bar", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "two pods, two replicasets, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_POD_NAME": "test-pod-foo",
			},
			clientset: fake.NewSimpleClientset(
				k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "2", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				k8sutil.CreateTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				k8sutil.CreateTestPod("test-pod-foo", "test-namespace", "test-replicaset-foo", map[string]string{"app": "test-app"}),
				k8sutil.CreateTestPod("test-pod-bar", "test-namespace", "test-replicaset-bar", map[string]string{"app": "test-app"}),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := canReport(tt.clientset, tt.namespace, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("canReport() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("canReport() = %v, want %v", got, tt.want)
			}
		})
	}
}
