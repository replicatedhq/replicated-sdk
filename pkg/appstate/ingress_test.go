package appstate

import (
	"reflect"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func mockClientsetK8sVersion(expectedMajor string, expectedMinor string) kubernetes.Interface {
	clientset := fake.NewSimpleClientset()
	clientset.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
		Major: expectedMajor,
		Minor: expectedMinor,
	}
	return clientset
}

func TestCalculateIngressState(t *testing.T) {
	type args struct {
		clientset kubernetes.Interface
		r         *networkingv1.Ingress
	}
	tests := []struct {
		name string
		args args
		want types.State
	}{
		{
			name: "ingress with k8s version < 1.22 and no default backend",
			args: args{
				clientset: mockClientsetK8sVersion("1", "21"),
				r: &networkingv1.Ingress{
					Spec: networkingv1.IngressSpec{},
				},
			},
			want: types.StateUnavailable,
		}, {
			name: "ingress with k8s version > 1.22 and default backend",
			args: args{
				clientset: mockClientsetK8sVersion("1", "23"),
				r: &networkingv1.Ingress{
					Spec: networkingv1.IngressSpec{},
				},
			},
			want: types.StateUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateIngressState(tt.args.clientset, tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalculateIngressState() = %v, want %v", got, tt.want)
			}
		})
	}
}
