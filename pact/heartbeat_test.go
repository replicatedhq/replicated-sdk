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
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSendAppHeartbeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := mock_store.NewMockStore(ctrl)
	clientset := fake.NewSimpleClientset(
		k8sutil.CreateTestDeployment("replicated-sdk", "sdk-heartbeat-namespace", "1", map[string]string{"app": "sdk-heartbeat-app"}),
		k8sutil.CreateTestReplicaSet("sdk-heartbeat-replicaset", "sdk-heartbeat-namespace", "1"),
		k8sutil.CreateTestPod("sdk-heartbeat-pod", "sdk-heartbeat-namespace", "sdk-heartbeat-replicaset", map[string]string{"app": "sdk-heartbeat-app"}),
	)

	t.Setenv("REPLICATED_SDK_POD_NAME", "sdk-heartbeat-pod")

	tests := []struct {
		name                  string
		mockStoreExpectations func()
		pactInteraction       func()
		wantErr               bool
	}{
		{
			name: "successful heartbeat",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-0-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Return("sdk-heartbeat-namespace")
				mockStore.EXPECT().GetReplicatedSDKID().Return("sdk-heartbeat-cluster-id")
				mockStore.EXPECT().GetAppID().Return("sdk-heartbeat-app")
				mockStore.EXPECT().GetChannelID().Return("sdk-heartbeat-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Nightly")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "sdk-heartbeat-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is not expired").
					UponReceiving("A heartbeat from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-heartbeat-customer-0-license", "sdk-heartbeat-customer-0-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("sdk-heartbeat-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("sdk-heartbeat-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("sdk-heartbeat-app-nightly"),
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
			name: "expired license heartbeat should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-2-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Return("sdk-heartbeat-namespace")
				mockStore.EXPECT().GetReplicatedSDKID().Return("sdk-heartbeat-cluster-id")
				mockStore.EXPECT().GetAppID().Return("sdk-heartbeat-app")
				mockStore.EXPECT().GetChannelID().Return("sdk-heartbeat-app-beta")
				mockStore.EXPECT().GetChannelName().Return("Beta")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "sdk-heartbeat-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is expired").
					UponReceiving("A heartbeat from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-heartbeat-customer-2-license", "sdk-heartbeat-customer-2-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("sdk-heartbeat-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("sdk-heartbeat-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("sdk-heartbeat-app-beta"),
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
			name: "nonexistent license heartbeat should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-nonexistent-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Return("sdk-heartbeat-namespace")
				mockStore.EXPECT().GetReplicatedSDKID().Return("sdk-heartbeat-cluster-id")
				mockStore.EXPECT().GetAppID().Return("sdk-heartbeat-app")
				mockStore.EXPECT().GetChannelID().Return("sdk-heartbeat-app-beta")
				mockStore.EXPECT().GetChannelName().Return("Beta")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "sdk-heartbeat-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License does not exist").
					UponReceiving("A heartbeat from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-heartbeat-customer-nonexistent-license", "sdk-heartbeat-customer-nonexistent-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("sdk-heartbeat-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("sdk-heartbeat-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("sdk-heartbeat-app-beta"),
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
			name: "unauthenticated heartbeat should return error",
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-0-license",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				})
				mockStore.EXPECT().GetNamespace().Return("sdk-heartbeat-namespace")
				mockStore.EXPECT().GetReplicatedSDKID().Return("sdk-heartbeat-cluster-id")
				mockStore.EXPECT().GetAppID().Return("sdk-heartbeat-app")
				mockStore.EXPECT().GetChannelID().Return("sdk-heartbeat-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Nightly")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "sdk-heartbeat-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists and is not expired").
					UponReceiving("An unauthenticated heartbeat from the Replicated SDK").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("sdk-heartbeat-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("sdk-heartbeat-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("sdk-heartbeat-app-nightly"),
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
				if err := heartbeat.SendAppHeartbeat(clientset, mockStore); (err != nil) != tt.wantErr {
					t.Errorf("SendAppHeartbeat() error = %v, wantErr %v", err, tt.wantErr)
				}
				return nil
			}); err != nil {
				t.Fatalf("Error on Verify: %v", err)
			}
		})
	}
}
