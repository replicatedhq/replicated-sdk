package report

import (
	"encoding/json"
	"testing"

	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAirgapReportsNeverStoreInstallationCredential(t *testing.T) {
	token, err := installationtoken.New("installation", 2)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	previous := store.GetStore()
	t.Cleanup(func() { store.SetStore(previous) })
	store.InitInMemory(store.InitInMemoryStoreOptions{Namespace: "app", AppID: "sdk-instance", License: licensewrapper.LicenseWrapper{
		V2: &kotsv1beta2.License{Spec: kotsv1beta2.LicenseSpec{LicenseID: encoded}},
	}})
	instance, metrics := &InstanceReport{}, &CustomAppMetricsReport{}
	client := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: instance.GetSecretName(), Namespace: "app"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: metrics.GetSecretName(), Namespace: "app"}},
	)
	require.NoError(t, SendAirgapInstanceData(client, "app", encoded, &types.InstanceData{InstanceID: "sdk-instance", ClusterID: "cluster"}))
	require.NoError(t, SendAirgapCustomAppMetrics(client, store.GetStore(), map[string]interface{}{"requests": 1}))
	for _, report := range []Report{instance, metrics} {
		secret, err := client.CoreV1().Secrets("app").Get(t.Context(), report.GetSecretName(), metav1.GetOptions{})
		require.NoError(t, err)
		decoded, err := DecodeReport(secret.Data[report.GetSecretKey()], report.GetType())
		require.NoError(t, err)
		body, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.NotContains(t, string(body), encoded)
		require.NotContains(t, string(body), token.Secret)
		require.Contains(t, string(body), `"installation_id":"installation"`)
		require.Contains(t, string(body), `"license_id":""`)
	}
	legacy, id := reportLicenseIdentity("legacy-license")
	require.Equal(t, "legacy-license", legacy)
	require.Empty(t, id)
}
