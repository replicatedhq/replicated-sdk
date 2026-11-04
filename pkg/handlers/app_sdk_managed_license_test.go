package handlers

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	licensecrypto "github.com/replicatedhq/kotskinds/pkg/crypto"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Exercise the existing update-check handler, not a second refresh helper.
// A channel/license-sequence change keeps the same installation credential.
func TestAppUpdatesPersistsRefreshedSDKManagedLicense(t *testing.T) {
	t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "false")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	public := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	keyID := "update-check-test-root"
	licensecrypto.PublicKeysRSA[keyID] = public
	t.Cleanup(func() { delete(licensecrypto.PublicKeysRSA, keyID) })
	marshal := func(v any) []byte {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return b
	}
	sign := func(b []byte) []byte {
		digest := sha256.Sum256(b)
		signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], nil)
		require.NoError(t, err)
		return signature
	}
	token, err := installationtoken.New("installation", 1)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	artifact := func(sequence int64, channel string) []byte {
		lic := kotsv1beta2.License{
			TypeMeta: metav1.TypeMeta{APIVersion: "kots.io/v1beta2", Kind: "License"},
			Spec: kotsv1beta2.LicenseSpec{LicenseID: encoded, CustomerID: "customer", AppSlug: "app",
				LicenseSequence: sequence, ChannelID: channel, ReplicatedProxyDomain: "proxy.example.com"},
		}
		data := marshal(lic)
		inner := marshal(licensecrypto.InnerSignature{PublicKey: string(public),
			V2KeySignature:     marshal(licensecrypto.KeySignature{GlobalKeyID: keyID, Signature: sign(public)}),
			V2LicenseSignature: sign(data)})
		lic.Spec.Signature = marshal(licensecrypto.OuterSignature{LicenseData: data, InnerSignature: inner})
		return marshal(lic)
	}
	first, next := artifact(1, "stable"), artifact(2, "beta")
	previousStore, previousManager := store.GetStore(), licensestate.Current()
	previousFlags := k8sutil.KubernetesConfigFlags
	t.Cleanup(func() {
		store.SetStore(previousStore)
		licensestate.Configure(previousManager)
		k8sutil.KubernetesConfigFlags = previousFlags
	})
	for _, failWrite := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistence-fails-%t", failWrite), func(t *testing.T) {
			updateCalls := 0
			portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/license/app":
					_, _ = w.Write(next)
				case "/release/app/pending":
					updateCalls++
					require.Equal(t, "2", r.URL.Query().Get("licenseSequence"))
					_, _ = w.Write([]byte(`{"channelReleases":[]}`))
				case "/version", "/api", "/apis", "/api/v1/nodes", "/api/v1/namespaces/app/secrets/replicated-meta-data":
					// Reporting gathers optional cluster metadata alongside the
					// license request. This test does not supply cluster metadata.
					w.WriteHeader(http.StatusNotFound)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer portal.Close()
			// GetAppUpdates constructs a client even when integration mode is off.
			// Point it at a local server; no developer cluster is contacted.
			k8sutil.KubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
			k8sutil.KubernetesConfigFlags.APIServer = &portal.URL
			store.InitInMemory(store.InitInMemoryStoreOptions{Namespace: "app", ReplicatedAppEndpoint: portal.URL})
			client := fake.NewSimpleClientset()
			manager := licensestate.NewManager(client, "app", "sdk-state", "pull-secret")
			manager.SetRegistryDomains([]string{"proxy.replicated.com", "registry.replicated.com"})
			licensestate.Configure(manager)
			_, err := manager.PrepareBinding(t.Context(), "installation")
			require.NoError(t, err)
			_, err = manager.ApplyLicense(t.Context(), first, licensestate.ApplyOptions{})
			require.NoError(t, err)
			if failWrite {
				client.PrependReactor("update", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("test persistence failure")
				})
			}
			recorder := httptest.NewRecorder()
			GetAppUpdates(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/app/updates", nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			want := next
			if failWrite {
				want = first
				require.Zero(t, updateCalls, "must not use a license that failed to persist")
			} else {
				require.Equal(t, 1, updateCalls)
			}
			saved, err := manager.Load(t.Context())
			require.NoError(t, err)
			require.Equal(t, want, saved.ActiveLicense)
			wrapper, err := sdklicense.LoadLicenseFromBytes(want)
			require.NoError(t, err)
			require.Equal(t, wrapper.GetChannelID(), store.GetStore().GetLicense().GetChannelID())
			pull, err := client.CoreV1().Secrets("app").Get(t.Context(), "pull-secret", metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, want, pull.Data[licensestate.ActiveLicenseDataKey])
			require.Equal(t, corev1.SecretTypeDockerConfigJson, pull.Type)
		})
	}
}
