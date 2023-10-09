package heartbeat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetHeartbeatInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStore := mock_store.NewMockStore(ctrl)

	mockRouter := mux.NewRouter()
	mockServer := httptest.NewServer(mockRouter)
	defer mockServer.Close()
	mockRouter.Methods("GET").Path("/metrics").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"X-Replicated-Metric": "test"}`))
		w.WriteHeader(http.StatusOK)
	})

	type args struct {
		sdkStore  store.Store
		clientset kubernetes.Interface
	}
	tests := []struct {
		name                  string
		args                  args
		mockStoreExpectations func()
		want                  *types.HeartbeatInfo
	}{
		{
			name: "with no k8s client or metrics endpoint",
			args: args{
				sdkStore:  mockStore,
				clientset: nil,
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetReplicatedID().Return("sdk-heartbeat-cluster-id")
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
				mockStore.EXPECT().GetAdditionalMetricsEndpoint().Return("")
			},
			want: &types.HeartbeatInfo{
				ClusterID:       "sdk-heartbeat-cluster-id",
				InstanceID:      "sdk-heartbeat-app",
				ChannelID:       "sdk-heartbeat-app-nightly",
				ChannelName:     "Nightly",
				ChannelSequence: 1,
				AppStatus:       string(appstatetypes.StateMissing),
				ResourceStates:  []appstatetypes.ResourceState{},
			},
		},
		{
			name: "with k8s client and no metrics endpoint",
			args: args{
				sdkStore:  mockStore,
				clientset: mockClientset("v1.26.0+k3s"),
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetReplicatedID().Return("sdk-heartbeat-cluster-id")
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
				mockStore.EXPECT().GetAdditionalMetricsEndpoint().Return("")
			},
			want: &types.HeartbeatInfo{
				ClusterID:       "sdk-heartbeat-cluster-id",
				InstanceID:      "sdk-heartbeat-app",
				ChannelID:       "sdk-heartbeat-app-nightly",
				ChannelName:     "Nightly",
				ChannelSequence: 1,
				AppStatus:       string(appstatetypes.StateMissing),
				ResourceStates:  []appstatetypes.ResourceState{},
				K8sVersion:      "v1.26.0+k3s",
				K8sDistribution: "k3s",
			},
		},
		{
			name: "with no k8s client with metrics endpoint",
			args: args{
				sdkStore:  mockStore,
				clientset: nil,
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetReplicatedID().Return("sdk-heartbeat-cluster-id")
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
				mockStore.EXPECT().GetAdditionalMetricsEndpoint().Times(2).Return(fmt.Sprintf("%s/metrics", mockServer.URL))
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-0-license",
					},
				})
			},
			want: &types.HeartbeatInfo{
				ClusterID:       "sdk-heartbeat-cluster-id",
				InstanceID:      "sdk-heartbeat-app",
				ChannelID:       "sdk-heartbeat-app-nightly",
				ChannelName:     "Nightly",
				ChannelSequence: 1,
				AppStatus:       string(appstatetypes.StateMissing),
				ResourceStates:  []appstatetypes.ResourceState{},
				AdditionalMetrics: types.AdditionalMetrics{
					"X-Replicated-Metric": "test",
				},
			},
		},
		{
			name: "with k8s client and metrics endpoint",
			args: args{
				sdkStore:  mockStore,
				clientset: mockClientset("v1.26.0+k3s"),
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetReplicatedID().Return("sdk-heartbeat-cluster-id")
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
				mockStore.EXPECT().GetAdditionalMetricsEndpoint().Times(2).Return(fmt.Sprintf("%s/metrics", mockServer.URL))
				mockStore.EXPECT().GetLicense().Return(&v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-heartbeat-customer-0-license",
					},
				})
			},
			want: &types.HeartbeatInfo{
				ClusterID:       "sdk-heartbeat-cluster-id",
				InstanceID:      "sdk-heartbeat-app",
				ChannelID:       "sdk-heartbeat-app-nightly",
				ChannelName:     "Nightly",
				ChannelSequence: 1,
				AppStatus:       string(appstatetypes.StateMissing),
				ResourceStates:  []appstatetypes.ResourceState{},
				K8sVersion:      "v1.26.0+k3s",
				K8sDistribution: "k3s",
				AdditionalMetrics: types.AdditionalMetrics{
					"X-Replicated-Metric": "test",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockStoreExpectations()
			if got := GetHeartbeatInfo(tt.args.sdkStore, tt.args.clientset); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHeartbeatInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mockClientset(gitVersion string, objects ...runtime.Object) kubernetes.Interface {
	clientset := fake.NewSimpleClientset(objects...)
	clientset.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
		GitVersion: gitVersion,
	}
	return clientset
}
