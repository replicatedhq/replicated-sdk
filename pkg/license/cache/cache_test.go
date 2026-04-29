package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "test-ns"

// withDeployment seeds a fake clientset with the SDK's own Deployment so
// that owner-reference resolution succeeds during cache writes.
func withDeployment(t *testing.T) kubernetes.Interface {
	t.Helper()
	clientset := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "replicated",
			Namespace: testNamespace,
			UID:       apimachinerytypes.UID("deadbeef-0000-0000-0000-000000000000"),
		},
	})
	return clientset
}

func TestRead_NoSecret_ReturnsNotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	_, err := Read(context.Background(), clientset, testNamespace)
	require.ErrorIs(t, err, ErrLicenseCacheNotFound)
}

func TestRead_SecretWithoutLicenseKey_ReturnsNotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
		Data: map[string][]byte{
			KeyLicenseFields: []byte("{}"),
		},
	})

	_, err := Read(context.Background(), clientset, testNamespace)
	require.ErrorIs(t, err, ErrLicenseCacheNotFound, "fields-only secret must not satisfy a cache read")
}

func TestWriteLicense_CreatesSecret(t *testing.T) {
	clientset := withDeployment(t)
	licenseBytes := []byte("license-yaml-document")

	require.NoError(t, WriteLicense(context.Background(), clientset, testNamespace, licenseBytes, false))

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), SecretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, licenseBytes, secret.Data[KeyLicense])
	require.NotEmpty(t, secret.Data[KeyLastFetched], "last-fetched must be set on every write")

	require.Len(t, secret.OwnerReferences, 1)
	require.Equal(t, "Deployment", secret.OwnerReferences[0].Kind)
}

func TestWriteLicenseFields_PreservesLicense(t *testing.T) {
	clientset := withDeployment(t)
	ctx := context.Background()

	licenseBytes := []byte("license-yaml-document")
	require.NoError(t, WriteLicense(ctx, clientset, testNamespace, licenseBytes, false))

	fields := licensetypes.LicenseFields{
		"my_field": licensetypes.LicenseField{
			Title: "My Field",
			Value: "v1",
		},
	}
	require.NoError(t, WriteLicenseFields(ctx, clientset, testNamespace, fields, false))

	cached, err := Read(ctx, clientset, testNamespace)
	require.NoError(t, err)
	require.Equal(t, licenseBytes, cached.LicenseBytes, "license bytes must survive a fields-only write")
	require.Equal(t, "v1", cached.LicenseFields["my_field"].Value)
}

func TestWrite_RefreshesLastFetched(t *testing.T) {
	clientset := withDeployment(t)
	ctx := context.Background()

	require.NoError(t, WriteLicense(ctx, clientset, testNamespace, []byte("v1"), false))
	cached1, err := Read(ctx, clientset, testNamespace)
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond) // RFC3339 has 1s resolution.

	require.NoError(t, WriteLicense(ctx, clientset, testNamespace, []byte("v2"), false))
	cached2, err := Read(ctx, clientset, testNamespace)
	require.NoError(t, err)

	require.True(t, cached2.LastFetched.After(cached1.LastFetched), "last-fetched must advance on subsequent writes")
}

func TestWrite_ReadOnlyMode_NoSecretCreated(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	require.NoError(t, WriteLicense(context.Background(), clientset, testNamespace, []byte("license"), true))

	_, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), SecretName, metav1.GetOptions{})
	require.True(t, kuberneteserrors.IsNotFound(err), "no secret should be created in read-only mode")
}

// TestWrite_ReadOnlyMode_BeforeStoreInit verifies that the read-only
// gate works during early bootstrap, before store.InitInMemory has run.
// Pre-fix this would have observed the empty fallback InMemoryStore's
// readOnlyMode=false default, attempted the write, and only failed via
// RBAC (with a confusing log line). Now the read-only flag is passed
// explicitly so no global-state observation is needed.
func TestWrite_ReadOnlyMode_BeforeStoreInit(t *testing.T) {
	store.SetStore(nil) // explicitly: no store installed.
	clientset := fake.NewSimpleClientset()

	require.NoError(t, WriteLicense(context.Background(), clientset, testNamespace, []byte("license"), true))
	require.NoError(t, WriteLicenseFields(context.Background(), clientset, testNamespace, licensetypes.LicenseFields{}, true))

	_, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), SecretName, metav1.GetOptions{})
	require.True(t, kuberneteserrors.IsNotFound(err), "no secret should be created when caller declares read-only mode, regardless of store init state")
}

func TestWriteLicense_RejectsEmpty(t *testing.T) {
	clientset := withDeployment(t)

	require.Error(t, WriteLicense(context.Background(), clientset, testNamespace, nil, false),
		"writing empty license bytes must be rejected to avoid corrupting the cache")
}

func TestRead_MalformedFields_ReturnsLicenseWithoutFields(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
		Data: map[string][]byte{
			KeyLicense:       []byte("license-yaml"),
			KeyLicenseFields: []byte("not valid json"),
		},
	})

	cached, err := Read(context.Background(), clientset, testNamespace)
	require.NoError(t, err, "malformed fields must not fail the read")
	require.NotNil(t, cached.LicenseBytes)
	require.Nil(t, cached.LicenseFields, "malformed fields are dropped, not surfaced")
}

// Sanity check: cached fields round-trip through JSON without losing the
// LicenseFieldSignature struct.
func TestRead_RoundTripsFieldSignatures(t *testing.T) {
	encoded, err := json.Marshal(licensetypes.LicenseFields{
		"f1": licensetypes.LicenseField{
			Title: "F1",
			Value: 42.0,
			Signature: licensetypes.LicenseFieldSignature{
				V1: "sig-v1",
				V2: "sig-v2",
			},
		},
	})
	require.NoError(t, err)

	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: testNamespace},
		Data: map[string][]byte{
			KeyLicense:       []byte("license-yaml"),
			KeyLicenseFields: encoded,
		},
	})

	cached, err := Read(context.Background(), clientset, testNamespace)
	require.NoError(t, err)
	require.Equal(t, "sig-v1", cached.LicenseFields["f1"].Signature.V1)
	require.Equal(t, "sig-v2", cached.LicenseFields["f1"].Signature.V2)
}
