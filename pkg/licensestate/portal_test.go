package licensestate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// This fixture represents already persisted state for transport tests. Tests
// exercising ApplyLicense must still supply a valid signed license.
func portalTestState(t *testing.T) (*State, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	token, err := installationtoken.New("installation-1", 2)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	artifact := []byte("apiVersion: kots.io/v1beta1\nkind: License\nmetadata:\n  name: test\nspec:\n  licenseID: " + encoded + "\n  replicatedProxyDomain: proxy.example.com\n")
	docker, err := json.Marshal(map[string]any{"auths": map[string]any{"proxy.example.com": map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte("LICENSE_ID:" + encoded))}}})
	require.NoError(t, err)
	return &State{Version: 1, Status: StatusCurrent, InstallationID: token.InstallationID, CredentialGeneration: 2,
		PublicKey: public, PrivateKey: private, ActiveLicense: artifact, ActiveGeneration: 2,
		ActiveLicenseFingerprint: fingerprint(artifact), ActiveImageCredentials: docker}, encoded
}

func TestBindResumesStagedInitialLicenseWithTheSameKey(t *testing.T) {
	artifact := signedInstallationLicense(t)(1)
	wrapper, err := sdklicense.LoadLicenseFromBytes(artifact)
	require.NoError(t, err)
	docker, err := json.Marshal(map[string]any{"auths": map[string]any{"proxy.example.com": map[string]string{
		"auth": base64.StdEncoding.EncodeToString([]byte("LICENSE_ID:" + wrapper.GetLicenseID())),
	}}})
	require.NoError(t, err)
	client := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap", Namespace: "sdk"}, Data: map[string][]byte{
			"binding-grant": []byte("test-grant"), ActiveLicenseDataKey: artifact,
		}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "sdk"}, Data: map[string][]byte{corev1.DockerConfigJsonKey: docker}},
	)
	manager := NewManager(client, "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	state, err := manager.PrepareBinding(t.Context(), "installation")
	require.NoError(t, err)
	state.Status = StatusStaging
	state.CandidateLicense = artifact
	state.CandidateGeneration = wrapper.GetLicenseSequence()
	state.CandidateCredentialGeneration = 1
	secret, err := client.CoreV1().Secrets("sdk").Get(t.Context(), "sdk-state", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = manager.writeState(t.Context(), secret, state)
	require.NoError(t, err)
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sdk/bind", r.URL.Path)
		var request map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, base64.StdEncoding.EncodeToString(state.PublicKey), request["publicKey"])
		require.NoError(t, json.NewEncoder(w).Encode(portalArtifactResponse{InstallationID: "installation", CredentialGeneration: 1,
			License: base64.StdEncoding.EncodeToString(artifact)}))
	}))
	defer portal.Close()
	previous := store.GetStore()
	store.SetStore(&store.InMemoryStore{})
	t.Cleanup(func() { store.SetStore(previous) })
	restarted := NewManager(client, "sdk", "sdk-state", "managed-pull")
	restarted.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	active, err := restarted.BindFromBootstrapSecret(t.Context(), portal.URL, "bootstrap")
	require.NoError(t, err)
	require.Equal(t, StatusCurrent, active.Status)
	require.Equal(t, state.PrivateKey, active.PrivateKey)
	require.Equal(t, artifact, active.ActiveLicense)
	require.Empty(t, active.CandidateLicense)
}

func verifyPortalTestRequest(t *testing.T, r *http.Request, state *State, encoded string) map[string]interface{} {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer "+encoded {
		t.Error("request did not use the active token")
	}
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	timestamp, err := strconv.ParseInt(r.Header.Get("X-Replicated-Installation-Timestamp"), 10, 64)
	require.NoError(t, err)
	proof := installationtoken.Proof{Timestamp: timestamp, Nonce: r.Header.Get("X-Replicated-Installation-Nonce"), Signature: r.Header.Get("X-Replicated-Installation-Signature")}
	require.NoError(t, proof.Verify(ed25519.PublicKey(state.PublicKey), encoded, r.Method, r.URL.EscapedPath(), body, time.Now()))
	var request map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &request))
	require.NotContains(t, request, "license")
	require.NotContains(t, request, "licenseId")
	require.NotContains(t, request, "signature")
	return request
}

func TestLicenseImageCredentialsUseInstallationTokenWithoutExchange(t *testing.T) {
	state, encoded := portalTestState(t)
	state.CandidateLicense = state.ActiveLicense
	// Hosts come from the installer, not the license.
	config, err := licenseDockerConfig(state, []string{"proxy.example.com"})
	require.NoError(t, err)
	var docker struct {
		Auths map[string]struct{ Auth string }
	}
	require.NoError(t, json.Unmarshal(config, &docker))
	auth, err := base64.StdEncoding.DecodeString(docker.Auths["proxy.example.com"].Auth)
	require.NoError(t, err)
	if string(auth) != "LICENSE_ID:"+encoded {
		t.Error("Docker config does not contain the license's token")
	}

	ctx := context.Background()
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "sdk", UID: "stable-uid"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("old")},
	})
	manager := NewManager(client, "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err = manager.stageImagePullSecretUpdate(ctx, config, state.ActiveLicense)
	require.NoError(t, err)
	secret, err := client.CoreV1().Secrets("sdk").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.EqualValues(t, "stable-uid", secret.UID)
	require.Equal(t, config, secret.Data[corev1.DockerConfigJsonKey])
	if string(secret.Data[InstallationTokenDataKey]) != encoded {
		t.Error("pull Secret token projection is wrong")
	}
	require.Equal(t, state.ActiveLicense, secret.Data[ActiveLicenseDataKey])
	require.NotContains(t, secret.Data, "state.json")
	require.NotContains(t, secret.Data, "privateKey")
}

func TestReportPortalRotationStatusSignsStatusAndUsesCurrentToken(t *testing.T) {
	ctx := context.Background()
	state, encoded := portalTestState(t)
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(ctx, nil, state)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sdk/rotation-status", r.URL.Path)
		request := verifyPortalTestRequest(t, r, state, encoded)
		require.Equal(t, "rotation-1", request["rotationId"])
		require.Equal(t, ServiceAccountRotationStatusStaging, request["status"])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	require.NoError(t, manager.reportPortalRotationStatus(ctx, server.URL, "rotation-1", ServiceAccountRotationStatusStaging))
}

func TestSynchronizePortalRotationMarksManualReplacementRequiredAfterOverlap(t *testing.T) {
	ctx := context.Background()
	state, encoded := portalTestState(t)
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(ctx, nil, state)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sdk/rotation-offer", r.URL.Path)
		verifyPortalTestRequest(t, r, state, encoded)
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"status":"manual_replacement_required"}`))
	}))
	defer server.Close()
	updated, applied, err := manager.SynchronizePortalRotation(ctx, server.URL)
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, StatusManualReplacementRequired, updated.Status)
	loaded, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, StatusManualReplacementRequired, loaded.Status)
	require.Equal(t, state.ActiveLicense, loaded.ActiveLicense)
	require.Equal(t, state.ActiveImageCredentials, loaded.ActiveImageCredentials)
}

func TestSuccessfulCheckInClearsTemporaryAuthenticationWarning(t *testing.T) {
	state, encoded := portalTestState(t)
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(t.Context(), nil, state)
	require.NoError(t, err)
	response := http.StatusUnauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifyPortalTestRequest(t, r, state, encoded)
		w.WriteHeader(response)
	}))
	defer server.Close()
	first, _, err := manager.SynchronizePortalRotation(t.Context(), server.URL)
	require.NoError(t, err)
	require.Equal(t, StatusManualReplacementRequired, first.Status)
	response = http.StatusNoContent
	current, applied, err := manager.SynchronizePortalRotation(t.Context(), server.URL)
	require.NoError(t, err)
	require.False(t, applied, "successful check-in must not apply a new license")
	require.Equal(t, StatusCurrent, current.Status)
	require.Empty(t, current.LastError)
	saved, err := manager.Load(t.Context())
	require.NoError(t, err)
	require.Equal(t, StatusCurrent, saved.Status)
	require.Empty(t, saved.LastError)
	require.Equal(t, state.ActiveLicense, saved.ActiveLicense)
	require.Equal(t, state.ActiveImageCredentials, saved.ActiveImageCredentials)
	require.Equal(t, state.PrivateKey, saved.PrivateKey)
}

func TestSynchronizeRetriesSavedSuccessorAcknowledgement(t *testing.T) {
	ctx := context.Background()
	state, encoded := portalTestState(t)
	state.Status, state.RotationID = StatusActivated, "rotation-1"
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(ctx, nil, state)
	require.NoError(t, err)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		require.Equal(t, "/sdk/rotation-acknowledgement", r.URL.Path)
		request := verifyPortalTestRequest(t, r, state, encoded)
		require.Equal(t, float64(2), request["credentialGeneration"])
		require.Equal(t, state.ActiveLicenseFingerprint, request["licenseFingerprint"])
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	_, _, err = manager.SynchronizePortalRotation(ctx, server.URL)
	require.Error(t, err)
	loaded, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, StatusActivated, loaded.Status)
	updated, changed, err := manager.SynchronizePortalRotation(ctx, server.URL)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, StatusConfirmed, updated.Status)
	require.Equal(t, 2, attempts)
	require.Equal(t, state.ActiveLicense, updated.ActiveLicense)
}

func TestInvalidReplacementLeavesCurrentLicenseAndCredentials(t *testing.T) {
	ctx := context.Background()
	state, _ := portalTestState(t)
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(ctx, nil, state)
	require.NoError(t, err)
	// A parseable but unsigned replacement cannot alter active state.
	_, err = manager.ApplyLicense(ctx, []byte(strings.ReplaceAll(string(state.ActiveLicense), "proxy.example.com", "different.example.com")), ApplyOptions{})
	require.Error(t, err)
	loaded, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, state.ActiveLicense, loaded.ActiveLicense)
	require.Equal(t, state.ActiveImageCredentials, loaded.ActiveImageCredentials)
}

func TestManuallyAppliedSuccessorConfirmsOnCheckIn(t *testing.T) {
	sign := signedInstallationLicense(t)
	previous := store.GetStore()
	t.Cleanup(func() { store.SetStore(previous) })
	store.SetStore(&store.InMemoryStore{})
	client := fake.NewSimpleClientset()
	m := NewManager(client, "app", "sdk-state", "managed-pull")
	m.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := m.PrepareBinding(t.Context(), "installation")
	require.NoError(t, err)
	_, err = m.ApplyLicense(t.Context(), sign(1), ApplyOptions{})
	require.NoError(t, err)
	successor := sign(2)
	state, err := m.ApplyLicense(t.Context(), successor, ApplyOptions{})
	require.NoError(t, err)
	require.Empty(t, state.RotationID, "manual upload does not supply Portal rotation metadata")
	wrapper, err := sdklicense.LoadLicenseFromBytes(successor)
	require.NoError(t, err)
	acknowledgements := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := verifyPortalTestRequest(t, r, state, wrapper.GetLicenseID())
		switch r.URL.Path {
		case "/sdk/rotation-offer":
			require.NoError(t, json.NewEncoder(w).Encode(portalArtifactResponse{
				InstallationID: "installation", RotationID: "rotation", CredentialGeneration: 2,
				License: base64.StdEncoding.EncodeToString(successor),
			}))
		case "/sdk/rotation-acknowledgement":
			acknowledgements++
			loaded, err := m.Load(t.Context())
			require.NoError(t, err)
			require.Equal(t, "rotation", loaded.RotationID, "save before sending confirmation")
			require.Equal(t, fingerprint(successor), request["licenseFingerprint"])
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client.ClearActions()
	updated, changed, err := m.SynchronizePortalRotation(t.Context(), server.URL)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, StatusConfirmed, updated.Status)
	require.Equal(t, 1, acknowledgements)
	require.Equal(t, successor, updated.ActiveLicense)
	for _, action := range client.Actions() {
		if action.GetVerb() == "update" {
			update := action.(interface{ GetObject() runtime.Object })
			require.Equal(t, "sdk-state", update.GetObject().(*corev1.Secret).Name, "confirmation must not rewrite image credentials")
		}
	}
	// A same-generation license with a different secret is not the saved
	// successor, even if its signature and customer are valid.
	_, err = m.attachManuallyAppliedRotation(t.Context(), updated, "another-rotation", sign(2))
	require.Error(t, err)
}

// "Last synced" has to mean the last time this installation asked, not the last
// time something changed. A check that finds nothing new is still a check, and
// it is what tells someone reading the card that they are looking at current
// information rather than hours-old information.
func TestSynchronizeRecordsTheCheckInEvenWhenNothingChanged(t *testing.T) {
	ctx := context.Background()
	state, _ := portalTestState(t)
	manager := NewManager(fake.NewSimpleClientset(), "sdk", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	_, err := manager.writeState(ctx, nil, state)
	require.NoError(t, err)

	before, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Nil(t, before.LastSynchronization)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sdk/rotation-offer", r.URL.Path)
		w.WriteHeader(http.StatusNoContent) // nothing to do
	}))
	defer server.Close()

	returned, applied, err := manager.SynchronizePortalRotation(ctx, server.URL)
	require.NoError(t, err)
	require.False(t, applied, "nothing was rotated")
	require.NotNil(t, returned.LastSynchronization, "the check-in must be reported to callers")

	saved, err := manager.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, saved.LastSynchronization, "and must survive a restart")
	require.Equal(t, state.ActiveGeneration, saved.ActiveGeneration, "recording a check-in must not touch the license")
}
