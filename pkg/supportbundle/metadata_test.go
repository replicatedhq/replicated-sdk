package supportbundle

import (
	"context"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func secretDataToStringMap(data map[string][]byte) map[string]string {
	result := make(map[string]string, len(data))
	for k, v := range data {
		result[k] = string(v)
	}
	return result
}

func TestSaveMetadata_Overwrite(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	namespace := "test-ns"

	// Create the secret first (helm chart creates it)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SupportBundleMetadataSecretName,
			Namespace: namespace,
		},
	}
	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	require.NoError(t, err)

	// POST first set of data
	data1 := map[string]string{"key1": "val1", "key2": "val2"}
	err = SaveMetadata(ctx, clientset, namespace, data1, true)
	require.NoError(t, err)

	// Verify data was saved as top-level keys
	s, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SupportBundleMetadataSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, data1, secretDataToStringMap(s.Data))

	// POST (overwrite) with new data
	data2 := map[string]string{"key3": "val3"}
	err = SaveMetadata(ctx, clientset, namespace, data2, true)
	require.NoError(t, err)

	s, err = clientset.CoreV1().Secrets(namespace).Get(ctx, SupportBundleMetadataSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	saved := secretDataToStringMap(s.Data)
	require.Equal(t, data2, saved)
	require.NotContains(t, saved, "key1")
}

func TestSaveMetadata_Patch(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	namespace := "test-ns"

	// Create the secret first
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SupportBundleMetadataSecretName,
			Namespace: namespace,
		},
	}
	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Set initial data
	data1 := map[string]string{"key1": "val1", "key2": "val2"}
	err = SaveMetadata(ctx, clientset, namespace, data1, true)
	require.NoError(t, err)

	// PATCH with partial update
	data2 := map[string]string{"key2": "updated", "key3": "val3"}
	err = SaveMetadata(ctx, clientset, namespace, data2, false)
	require.NoError(t, err)

	s, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SupportBundleMetadataSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key1": "val1", "key2": "updated", "key3": "val3"}, secretDataToStringMap(s.Data))
}

func TestSaveMetadata_SecretNotFound(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	err := SaveMetadata(ctx, clientset, "test-ns", map[string]string{"key": "val"}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestSaveMetadata_ReadOnlyMode(t *testing.T) {
	req := require.New(t)

	store.InitInMemory(store.InitInMemoryStoreOptions{
		ReadOnlyMode: true,
	})
	defer store.SetStore(nil)

	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	namespace := "test-ns"

	data := map[string]string{"key": "value"}
	err := SaveMetadata(ctx, clientset, namespace, data, true)
	req.Error(err)
	req.Contains(err.Error(), "read-only mode")
}
