package mock

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

//go:embed test_mock_data.yaml
var testMockDataYAML []byte

func TestMock_IsMockEnabled(t *testing.T) {
	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	type args struct {
		license *kotsv1beta1.License
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "is not enabled",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "is enabled",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockEnabledKey: []byte("true"),
						},
					}},
				}),
				namespace: "default",
			},
			args: args{
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
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockEnabledKey: []byte("true"),
						},
					}},
				}),
				namespace: "default",
			},
			args: args{
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
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockEnabledKey: []byte("false"),
						},
					}},
				}),
				namespace: "default",
			},
			args: args{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseType: "dev",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().IsMockEnabled(context.Background(), tt.args.license)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.IsMockEnabled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Mock.IsMockEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMock_GetHelmChartURL(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "default url",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    defaultMockData.HelmChartURL,
			wantErr: false,
		},
		{
			name: "custom mock data url",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockDataKey: []byte(testMockDataYAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockData.HelmChartURL,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().GetHelmChartURL(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetHelmChartURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Mock.GetHelmChartURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMock_GetCurrentRelease(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *MockRelease
		wantErr bool
	}{
		{
			name: "default current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    defaultMockData.CurrentRelease,
			wantErr: false,
		},
		{
			name: "custom mock data current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockDataKey: []byte(testMockDataYAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockData.CurrentRelease,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().GetCurrentRelease(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetCurrentRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Mock.GetCurrentRelease() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func TestMock_GetAvailableReleases(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []MockRelease
		wantErr bool
	}{
		{
			name: "default current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    defaultMockData.AvailableReleases,
			wantErr: false,
		},
		{
			name: "custom mock data current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockDataKey: []byte(testMockDataYAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockData.AvailableReleases,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().GetAvailableReleases(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetAvailableReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Mock.GetAvailableReleases() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func TestMock_GetDeployedReleases(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []MockRelease
		wantErr bool
	}{
		{
			name: "default current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    defaultMockData.DeployedReleases,
			wantErr: false,
		},
		{
			name: "custom mock data current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockDataKey: []byte(testMockDataYAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockData.DeployedReleases,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().GetDeployedReleases(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetDeployedReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Mock.GetDeployedReleases() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func TestMock_GetMockData(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *MockData
		wantErr bool
	}{
		{
			name: "default current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			want:    defaultMockData,
			wantErr: false,
		},
		{
			name: "custom mock data current release",
			fields: fields{
				clientset: fake.NewSimpleClientset(&corev1.SecretList{
					TypeMeta: metav1.TypeMeta{},
					ListMeta: metav1.ListMeta{},
					Items: []corev1.Secret{{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicatedSecretName,
							Namespace: "default",
						},
						Data: map[string][]byte{
							replicatedMockDataKey: []byte(testMockDataYAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockData,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			got, err := MustGetMock().GetMockData(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.GetMockData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Mock.GetMockData() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func TestMock_SetMockData(t *testing.T) {
	testMockData, err := GetTestMockData()
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	type args struct {
		mockData *MockData
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *MockData
		wantErr bool
	}{
		{
			name: "creates the replicated secret with the mock data",
			fields: fields{
				clientset: fake.NewSimpleClientset(),
				namespace: "default",
			},
			args: args{
				mockData: testMockData,
			},
			want:    testMockData,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitMock(tt.fields.clientset, tt.fields.namespace)

			err := MustGetMock().SetMockData(context.Background(), *tt.args.mockData)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mock.SetMockData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			secret, err := tt.fields.clientset.CoreV1().Secrets(tt.fields.namespace).Get(context.Background(), replicatedSecretName, metav1.GetOptions{})
			require.NoError(t, err)

			var got MockData
			err = yaml.Unmarshal(secret.Data[replicatedMockDataKey], &got)
			require.NoError(t, err)

			if !reflect.DeepEqual(tt.want, &got) {
				t.Errorf("Mock.SetMockData() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func GetTestMockData() (*MockData, error) {
	var testMockData *MockData
	err := yaml.Unmarshal([]byte(testMockDataYAML), &testMockData)
	if err != nil {
		return nil, err
	}
	return testMockData, nil
}

func fmtJSONDiff(got, want interface{}) string {
	a, _ := json.MarshalIndent(got, "", "  ")
	b, _ := json.MarshalIndent(want, "", "  ")
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(a)),
		B:        difflib.SplitLines(string(b)),
		FromFile: "Got",
		ToFile:   "Want",
		Context:  1,
	}
	diffStr, _ := difflib.GetUnifiedDiffString(diff)
	return fmt.Sprintf("got:\n%s \n\nwant:\n%s \n\ndiff:\n%s", a, b, diffStr)
}
