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
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

func TestGetUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStore := mock_store.NewMockStore(ctrl)

	tests := []struct {
		name                  string
		license               *v1beta1.License
		currentCursor         upstreamtypes.ReplicatedCursor
		mockStoreExpectations func()
		pactInteraction       func()
		wantErr               bool
	}{
		{
			name: "get updates for non-semver channel",
			license: &v1beta1.License{
				Spec: v1beta1.LicenseSpec{
					LicenseID:       "sdk-upstream-customer-nightly-license",
					LicenseSequence: 1,
					AppSlug:         "sdk-upstream-app",
					ChannelID:       "sdk-upstream-app-nightly",
					Endpoint:        fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
				},
			},
			currentCursor: upstreamtypes.ReplicatedCursor{
				ChannelID:       "sdk-upstream-app-nightly",
				ChannelName:     "Nightly",
				ChannelSequence: 1,
			},
			mockStoreExpectations: func() {
				mockStore.EXPECT().GetReplicatedAppEndpoint().Return(fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port))
				mockStore.EXPECT().GetReplicatedSDKID().Return("sdk-upstream-cluster-id")
				mockStore.EXPECT().GetAppID().Return("sdk-upstream-app")
				mockStore.EXPECT().GetChannelID().Return("sdk-upstream-app-nightly")
				mockStore.EXPECT().GetChannelName().Return("Nightly")
				mockStore.EXPECT().GetChannelSequence().Return(int64(1))
				mockStore.EXPECT().GetAppStatus().Times(2).Return(appstatetypes.AppStatus{
					AppSlug:        "sdk-upstream-app",
					Sequence:       1,
					State:          appstatetypes.StateMissing,
					ResourceStates: []appstatetypes.ResourceState{},
				})
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("Channel is not semver-enabled").
					UponReceiving("A request to get pending updates").
					WithRequest(dsl.Request{
						Method: http.MethodPost,
						Headers: dsl.MapMatcher{
							"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-upstream-customer-nightly-license", "sdk-upstream-customer-nightly-license"))))),
							"Content-Type":                           dsl.String("application/json"),
							"X-Replicated-K8sVersion":                dsl.Like("v1.25.3"),
							"X-Replicated-AppStatus":                 dsl.String("missing"),
							"X-Replicated-ClusterID":                 dsl.String("sdk-upstream-cluster-id"),
							"X-Replicated-InstanceID":                dsl.String("sdk-upstream-app"),
							"X-Replicated-DownstreamChannelID":       dsl.String("sdk-upstream-app-nightly"),
							"X-Replicated-DownstreamChannelSequence": dsl.String("1"),
						},
						Path: dsl.String("/release/sdk-upstream-app/pending"),
						Body: map[string]interface{}{
							"resource_states": dsl.String("[]"),
						},
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusOK,
						// Body: // TODO: validate there are channel releases with the necessary props
					})
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockStoreExpectations()
			tt.pactInteraction()
			if err := pact.Verify(func() error {
				_, err := upstream.GetUpdates(mockStore, tt.license, tt.currentCursor)
				if (err != nil) != tt.wantErr {
					t.Errorf("GetUpdates() error = %v, wantErr %v", err, tt.wantErr)
				}
				return nil
			}); err != nil {
				t.Fatalf("Error on Verify: %v", err)
			}
		})
	}
}
