package integration

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
	integrationtypes "github.com/replicatedhq/replicated-sdk/pkg/integration/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

//go:embed data/test_mock_data.yaml
var testMockDataYAML []byte

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
							replicatedIntegrationMockDataKey: []byte(testMockDataYAML),
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
			got, err := GetHelmChartURL(context.Background(), tt.fields.clientset, tt.fields.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHelmChartURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHelmChartURL() = %v, want %v", got, tt.want)
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
		want    *integrationtypes.MockRelease
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
							replicatedIntegrationMockDataKey: []byte(testMockDataYAML),
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
			got, err := GetCurrentRelease(context.Background(), tt.fields.clientset, tt.fields.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("GetCurrentRelease() \n\n%s", fmtJSONDiff(got, tt.want))
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
		want    []integrationtypes.MockRelease
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
							replicatedIntegrationMockDataKey: []byte(testMockDataYAML),
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
			got, err := GetAvailableReleases(context.Background(), tt.fields.clientset, tt.fields.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAvailableReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("GetAvailableReleases() \n\n%s", fmtJSONDiff(got, tt.want))
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
		want    []integrationtypes.MockRelease
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
							replicatedIntegrationMockDataKey: []byte(testMockDataYAML),
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
			got, err := GetDeployedReleases(context.Background(), tt.fields.clientset, tt.fields.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeployedReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("GetDeployedReleases() \n\n%s", fmtJSONDiff(got, tt.want))
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
		want    *integrationtypes.MockData
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
							replicatedIntegrationMockDataKey: []byte(testMockDataYAML),
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
			got, err := GetMockData(context.Background(), tt.fields.clientset, tt.fields.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMockData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("GetMockData() \n\n%s", fmtJSONDiff(got, tt.want))
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
		mockData *integrationtypes.MockData
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *integrationtypes.MockData
		wantErr bool
	}{
		{
			name: "updates the replicated secret with the mock data",
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
						Data: map[string][]byte{},
					}},
				}),
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
			err := SetMockData(context.Background(), tt.fields.clientset, tt.fields.namespace, *tt.args.mockData)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetMockData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			secret, err := tt.fields.clientset.CoreV1().Secrets(tt.fields.namespace).Get(context.Background(), replicatedSecretName, metav1.GetOptions{})
			require.NoError(t, err)

			var got integrationtypes.MockData
			err = yaml.Unmarshal(secret.Data[replicatedIntegrationMockDataKey], &got)
			require.NoError(t, err)

			if !reflect.DeepEqual(tt.want, &got) {
				t.Errorf("SetMockData() \n\n%s", fmtJSONDiff(got, tt.want))
			}
		})
	}
}

func GetTestMockData() (*integrationtypes.MockData, error) {
	var testMockData *integrationtypes.MockData
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
