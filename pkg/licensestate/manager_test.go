package licensestate

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// testManager builds a manager for the given namespace with the registry
// domains EC supplies in practice. Tests that care about a specific set of
// domains configure them explicitly instead.
func testManager(client kubernetes.Interface, namespace string) *Manager {
	manager := NewManager(client, namespace, "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
	return manager
}

func TestWriteStateAndLoadRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	manager := testManager(client, "app")
	now := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	want := &State{
		Version:                  1,
		InstallationID:           "installation-1",
		ApplicationID:            "app-1",
		CustomerID:               "customer-1",
		PrivateKey:               []byte("private"),
		PublicKey:                []byte("public"),
		ActiveLicense:            []byte("signed-license"),
		ActiveGeneration:         4,
		ActiveLicenseFingerprint: "fingerprint",
		ActiveImageCredentials:   []byte("docker-config"),
		CredentialGeneration:     9,
		LastSynchronization:      &now,
		Status:                   StatusActivated,
	}
	secret, err := manager.writeState(ctx, nil, want)
	require.NoError(t, err)
	require.Equal(t, "sdk-state", secret.Name)

	got, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestPrepareBindingPersistsOneInstallationIdentity(t *testing.T) {
	ctx := context.Background()
	manager := testManager(fake.NewSimpleClientset(), "app")
	first, err := manager.PrepareBinding(ctx, "portal-installation")
	require.NoError(t, err)
	require.Equal(t, StatusUnbound, first.Status)
	require.Equal(t, "portal-installation", first.InstallationID)
	require.Len(t, first.PrivateKey, ed25519.PrivateKeySize)
	require.Len(t, first.PublicKey, ed25519.PublicKeySize)

	second, err := manager.PrepareBinding(ctx, "portal-installation")
	require.NoError(t, err)
	require.Equal(t, first.InstallationID, second.InstallationID)
	require.Equal(t, first.PublicKey, second.PublicKey)

	loaded, err := manager.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, first.InstallationID, loaded.InstallationID)
	_, err = manager.PrepareBinding(ctx, "another-installation")
	require.Error(t, err)
}

func TestMissingBootstrapDoesNotCreateReplacementKey(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := testManager(client, "app")
	_, err := manager.BindFromBootstrapSecret(t.Context(), "https://portal.example", "bootstrap")
	require.Error(t, err)
	_, err = client.CoreV1().Secrets("app").Get(t.Context(), "sdk-state", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "missing setup material must not create a new identity")
}

func TestLostOnlineIdentityRequiresManualRecoveryWithoutRecreatingState(t *testing.T) {
	for _, missingState := range []bool{false, true} {
		t.Run(fmt.Sprintf("whole-state-missing-%t", missingState), func(t *testing.T) {
			state, _ := portalTestState(t)
			client := fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app"},
				Data: map[string][]byte{ActiveLicenseDataKey: state.ActiveLicense}})
			manager := testManager(client, "app")
			if !missingState {
				state.PrivateKey = nil
				_, err := manager.writeState(t.Context(), nil, state)
				require.NoError(t, err)
			}
			loaded, err := manager.LoadOnlineState(t.Context())
			require.NoError(t, err)
			require.Equal(t, StatusManualRecoveryRequired, loaded.Status)
			require.Contains(t, loaded.LastError, "manual recovery is required")
			require.Empty(t, loaded.PrivateKey)
			// An invalid endpoint proves synchronization stops before any request.
			loaded, changed, err := manager.SynchronizePortalRotation(t.Context(), ":invalid")
			require.NoError(t, err)
			require.False(t, changed)
			require.Equal(t, StatusManualRecoveryRequired, loaded.Status)
			_, err = manager.BindFromBootstrapSecret(t.Context(), ":invalid", "bootstrap")
			require.ErrorContains(t, err, "manual recovery is required")
			saved, err := manager.Load(t.Context())
			require.NoError(t, err)
			require.Empty(t, saved.PrivateKey)
			if missingState {
				require.Empty(t, saved.InstallationID, "shared credentials must not recreate private state")
				_, err := client.CoreV1().Secrets("app").Get(t.Context(), "sdk-state", metav1.GetOptions{})
				require.True(t, apierrors.IsNotFound(err))
			} else {
				require.Equal(t, state.ActiveLicenseFingerprint, saved.ActiveLicenseFingerprint)
			}
		})
	}
}

func TestBootstrapTokenDoesNotLookLikeLostSDKState(t *testing.T) {
	_, token := portalTestState(t)
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app"},
		Data:       map[string][]byte{InstallationTokenDataKey: []byte(token)},
	})
	manager := testManager(client, "app")
	state, err := manager.LoadOnlineState(t.Context())
	require.NoError(t, err)
	require.Equal(t, StatusUnbound, state.Status)
	require.Empty(t, state.PrivateKey)
	_, err = client.CoreV1().Secrets("app").Get(t.Context(), "sdk-state", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestLoadMissingStateIsUnboundWithoutIdentity(t *testing.T) {
	manager := testManager(fake.NewSimpleClientset(), "app")
	state, err := manager.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, StatusUnbound, state.Status)
	require.Empty(t, state.InstallationID)
	require.Empty(t, state.PrivateKey)
}

func TestCallerCannotOverrideSignedLicenseIdentity(t *testing.T) {
	options := ApplyOptions{ApplicationID: "app", CustomerID: "customer"}
	_, err := options.withSignedIdentity("different-app", "customer")
	require.Error(t, err)
	_, err = options.withSignedIdentity("app", "different-customer")
	require.Error(t, err)
	_, err = options.withSignedIdentity("app", "")
	require.Error(t, err)
	validated, err := options.withSignedIdentity("app", "customer")
	require.NoError(t, err)
	require.Equal(t, options, validated)
}

func TestUpdateImagePullSecretUpdatesExistingSecretInPlace(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("old")},
	})
	manager := testManager(client, "app")
	_, err := manager.stageImagePullSecretUpdate(ctx, []byte("new"), nil)
	require.NoError(t, err)
	secret, err := client.CoreV1().Secrets("app").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte("new"), secret.Data[corev1.DockerConfigJsonKey])
}

func TestUpdateImagePullSecretUsesOnlySDKNamespace(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	manager := testManager(client, "sdk")
	_, err := manager.stageImagePullSecretUpdate(ctx, []byte("new"), nil)
	require.NoError(t, err)
	for _, action := range client.Actions() {
		require.Equal(t, "sdk", action.GetNamespace(), "SDK must not access other namespaces")
		require.Equal(t, "secrets", action.GetResource().Resource)
	}
	secret, err := client.CoreV1().Secrets("sdk").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte("new"), secret.Data[corev1.DockerConfigJsonKey])
}

func TestStageImagePullSecretUpdateFailurePreservesExistingCredentials(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "sdk"}, Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte("old-sdk")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app-a"}, Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte("old-app-a")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app-b"}, Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte("old-app-b")}},
	)
	client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		update := action.(k8stesting.UpdateAction)
		secret := update.GetObject().(*corev1.Secret)
		if update.GetNamespace() == "sdk" && string(secret.Data[corev1.DockerConfigJsonKey]) == "successor" {
			return true, nil, errors.New("simulated Secret update failure")
		}
		return false, nil, nil
	})
	manager := testManager(client, "sdk")

	_, err := manager.stageImagePullSecretUpdate(ctx, []byte("successor"), nil)
	require.Error(t, err)
	for namespace, want := range map[string]string{"sdk": "old-sdk", "app-a": "old-app-a", "app-b": "old-app-b"} {
		secret, getErr := client.CoreV1().Secrets(namespace).Get(ctx, "managed-pull", metav1.GetOptions{})
		require.NoErrorf(t, getErr, "get pull Secret in %s", namespace)
		require.Equalf(t, want, string(secret.Data[corev1.DockerConfigJsonKey]), "pull Secret in %s was not restored", namespace)
	}
}

func TestRestoreImagePullSecretDeletesSecretCreatedByFailedActivation(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	manager := testManager(client, "app")

	snapshot, err := manager.stageImagePullSecretUpdate(ctx, []byte("successor"), nil)
	require.NoError(t, err)
	require.NoError(t, manager.restoreImagePullSecret(ctx, snapshot))
	_, err = client.CoreV1().Secrets("app").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestPullSecretRollbackDoesNotOverwriteAnotherWriter(t *testing.T) {
	for _, existed := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-%t", existed), func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if existed {
				_, err := client.CoreV1().Secrets("app").Create(t.Context(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "managed-pull", Namespace: "app"},
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("old")},
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			manager := testManager(client, "app")
			snapshot, err := manager.stageImagePullSecretUpdate(t.Context(), []byte("candidate"), nil)
			require.NoError(t, err)
			current, err := client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
			require.NoError(t, err)
			current.Data[corev1.DockerConfigJsonKey] = []byte("another-writer")
			_, err = client.CoreV1().Secrets("app").Update(t.Context(), current, metav1.UpdateOptions{})
			require.NoError(t, err)
			require.Error(t, manager.restoreImagePullSecret(t.Context(), snapshot))
			current, err = client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, []byte("another-writer"), current.Data[corev1.DockerConfigJsonKey])
		})
	}
}

func TestValidateCandidateProtectsImmutableIdentityAndGeneration(t *testing.T) {
	state := &State{InstallationID: "install", ApplicationID: "app", CustomerID: "customer", ActiveGeneration: 7, ActiveLicenseFingerprint: "active"}
	require.Error(t, validateCandidate(state, 6, ApplyOptions{ApplicationID: "app", CustomerID: "customer", TargetInstallationID: "install"}))
	require.Error(t, validateCandidate(state, 8, ApplyOptions{ApplicationID: "other", CustomerID: "customer", TargetInstallationID: "install"}))
	require.Error(t, validateCandidate(state, 8, ApplyOptions{ApplicationID: "app", CustomerID: "other", TargetInstallationID: "install"}))
	require.Error(t, validateCandidate(state, 8, ApplyOptions{ApplicationID: "app", CustomerID: "customer", TargetInstallationID: "other"}))
	require.NoError(t, validateCandidate(state, 8, ApplyOptions{ApplicationID: "app", CustomerID: "customer", TargetInstallationID: "install"}))
}

func TestEstablishIdentityAnchorsAfterPrepareBinding(t *testing.T) {
	// PrepareBinding writes the key before Portal returns the first signed
	// artifact. The bind transaction must still persist its immutable anchors.
	state := &State{InstallationID: "installation-1"}
	options := ApplyOptions{ApplicationID: "app-1", CustomerID: "customer-1", TargetInstallationID: "installation-1"}
	require.NoError(t, validateCandidate(state, 1, options))
	establishIdentityAnchors(state, options)
	require.Equal(t, "app-1", state.ApplicationID)
	require.Equal(t, "customer-1", state.CustomerID)

	// A later replacement cannot overwrite the established identity.
	establishIdentityAnchors(state, ApplyOptions{ApplicationID: "other-app", CustomerID: "other-customer"})
	require.Equal(t, "app-1", state.ApplicationID)
	require.Equal(t, "customer-1", state.CustomerID)
}
