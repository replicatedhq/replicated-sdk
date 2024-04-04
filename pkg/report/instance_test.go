package report

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
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_SendInstanceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStore := mock_store.NewMockStore(ctrl)

	respRecorder := httptest.NewRecorder()
	mockRouter := mux.NewRouter()
	mockServer := httptest.NewServer(mockRouter)
	defer mockServer.Close()
	mockRouter.Methods("POST").Path("/kots_metrics/license_instance/info").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respRecorder.Write([]byte("received instance data"))
		w.WriteHeader(http.StatusOK)
	})

	type args struct {
		clientset kubernetes.Interface
		sdkStore  store.Store
	}
	tests := []struct {
		name                  string
		args                  args
		env                   map[string]string
		isAirgap              bool
		mockStoreExpectations func()
	}{
		{
			name: "send online instance data",
			args: args{
				clientset: fake.NewSimpleClientset(
					k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "1", map[string]string{"app": "test-app"}),
					k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
					k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
				),
				sdkStore: mockStore,
			},
			env: map[string]string{
				"DISABLE_OUTBOUND_CONNECTIONS": "false",
				"REPLICATED_POD_NAME":          "test-pod",
			},
			isAirgap: false,
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "test-license-id",
						Endpoint:  mockServer.URL,
					},
				})
				mockStore.EXPECT().GetNamespace().Times(2).Return("test-namespace")
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
		},
		{
			name: "send airgap instance data",
			args: args{
				clientset: fake.NewSimpleClientset(
					k8sutil.CreateTestDeployment(util.GetReplicatedDeploymentName(), "test-namespace", "1", map[string]string{"app": "test-app"}),
					k8sutil.CreateTestReplicaSet("test-replicaset", "test-namespace", "1"),
					k8sutil.CreateTestPod("test-pod", "test-namespace", "test-replicaset", map[string]string{"app": "test-app"}),
				),
				sdkStore: mockStore,
			},
			env: map[string]string{
				"DISABLE_OUTBOUND_CONNECTIONS": "true",
				"REPLICATED_POD_NAME":          "test-pod",
			},
			isAirgap: true,
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "test-license-id",
						Endpoint:  mockServer.URL,
					},
				})
				mockStore.EXPECT().GetNamespace().Times(3).Return("test-namespace")
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			respRecorder.Body.Reset()

			tt.mockStoreExpectations()

			err := SendInstanceData(tt.args.clientset, tt.args.sdkStore)
			req.NoError(err)

			if !tt.isAirgap {
				req.Equal("received instance data", respRecorder.Body.String())
			} else {
				req.Equal("", respRecorder.Body.String())
			}
		})
	}
}
