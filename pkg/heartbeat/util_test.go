package heartbeat

import (
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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
				"REPLICATED_SDK_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "1"),
				createTestReplicaSet("test-replicaset", "test-namespace", "1"),
				createTestPod("test-pod", "test-namespace", "test-replicaset"),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "one pod, one replicaset, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_SDK_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "2"),
				createTestReplicaSet("test-replicaset", "test-namespace", "1"),
				createTestPod("test-pod", "test-namespace", "test-replicaset"),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
		{
			name: "one pod, two replicasets, revision matches deployment revision",
			env: map[string]string{
				"REPLICATED_SDK_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "2"),
				createTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				createTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				createTestPod("test-pod", "test-namespace", "test-replicaset-bar"),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "one pod, two replicasets, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_SDK_POD_NAME": "test-pod",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "2"),
				createTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				createTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				createTestPod("test-pod", "test-namespace", "test-replicaset-foo"),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
		{
			name: "two pods, two replicasets, revision matches deployment revision",
			env: map[string]string{
				"REPLICATED_SDK_POD_NAME": "test-pod-bar",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "2"),
				createTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				createTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				createTestPod("test-pod-foo", "test-namespace", "test-replicaset-foo"),
				createTestPod("test-pod-bar", "test-namespace", "test-replicaset-bar"),
			),
			namespace: "test-namespace",
			want:      true,
			wantErr:   false,
		},
		{
			name: "two pods, two replicasets, revision does not match deployment revision",
			env: map[string]string{
				"REPLICATED_SDK_POD_NAME": "test-pod-foo",
			},
			clientset: fake.NewSimpleClientset(
				createTestDeployment("replicated-sdk", "test-namespace", "2"),
				createTestReplicaSet("test-replicaset-foo", "test-namespace", "1"),
				createTestReplicaSet("test-replicaset-bar", "test-namespace", "2"),
				createTestPod("test-pod-foo", "test-namespace", "test-replicaset-foo"),
				createTestPod("test-pod-bar", "test-namespace", "test-replicaset-bar"),
			),
			namespace: "test-namespace",
			want:      false,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			defer func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()

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

func createTestDeployment(name string, namespace string, revision string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": revision,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-app"},
			},
		},
	}
}

func createTestReplicaSet(name string, namespace string, revision string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": revision,
			},
		},
	}
}

func createTestPod(name string, namespace string, replicaSetName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       replicaSetName,
				},
			},
		},
	}
}
