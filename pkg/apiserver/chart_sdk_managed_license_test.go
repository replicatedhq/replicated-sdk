package apiserver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

func TestSDKManagedLicenseChartSecretPermissions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "sdkManagedLicense"
		}
		t.Run(name, func(t *testing.T) {
			chart, err := loader.Load("../../chart")
			require.NoError(t, err)
			values, err := chartutil.ToRenderValues(chart, map[string]interface{}{
				"minimalRBAC":       true,
				"sdkManagedLicense": map[string]interface{}{"enabled": enabled},
			}, chartutil.ReleaseOptions{Name: "replicated", Namespace: "app", IsInstall: true}, nil)
			require.NoError(t, err)
			rendered, err := engine.Render(chart, values)
			require.NoError(t, err)
			var role rbacv1.Role
			require.NoError(t, yaml.Unmarshal([]byte(rendered[chart.Name()+"/templates/replicated-role.yaml"]), &role))
			require.NotEmpty(t, role.Rules)
			var deleted []string
			for _, rule := range role.Rules {
				for _, verb := range rule.Verbs {
					if verb == "delete" {
						deleted = append(deleted, rule.ResourceNames...)
					}
				}
				if !enabled {
					require.NotContains(t, rule.ResourceNames, "replicated-sdk-state")
					require.NotContains(t, rule.ResourceNames, "replicated-sdk-bootstrap")
				}
			}
			if enabled {
				require.ElementsMatch(t, []string{"enterprise-pull-secret", "replicated-sdk-bootstrap"}, deleted)
			} else {
				require.Empty(t, deleted, "legacy installs must not gain delete permission")
			}
		})
	}
}
