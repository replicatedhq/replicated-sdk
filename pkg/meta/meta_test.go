package meta

import (
	"context"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_save_ReadOnlyMode(t *testing.T) {
	req := require.New(t)

	store.InitInMemory(store.InitInMemoryStoreOptions{
		ReadOnlyMode: true,
	})
	defer store.SetStore(nil)

	clientset := fake.NewSimpleClientset()

	err := save(context.Background(), clientset, "test-ns", instanceTagSecretKey, map[string]string{"key": "value"})
	req.NoError(err)

	// Verify no secret was created
	_, err = clientset.CoreV1().Secrets("test-ns").Get(context.Background(), ReplicatedMetadataSecretName, metav1.GetOptions{})
	req.True(kuberneteserrors.IsNotFound(err), "secret should not have been created in read-only mode")
}
