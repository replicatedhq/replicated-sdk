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
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

//go:embed data/test_mock_data_v1.yaml
var testMockDataV1YAML []byte

//go:embed data/test_mock_data_v2.yaml
var testMockDataV2YAML []byte

func TestMock_GetMockData(t *testing.T) {
	defaultMockData, err := GetDefaultMockData(context.Background())
	require.NoError(t, err)

	testMockDataV1, err := GetTestMockData(testMockDataV1YAML)
	require.NoError(t, err)

	testMockDataV2, err := GetTestMockData(testMockDataV2YAML)
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    integrationtypes.MockData
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
			name: "custom v1 mock data current release",
			fields: fields{
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
							integrationMockDataKey: []byte(testMockDataV1YAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockDataV1,
			wantErr: false,
		},
		{
			name: "custom v2 mock data current release",
			fields: fields{
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
							integrationMockDataKey: []byte(testMockDataV2YAML),
						},
					}},
				}),
				namespace: "default",
			},
			want:    testMockDataV2,
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
	testMockDataV1, err := GetTestMockData(testMockDataV1YAML)
	require.NoError(t, err)

	testMockDataV2, err := GetTestMockData(testMockDataV2YAML)
	require.NoError(t, err)

	type fields struct {
		clientset kubernetes.Interface
		namespace string
	}
	type args struct {
		mockData integrationtypes.MockData
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		validate func(t *testing.T, got integrationtypes.MockData)
	}{
		{
			name: "updates the replicated secret with the mock data v1",
			fields: fields{
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
			},
			args: args{
				mockData: testMockDataV1,
			},
			validate: func(t *testing.T, got integrationtypes.MockData) {
				gotV1, ok := got.(*integrationtypes.MockDataV1)
				if !ok {
					t.Errorf("SetMockData() expected type %T, got %T", &integrationtypes.MockDataV1{}, got)
				}
				require.True(t, ok)
				if !reflect.DeepEqual(testMockDataV1, gotV1) {
					t.Errorf("SetMockData() \n\n%q", fmtJSONDiff(gotV1, testMockDataV1))
				}
			},
		},
		{
			name: "updates the replicated secret with the mock data v2",
			fields: fields{
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
			},
			args: args{
				mockData: testMockDataV2,
			},
			validate: func(t *testing.T, got integrationtypes.MockData) {
				gotV2, ok := got.(*integrationtypes.MockDataV2)
				if !ok {
					t.Errorf("SetMockData() expected type %T, got %T", &integrationtypes.MockDataV2{}, got)
				}
				require.True(t, ok)
				if !reflect.DeepEqual(testMockDataV2, gotV2) {
					t.Errorf("SetMockData() \n\n%q", fmtJSONDiff(gotV2, testMockDataV2))
				}
			},
			// want:    testMockDataV2,
			// wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetMockData(context.Background(), tt.fields.clientset, tt.fields.namespace, tt.args.mockData)

			secret, err := tt.fields.clientset.CoreV1().Secrets(tt.fields.namespace).Get(context.Background(), util.GetReplicatedSecretName(), metav1.GetOptions{})
			require.NoError(t, err)

			got, err := UnmarshalYAML(secret.Data[integrationMockDataKey])
			require.NoError(t, err)

			tt.validate(t, got)
		})
	}
}

func GetTestMockData(b []byte) (integrationtypes.MockData, error) {
	testMockData, err := UnmarshalYAML(b)
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
