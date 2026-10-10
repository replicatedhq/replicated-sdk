package licensestate

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"testing"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	licensecrypto "github.com/replicatedhq/kotskinds/pkg/crypto"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Generate a fresh test root; never load a real license or signing key.
// Tests using this helper are serial because the verifier's trust map is global.
func signedInstallationLicense(t *testing.T) func(int64) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	public := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	keyID := "sdk-test-" + t.Name()
	licensecrypto.PublicKeysRSA[keyID] = public
	t.Cleanup(func() { delete(licensecrypto.PublicKeysRSA, keyID) })
	sign := func(data []byte) []byte {
		digest := sha256.Sum256(data)
		signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], nil)
		require.NoError(t, err)
		return signature
	}
	marshal := func(value any) []byte {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		return data
	}
	return func(generation int64) []byte {
		token, err := installationtoken.New("installation", generation)
		require.NoError(t, err)
		encoded, err := token.Encode()
		require.NoError(t, err)
		lic := kotsv1beta2.License{
			TypeMeta: metav1.TypeMeta{APIVersion: "kots.io/v1beta2", Kind: "License"},
			Spec: kotsv1beta2.LicenseSpec{
				LicenseID: encoded, CustomerID: "customer", AppSlug: "app",
				LicenseSequence: 10, ReplicatedProxyDomain: "proxy.example.com",
			},
		}
		data := marshal(lic)
		keySignature := marshal(licensecrypto.KeySignature{GlobalKeyID: keyID, Signature: sign(public)})
		inner := marshal(licensecrypto.InnerSignature{
			PublicKey: string(public), V2KeySignature: keySignature, V2LicenseSignature: sign(data),
		})
		lic.Spec.Signature = marshal(licensecrypto.OuterSignature{LicenseData: data, InnerSignature: inner})
		return marshal(lic)
	}
}

func TestApplyLicenseActivatesMemoryOnlyAfterDurableState(t *testing.T) {
	sign := signedInstallationLicense(t)
	previousStore := store.GetStore()
	t.Cleanup(func() { store.SetStore(previousStore) })
	store.SetStore(&store.InMemoryStore{})
	client := fake.NewSimpleClientset()
	manager := testManager(client, "app")
	_, err := manager.PrepareBinding(t.Context(), "installation")
	require.NoError(t, err)
	first, next := sign(1), sign(2)
	options := ApplyOptions{}
	_, err = manager.ApplyLicense(t.Context(), first, options)
	require.NoError(t, err)
	firstWrapper, err := sdklicense.LoadLicenseFromBytes(first)
	require.NoError(t, err)
	nextWrapper, err := sdklicense.LoadLicenseFromBytes(next)
	require.NoError(t, err)
	checkedCommit := false
	client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret)
		if secret.Name == "sdk-state" {
			var state State
			require.NoError(t, json.Unmarshal(secret.Data[stateSecretDataKey], &state))
			if state.CredentialGeneration == 2 {
				checkedCommit = true
				require.True(t, store.GetStore().GetLicense().GetLicenseID() == firstWrapper.GetLicenseID(), "memory changed before commit")
			}
		}
		return false, nil, nil
	})
	active, err := manager.ApplyLicense(t.Context(), next, options)
	require.NoError(t, err)
	require.True(t, checkedCommit)
	require.EqualValues(t, 2, active.CredentialGeneration)
	require.True(t, store.GetStore().GetLicense().GetLicenseID() == nextWrapper.GetLicenseID())
	restarted := testManager(client, "app")
	saved, err := restarted.Load(t.Context())
	require.NoError(t, err)
	require.True(t, string(saved.ActiveLicense) == string(next))
	pull, err := client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, string(pull.Data[InstallationTokenDataKey]) == nextWrapper.GetLicenseID())
}

func TestApplyLicenseFailureKeepsPreviousLicense(t *testing.T) {
	for _, failAt := range []string{"pull-secret", "active-state"} {
		t.Run(failAt, func(t *testing.T) {
			sign := signedInstallationLicense(t)
			previousStore := store.GetStore()
			t.Cleanup(func() { store.SetStore(previousStore) })
			store.SetStore(&store.InMemoryStore{})
			client := fake.NewSimpleClientset()
			manager := testManager(client, "app")
			_, err := manager.PrepareBinding(t.Context(), "installation")
			require.NoError(t, err)
			first, next := sign(1), sign(2)
			options := ApplyOptions{}
			_, err = manager.ApplyLicense(t.Context(), first, options)
			require.NoError(t, err)
			before, err := client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
			require.NoError(t, err)
			client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
				secret := action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret)
				fail := failAt == "pull-secret" && secret.Name == "managed-pull"
				if failAt == "active-state" && secret.Name == "sdk-state" {
					var state State
					require.NoError(t, json.Unmarshal(secret.Data[stateSecretDataKey], &state))
					fail = state.CredentialGeneration == 2
				}
				if fail {
					return true, nil, errors.New("simulated write failure with sensitive details")
				}
				return false, nil, nil
			})
			_, err = manager.ApplyLicense(t.Context(), next, options)
			require.Error(t, err)
			saved, err := manager.Load(t.Context())
			require.NoError(t, err)
			require.EqualValues(t, 1, saved.CredentialGeneration)
			require.True(t, string(saved.ActiveLicense) == string(first))
			require.Equal(t, StatusReplacementFailed, saved.Status)
			require.NotContains(t, saved.LastError, "sensitive")
			after, err := client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
			require.NoError(t, err)
			require.True(t, string(before.Data[InstallationTokenDataKey]) == string(after.Data[InstallationTokenDataKey]))
			require.True(t, store.GetStore().GetLicense().GetLicenseID() == string(before.Data[InstallationTokenDataKey]))
		})
	}
}
func TestApplyLicenseRecoversWhenCommitResponseIsLost(t *testing.T) {
	sign := signedInstallationLicense(t)
	previousStore := store.GetStore()
	t.Cleanup(func() { store.SetStore(previousStore) })
	store.SetStore(&store.InMemoryStore{})
	client := fake.NewSimpleClientset()
	manager := testManager(client, "app")
	_, err := manager.PrepareBinding(t.Context(), "installation")
	require.NoError(t, err)
	first, next := sign(1), sign(2)
	options := ApplyOptions{}
	_, err = manager.ApplyLicense(t.Context(), first, options)
	require.NoError(t, err)
	committed := false
	client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret)
		if secret.Name == "sdk-state" {
			var state State
			require.NoError(t, json.Unmarshal(secret.Data[stateSecretDataKey], &state))
			if state.CredentialGeneration == 2 {
				require.NoError(t, client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), secret, "app"))
				committed = true
				return true, nil, errors.New("connection lost after commit")
			}
		}
		return false, nil, nil
	})
	active, err := manager.ApplyLicense(t.Context(), next, options)
	require.NoError(t, err)
	require.True(t, committed)
	require.EqualValues(t, 2, active.CredentialGeneration)
	pull, err := client.CoreV1().Secrets("app").Get(t.Context(), "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, string(pull.Data[ActiveLicenseDataKey]) == string(next), "a committed activation must not roll back the pull Secret")
	require.True(t, store.GetStore().GetLicense().GetLicenseID() == string(pull.Data[InstallationTokenDataKey]))
}

// Helm no longer renders the image pull Secret, so the SDK is its only owner.
// Re-applying the active license is the moment it revisits that Secret; if it
// did not, a deleted Secret would leave the cluster unable to pull images while
// durable state still reported the installation healthy.
func TestApplyLicenseRestoresADeletedImagePullSecret(t *testing.T) {
	ctx := context.Background()
	artifact := signedInstallationLicense(t)(1)
	wrapper, err := sdklicense.LoadLicenseFromBytes(artifact)
	require.NoError(t, err)
	credentials, err := licenseDockerConfig(&State{CandidateLicense: artifact}, []string{"proxy.replicated.com"})
	require.NoError(t, err)

	client := fake.NewSimpleClientset()
	manager := NewManager(client, "app", "sdk-state", "managed-pull")
	manager.SetRegistryDomains([]string{"proxy.replicated.com"})
	_, err = manager.writeState(ctx, nil, &State{
		Version: 1, Status: StatusCurrent, InstallationID: "installation-1",
		ActiveLicense: artifact, ActiveGeneration: wrapper.GetLicenseSequence(),
		ActiveLicenseFingerprint: fingerprint(artifact), ActiveImageCredentials: credentials,
	})
	require.NoError(t, err)
	_, err = manager.stageImagePullSecretUpdate(ctx, credentials, artifact)
	require.NoError(t, err)

	before, err := client.CoreV1().Secrets("app").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.NoError(t, client.CoreV1().Secrets("app").Delete(ctx, "managed-pull", metav1.DeleteOptions{}))

	// Re-applying the same artifact is a no-op for the active generation.
	reapplied, err := manager.ApplyLicense(ctx, artifact, ApplyOptions{})
	require.NoError(t, err)
	require.Equal(t, wrapper.GetLicenseSequence(), reapplied.ActiveGeneration)

	restored, err := client.CoreV1().Secrets("app").Get(ctx, "managed-pull", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, before.Data, restored.Data)
	require.Equal(t, corev1.SecretTypeDockerConfigJson, restored.Type)
}
