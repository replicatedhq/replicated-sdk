package pact

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/pact-foundation/pact-go/dsl"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
)

func TestGetSupportBundleUploadURL(t *testing.T) {
	license := &v1beta1.License{
		Spec: v1beta1.LicenseSpec{
			LicenseID: "replicated-sdk-supportbundle-customer-0-license",
			AppSlug:   "replicated-sdk-supportbundle-app",
			Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
			ChannelID: "replicated-sdk-supportbundle-app-nightly",
		},
	}

	channelSequence := int64(1)

	pactInteraction := func() {
		pact.
			AddInteraction().
			Given("License exists and support bundle upload is enabled").
			UponReceiving("A request to get a support bundle upload URL").
			WithRequest(dsl.Request{
				Method: http.MethodPost,
				Headers: dsl.MapMatcher{
					"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
					"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID))))),
				},
				Path: dsl.String("/v3/supportbundle/upload-url"),
			}).
			WillRespondWith(dsl.Response{
				Status: http.StatusOK,
				Headers: dsl.MapMatcher{
					"Content-Type": dsl.String("application/json"),
				},
				Body: dsl.MapMatcher{
					"bundle_id":  dsl.Like("bundle-id-123"),
					"upload_url": dsl.Like("https://s3.amazonaws.com/bucket/key?presigned=true"),
				},
			})
	}

	t.Run("Get support bundle upload URL", func(t *testing.T) {
		pactInteraction()

		storeOptions := store.InitInMemoryStoreOptions{
			License:               licensewrapper.LicenseWrapper{V1: license},
			LicenseFields:         nil,
			ReplicatedAppEndpoint: license.Spec.Endpoint,
			ChannelID:             license.Spec.ChannelID,
			ChannelSequence:       channelSequence,
			Namespace:             "default",
		}
		store.InitInMemory(storeOptions)
		defer store.SetStore(nil)

		if err := pact.Verify(func() error {
			result, err := upstream.GetSupportBundleUploadURL(store.GetStore())
			if err != nil {
				return fmt.Errorf("GetSupportBundleUploadURL() error: %v", err)
			}
			if result.BundleID == "" {
				return fmt.Errorf("expected non-empty bundle_id")
			}
			if result.UploadURL == "" {
				return fmt.Errorf("expected non-empty upload_url")
			}
			return nil
		}); err != nil {
			t.Fatalf("Error on Verify: %v", err)
		}
	})
}

func TestMarkSupportBundleUploaded(t *testing.T) {
	license := &v1beta1.License{
		Spec: v1beta1.LicenseSpec{
			LicenseID: "replicated-sdk-supportbundle-customer-0-license",
			AppSlug:   "replicated-sdk-supportbundle-app",
			Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
			ChannelID: "replicated-sdk-supportbundle-app-nightly",
		},
	}

	channelSequence := int64(1)
	bundleID := "bundle-id-123"

	pactInteraction := func() {
		pact.
			AddInteraction().
			Given("License exists and support bundle has been uploaded").
			UponReceiving("A request to mark a support bundle as uploaded").
			WithRequest(dsl.Request{
				Method: http.MethodPost,
				Headers: dsl.MapMatcher{
					"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
					"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID))))),
					"Content-Type":  dsl.String("application/json"),
				},
				Path: dsl.String(fmt.Sprintf("/v3/supportbundle/%s/uploaded", bundleID)),
				Body: map[string]interface{}{
					"channel_id": license.Spec.ChannelID,
				},
			}).
			WillRespondWith(dsl.Response{
				Status: http.StatusOK,
				Headers: dsl.MapMatcher{
					"Content-Type": dsl.String("application/json"),
				},
				Body: dsl.MapMatcher{
					"slug": dsl.Like("2026-03-03@14:30:05-myapp"),
				},
			})
	}

	t.Run("Mark support bundle as uploaded", func(t *testing.T) {
		pactInteraction()

		storeOptions := store.InitInMemoryStoreOptions{
			License:               licensewrapper.LicenseWrapper{V1: license},
			LicenseFields:         nil,
			ReplicatedAppEndpoint: license.Spec.Endpoint,
			ChannelID:             license.Spec.ChannelID,
			ChannelSequence:       channelSequence,
			Namespace:             "default",
		}
		store.InitInMemory(storeOptions)
		defer store.SetStore(nil)

		if err := pact.Verify(func() error {
			slug, err := upstream.MarkSupportBundleUploaded(store.GetStore(), bundleID)
			if err != nil {
				return fmt.Errorf("MarkSupportBundleUploaded() error: %v", err)
			}
			if slug == "" {
				return fmt.Errorf("expected non-empty slug")
			}
			return nil
		}); err != nil {
			t.Fatalf("Error on Verify: %v", err)
		}
	})
}
