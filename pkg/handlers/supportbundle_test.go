package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestUploadSupportBundle_NotSupported(t *testing.T) {
	store.InitInMemory(store.InitInMemoryStoreOptions{
		License: licensewrapper.LicenseWrapper{V1: &v1beta1.License{
			Spec: v1beta1.LicenseSpec{
				LicenseID:                     "test-license-id",
				IsSupportBundleUploadSupported: false,
			},
		}},
	})
	defer store.SetStore(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/app/supportbundle", nil)

	UploadSupportBundle(w, r)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestUploadSupportBundle_Airgap(t *testing.T) {
	store.InitInMemory(store.InitInMemoryStoreOptions{
		License: licensewrapper.LicenseWrapper{V1: &v1beta1.License{
			Spec: v1beta1.LicenseSpec{
				LicenseID:                     "test-license-id",
				IsSupportBundleUploadSupported: true,
			},
		}},
	})
	defer store.SetStore(nil)

	t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "true")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/app/supportbundle", nil)

	UploadSupportBundle(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
