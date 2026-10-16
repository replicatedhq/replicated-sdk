package handlers

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLostSDKStateReportsManualRecoveryAndDoesNotAcceptReplacement(t *testing.T) {
	t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "false")
	client := fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: "app"},
		Data: map[string][]byte{licensestate.ActiveLicenseDataKey: []byte("existing active license")}})
	previous := licensestate.Current()
	licensestate.Configure(licensestate.NewManager(client, "app", "sdk-state", "pull-secret"))
	t.Cleanup(func() { licensestate.Configure(previous) })
	recorder := httptest.NewRecorder()
	GetLicenseStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/license/status", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "ManualRecoveryRequired")
	require.NotContains(t, recorder.Body.String(), "existing active license")
	recorder = httptest.NewRecorder()
	body := `{"license":"` + base64.StdEncoding.EncodeToString([]byte("replacement")) + `"}`
	ApplyLicense(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/license/apply", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "manual recovery is required")
}
func TestGetLicenseStatusRedactsSecrets(t *testing.T) {
	manager := licensestate.NewManager(fake.NewSimpleClientset(), "app", "sdk-state", "pull-secret")
	licensestate.Configure(manager)

	recorder := httptest.NewRecorder()
	GetLicenseStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/license/status", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(licensestate.StatusUnbound))
	require.NotContains(t, recorder.Body.String(), "privateKey")
	require.NotContains(t, recorder.Body.String(), "activeLicense")
	require.NotContains(t, recorder.Body.String(), "activeImageCredentials")
}

func TestApplyLicenseRejectsInitialBindingAndMalformedArtifacts(t *testing.T) {
	manager := licensestate.NewManager(fake.NewSimpleClientset(), "app", "sdk-state", "pull-secret")
	licensestate.Configure(manager)

	t.Run("unbound", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		body := []byte(`{"license":"` + base64.StdEncoding.EncodeToString([]byte("not used")) + `"}`)
		ApplyLicense(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/license/apply", bytes.NewReader(body)))
		require.Equal(t, http.StatusConflict, recorder.Code)
		require.Contains(t, recorder.Body.String(), "binding grant")
	})

	t.Run("malformed artifact", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ApplyLicense(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/license/apply", bytes.NewBufferString(`{"license":"%%%"}`)))
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}
