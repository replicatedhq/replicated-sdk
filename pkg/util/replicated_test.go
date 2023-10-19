package util

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetReplicatedAndAppIDs(t *testing.T) {
	type args struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name             string
		args             args
		wantReplicatedID string
		wantAppID        string
		wantErr          bool
	}{
		{
			name: "get ids from legacy configmap if it exists",
			args: args{
				clientset: fake.NewSimpleClientset(
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      GetLegacyReplicatedConfigMapName(),
							Namespace: "default",
						},
						Data: map[string]string{
							"replicated-sdk-id": "legacy-replicated-id",
							"app-id":            "legacy-app-id",
						},
					},
				),
				namespace: "default",
			},
			wantReplicatedID: "legacy-replicated-id",
			wantAppID:        "legacy-app-id",
		},
		{
			name: "get ids from deployment uid if legacy configmap does not exist",
			args: args{
				clientset: fake.NewSimpleClientset(
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      GetReplicatedDeploymentName(),
							Namespace: "default",
							UID:       "replicated-id",
						},
					},
				),
				namespace: "default",
			},
			wantReplicatedID: "replicated-id",
			wantAppID:        "replicated-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReplicatedID, gotAppID, err := GetReplicatedAndAppIDs(tt.args.clientset, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReplicatedAndAppIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotReplicatedID != tt.wantReplicatedID {
				t.Errorf("GetReplicatedAndAppIDs() got = %v, want %v", gotReplicatedID, tt.wantReplicatedID)
			}
			if gotAppID != tt.wantAppID {
				t.Errorf("GetReplicatedAndAppIDs() got1 = %v, want %v", gotAppID, tt.wantAppID)
			}
		})
	}
}

func TestGetReplicatedDeploymentUID(t *testing.T) {
	type args struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    apimachinerytypes.UID
		wantErr bool
	}{
		{
			name: "deployment exists",
			args: args{
				clientset: fake.NewSimpleClientset(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      GetReplicatedDeploymentName(),
						Namespace: "default",
						UID:       "test-uid",
					},
				}),
				namespace: "default",
			},
			want: "test-uid",
		},
		{
			name: "deployment does not exist",
			args: args{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetReplicatedDeploymentUID(tt.args.clientset, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReplicatedDeploymentUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetReplicatedDeploymentUID() = %v, want %v", got, tt.want)
			}
		})
	}
}
