package pact

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pact-foundation/pact-go/dsl"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
)

const (
	supportBundlePactLicenseID = "support-bundle-pact-customer-0-license"
	supportBundlePactChannelID = "support-bundle-pact-app-nightly"
	supportBundlePactBundleID  = "pact-test-bundle-id"
)

func setupSupportBundleMockStore(t *testing.T) (*mock_store.MockStore, func()) {
	ctrl := gomock.NewController(t)

	mockStore := mock_store.NewMockStore(ctrl)
	mockStore.EXPECT().GetLicense().AnyTimes().Return(licensewrapper.LicenseWrapper{V1: &v1beta1.License{
		Spec: v1beta1.LicenseSpec{
			LicenseID: supportBundlePactLicenseID,
			Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
		},
	}})
	mockStore.EXPECT().GetReplicatedAppEndpoint().AnyTimes().Return("")
	mockStore.EXPECT().GetReplicatedID().AnyTimes().Return("support-bundle-pact-cluster-id")
	mockStore.EXPECT().GetAppID().AnyTimes().Return("support-bundle-pact-app")
	mockStore.EXPECT().GetChannelID().AnyTimes().Return(supportBundlePactChannelID)
	mockStore.EXPECT().GetChannelName().AnyTimes().Return("Nightly")
	mockStore.EXPECT().GetChannelSequence().AnyTimes().Return(int64(1))
	mockStore.EXPECT().GetAppStatus().AnyTimes().Return(appstatetypes.AppStatus{})
	mockStore.EXPECT().GetRunningImages().AnyTimes().Return(map[string][]string{})
	mockStore.EXPECT().GetNamespace().AnyTimes().Return("default")

	return mockStore, ctrl.Finish
}

func TestGetSupportBundleUploadURL(t *testing.T) {
	mockStore, cleanup := setupSupportBundleMockStore(t)
	defer cleanup()

	pact.
		AddInteraction().
		Given("Support bundle upload enabled for customer").
		UponReceiving("A request to get a support bundle upload URL").
		WithRequest(dsl.Request{
			Method: http.MethodPost,
			Headers: dsl.MapMatcher{
				"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
				"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", supportBundlePactLicenseID, supportBundlePactLicenseID))))),
			},
			Path: dsl.String("/v3/supportbundle/upload-url"),
		}).
		WillRespondWith(dsl.Response{
			Status: http.StatusOK,
			Headers: dsl.MapMatcher{
				"Content-Type": dsl.Term("application/json; charset=utf-8", `application\/json`),
			},
			Body: map[string]interface{}{
				"bundle_id":  dsl.Like("some-bundle-id"),
				"upload_url": dsl.Like("http://minio:9000/test-support-bundle/some-bundle-id/supportbundle.tar.gz"),
			},
		})

	if err := pact.Verify(func() error {
		resp, err := upstream.GetSupportBundleUploadURL(mockStore)
		if err != nil {
			return fmt.Errorf("GetSupportBundleUploadURL returned error: %v", err)
		}
		if resp.BundleID == "" {
			return fmt.Errorf("expected non-empty bundle_id")
		}
		if resp.UploadURL == "" {
			return fmt.Errorf("expected non-empty upload_url")
		}
		return nil
	}); err != nil {
		t.Fatalf("Error on Verify: %v", err)
	}
}

func TestMarkSupportBundleUploaded(t *testing.T) {
	mockStore, cleanup := setupSupportBundleMockStore(t)
	defer cleanup()

	pact.
		AddInteraction().
		Given("Support bundle upload enabled for customer").
		UponReceiving("A request to mark a support bundle as uploaded").
		WithRequest(dsl.Request{
			Method: http.MethodPost,
			Headers: dsl.MapMatcher{
				"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
				"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", supportBundlePactLicenseID, supportBundlePactLicenseID))))),
				"Content-Type":  dsl.String("application/json"),
			},
			Path: dsl.String(fmt.Sprintf("/v3/supportbundle/%s/uploaded", supportBundlePactBundleID)),
			Body: map[string]interface{}{
				"channel_id": supportBundlePactChannelID,
			},
		}).
		WillRespondWith(dsl.Response{
			Status: http.StatusOK,
			Headers: dsl.MapMatcher{
				"Content-Type": dsl.Term("application/json; charset=utf-8", `application\/json`),
			},
			Body: map[string]interface{}{
				"slug": dsl.Like("2019-01-01@02:34"),
			},
		})

	if err := pact.Verify(func() error {
		slug, err := upstream.MarkSupportBundleUploaded(mockStore, supportBundlePactBundleID)
		if err != nil {
			return fmt.Errorf("MarkSupportBundleUploaded returned error: %v", err)
		}
		if slug == "" {
			return fmt.Errorf("expected non-empty slug")
		}
		return nil
	}); err != nil {
		t.Fatalf("Error on Verify: %v", err)
	}
}
