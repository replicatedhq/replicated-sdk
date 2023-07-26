package k8sutil

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsReplicatedClusterScoped will check if replicated-sdk has cluster scope access or not
func IsReplicatedClusterScoped(ctx context.Context, clientset kubernetes.Interface, namespace string) bool {
	rb, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "replicated-sdk-rolebinding", metav1.GetOptions{})
	if err != nil {
		return false
	}
	for _, s := range rb.Subjects {
		if s.Kind != "ServiceAccount" {
			continue
		}
		if s.Name != "replicated-sdk" {
			continue
		}
		if s.Namespace != "" && s.Namespace == namespace {
			return true
		}
		if s.Namespace == "" && namespace == metav1.NamespaceDefault {
			return true
		}
	}
	return false
}
