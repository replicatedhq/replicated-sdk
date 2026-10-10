package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSDKManagedLicenseInfoUsesActiveLicenseWithoutExposingToken(t *testing.T) {
	previousStore, previousManager := store.GetStore(), licensestate.Current()
	t.Cleanup(func() { store.SetStore(previousStore); licensestate.Configure(previousManager) })
	token, err := installationtoken.New("installation", 3)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	store.InitInMemory(store.InitInMemoryStoreOptions{
		License:               licensewrapper.LicenseWrapper{V2: &kotsv1beta2.License{Spec: kotsv1beta2.LicenseSpec{LicenseID: encoded, AppSlug: "app"}}},
		ReplicatedAppEndpoint: server.URL,
	})
	licensestate.Configure(licensestate.NewManager(fake.NewSimpleClientset(), "app", "sdk-state", "managed-pull"))
	recorder := httptest.NewRecorder()
	GetLicenseInfo(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/license/info", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Zero(t, requests, "license info must not run the legacy refresh alongside automatic rotation")
	var info LicenseInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	require.Empty(t, info.LicenseID)
	require.Equal(t, "installation", info.InstallationID)
	require.NotContains(t, recorder.Body.String(), encoded)
	require.NotContains(t, recorder.Body.String(), token.Secret)
}

func TestLegacyLicenseInfoKeepsLicenseID(t *testing.T) {
	info := licenseInfoFromWrapper(licensewrapper.LicenseWrapper{V2: &kotsv1beta2.License{Spec: kotsv1beta2.LicenseSpec{LicenseID: "legacy-license"}}})
	require.Equal(t, "legacy-license", info.LicenseID)
	require.Empty(t, info.InstallationID)
}
