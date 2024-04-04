package pact

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pact-foundation/pact-go/dsl"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSendInstanceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := mock_store.NewMockStore(ctrl)
	clientset := fake.NewSimpleClientset(
		k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "replicated-sdk-instance-namespace", "1", map[string]string{"app": "replicated-sdk-instance-app"}),
		k8sutil.CreateTestReplicaSet("replicated-sdk-instance-replicaset", "replicated-sdk-instance-namespace", "1"),
		k8sutil.CreateTestPod("replicated-sdk-instance-pod", "replicated-sdk-instance-namespace", "replicated-sdk-instance-replicaset", map[string]string{"app": "replicated-sdk-instance-app"}),
	)

	t.Setenv("REPLICATED_POD_NAME", "replicated-sdk-instance-pod")

	tests := []struct {
		name                  string
		mockStoreExpectations func()
		pactInteraction       func()
		wantErr               bool
	}{
		{
			name: "successful instance data request",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "replicated-sdk-instance-customer-0-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Times(2).Return("replicated-sdk-instance-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("replicated-sdk-instance-cluster-id")
				mockStore.EXPECT().GetAppID().Return("replicated-sdk-instance-app")
				mockStore.EXPECT().GetChannelID().Return("replicated-sdk-instance-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Nightly")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "replicated-sdk-instance-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is not expired").
					UponReceiving("Instance data from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "replicated-sdk-instance-customer-0-license", "replicated-sdk-instance-customer-0-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("replicated-sdk-instance-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("replicated-sdk-instance-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("replicated-sdk-instance-app-nightly"),
							"X-Replicated-DownstreamChannelSequence": dsl.String("1"),
						},
						Path: dsl.String("/kots_metrics/license_instance/info"),
						Body: map[string]interface{}{
							"resource_states": dsl.String("[]"),
						},
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusOK,
					})
			},
			wantErr: false,
		},
		{
			name: "expired license should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "replicated-sdk-instance-customer-2-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Times(2).Return("replicated-sdk-instance-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("replicated-sdk-instance-cluster-id")
				mockStore.EXPECT().GetAppID().Return("replicated-sdk-instance-app")
				mockStore.EXPECT().GetChannelID().Return("replicated-sdk-instance-app-beta")
				mockStore.EXPECT().GetChannelName().Return("Beta")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "replicated-sdk-instance-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is expired").
					UponReceiving("Instance data from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "replicated-sdk-instance-customer-2-license", "replicated-sdk-instance-customer-2-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("replicated-sdk-instance-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("replicated-sdk-instance-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("replicated-sdk-instance-app-beta"),
							"X-Replicated-DownstreamChannelSequence": dsl.String("1"),
						},
						Path: dsl.String("/kots_metrics/license_instance/info"),
						Body: map[string]interface{}{
							"resource_states": dsl.String("[]"),
						},
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusForbidden,
					})
			},
			wantErr: true,
		},
		{
			name: "nonexistent license should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "replicated-sdk-instance-customer-nonexistent-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Times(2).Return("replicated-sdk-instance-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("replicated-sdk-instance-cluster-id")
				mockStore.EXPECT().GetAppID().Return("replicated-sdk-instance-app")
				mockStore.EXPECT().GetChannelID().Return("replicated-sdk-instance-app-beta")
				mockStore.EXPECT().GetChannelName().Return("Beta")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "replicated-sdk-instance-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License does not exist").
					UponReceiving("Instance data from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "replicated-sdk-instance-customer-nonexistent-license", "replicated-sdk-instance-customer-nonexistent-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("replicated-sdk-instance-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("replicated-sdk-instance-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("replicated-sdk-instance-app-beta"),
							"X-Replicated-DownstreamChannelSequence": dsl.String("1"),
						},
						Path: dsl.String("/kots_metrics/license_instance/info"),
						Body: map[string]interface{}{
							"resource_states": dsl.String("[]"),
						},
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusUnauthorized,
					})
			},
			wantErr: true,
		},
		{
			name: "unauthenticated instance data request should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "replicated-sdk-instance-customer-0-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Times(2).Return("replicated-sdk-instance-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("replicated-sdk-instance-cluster-id")
				mockStore.EXPECT().GetAppID().Return("replicated-sdk-instance-app")
				mockStore.EXPECT().GetChannelID().Return("replicated-sdk-instance-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Nightly")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "replicated-sdk-instance-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is not expired").
					UponReceiving("An unauthenticated instance data request from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("replicated-sdk-instance-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("replicated-sdk-instance-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("replicated-sdk-instance-app-nightly"),
							"X-Replicated-DownstreamChannelSequence": dsl.String("1"),
						},
						Path: dsl.String("/kots_metrics/license_instance/info"),
						Body: map[string]interface{}{
							"resource_states": dsl.String("[]"),
						},
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusUnauthorized,
					})
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockStoreExpectations()
			tt.pactInteraction()
			if err := pact.Verify(func() error {
				if err := report.SendInstanceData(clientset, mockStore); (err != nil) != tt.wantErr {
					t.Errorf("SendInstanceData() error = %v, wantErr %v", err, tt.wantErr)
				}
				return nil
			}); err != nil {
				t.Fatalf("Error on Verify: %v", err)
			}
		})
	}
}
