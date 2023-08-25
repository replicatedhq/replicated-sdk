package integration

import (
	"context"
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIntegration_IsEnabled(t *testing.T) {
	type args struct {
		clientset kubernetes.Interface
		namespace string
		license   *kotsv1beta1.License
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "is not enabled",
			args: args{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "is enabled",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedSecretName(),
							Namespace: "default",
						},
						Data: map[string][]byte{
							integrationEnabledKey: []byte("true"),
						},
					}},
				}),
				namespace: "default",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "dev",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not enabled because not a dev license",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedSecretName(),
							Namespace: "default",
						},
						Data: map[string][]byte{
							integrationEnabledKey: []byte("true"),
						},
					}},
				}),
				namespace: "default",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "paid",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "not enabled for a dev license",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedSecretName(),
							Namespace: "default",
						},
						Data: map[string][]byte{
							integrationEnabledKey: []byte("false"),
						},
					}},
				}),
				namespace: "default",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "dev",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "enabled for a dev license because key doesn't exist",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedSecretName(),
							Namespace: "default",
						},
						Data: map[string][]byte{},
					}},
				}),
				namespace: "default",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "dev",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "enabled for a dev license because value is empty",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedSecretName(),
							Namespace: "default",
						},
						Data: map[string][]byte{
							integrationEnabledKey: []byte(""),
						},
					}},
				}),
				namespace: "default",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "dev",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsEnabled(context.Background(), tt.args.clientset, tt.args.namespace, tt.args.license)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsEnabled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
