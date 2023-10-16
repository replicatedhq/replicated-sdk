package heartbeat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_SendAppHeartbeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStore := mock_store.NewMockStore(ctrl)

	respRecorder := httptest.NewRecorder()
	mockRouter := mux.NewRouter()
	mockServer := httptest.NewServer(mockRouter)
	defer mockServer.Close()
	mockRouter.Methods("POST").Path("/airgap/metrics").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respRecorder.Write([]byte("airgap"))
		w.WriteHeader(http.StatusOK)
	})
	mockRouter.Methods("POST").Path("/kots_metrics/license_instance/info").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respRecorder.Write([]byte("online"))
		w.WriteHeader(http.StatusOK)
	})

	type args struct {
		clientset kubernetes.Interface
		sdkStore  store.Store
	}
	tests := []struct {
		name                  string
		args                  args
		airgap                bool
		mockStoreExpectations func()
		wantResponseFrom      string
		wantErr               bool
	}{
		{
			name: "online heartbeat",
			args: args{
				clientset: fake.NewSimpleClientset(
					k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "1", map[string]string{"app": "test-app"}),
					k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
					k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
				),
				sdkStore: mockStore,
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "test-license-id",
						Endpoint:  mockServer.URL,
					},
				})
				mockStore.EXPECT().GetNamespace().Return("test-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("test-cluster-id")
				mockStore.EXPECT().GetAppID().Return("test-app")
				mockStore.EXPECT().GetChannelID().Return("test-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Test Channel")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "test-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			wantResponseFrom: "online",
		},
		{
			name: "airgap heartbeat",
			args: args{
				clientset: fake.NewSimpleClientset(
					k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "1", map[string]string{"app": "test-app"}),
					k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
					k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
				),
				sdkStore: mockStore,
			},
			airgap: true,
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "test-license-id",
						Endpoint:  mockServer.URL,
					},
				})
				mockStore.EXPECT().GetNamespace().Return("test-namespace")
				mockStore.EXPECT().GetReplicatedID().Return("test-cluster-id")
				mockStore.EXPECT().GetAppID().Return("test-app")
				mockStore.EXPECT().GetChannelID().Return("test-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Test Channel")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "test-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			wantResponseFrom: "airgap",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respRecorder.Flush()
			tt.mockStoreExpectations()

			if tt.airgap {
				t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "true")
				t.Setenv("REPLICATED_AIRGAP_HEARTBEAT_ENDPOINT", mockServer.URL+"/airgap/metrics")
			}

			if err := SendAppHeartbeat(tt.args.clientset, tt.args.sdkStore); (err != nil) != tt.wantErr {
				t.Errorf("SendAppHeartbeat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotResponseFrom := respRecorder.Body.String(); gotResponseFrom != tt.wantResponseFrom {
				t.Errorf("SendAppHeartbeat() got response from %v, wanted %v", gotResponseFrom, tt.wantResponseFrom)
			}
		})
	}
}
