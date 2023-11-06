package report

import (
	"context"
	"strings"

	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetDistribution(clientset kubernetes.Interface) types.Distribution {
	// First try get the special ones. This is because sometimes we cannot get the distribution from the server version
	if distribution := distributionFromServerGroupAndResources(clientset); distribution != types.UnknownDistribution {
		return distribution
	}

	if distribution := distributionFromProviderId(clientset); distribution != types.UnknownDistribution {
		return distribution
	}

	if distribution := distributionFromLabels(clientset); distribution != types.UnknownDistribution {
		return distribution
	}

	// Getting distribution from server version string
	k8sVersion, err := k8sutil.GetK8sVersion(clientset)
	if err != nil {
		logger.Debugf("failed to get k8s version: %v", err.Error())
		return types.UnknownDistribution
	}
	if distribution := distributionFromVersion(k8sVersion); distribution != types.UnknownDistribution {
		return distribution
	}

	return types.UnknownDistribution
}

func distributionFromServerGroupAndResources(clientset kubernetes.Interface) types.Distribution {
	_, resources, _ := clientset.Discovery().ServerGroupsAndResources()
	for _, resource := range resources {
		switch {
		case strings.HasPrefix(resource.GroupVersion, "apps.openshift.io/"):
			return types.OpenShift
		case strings.HasPrefix(resource.GroupVersion, "run.tanzu.vmware.com/"):
			return types.Tanzu
		}
	}

	return types.UnknownDistribution
}

func distributionFromProviderId(clientset kubernetes.Interface) types.Distribution {
	nodes, _ := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if len(nodes.Items) >= 1 {
		node := nodes.Items[0]
		if strings.HasPrefix(node.Spec.ProviderID, "kind:") {
			return types.Kind
		}
		if strings.HasPrefix(node.Spec.ProviderID, "digitalocean:") {
			return types.DigitalOcean
		}
	}
	return types.UnknownDistribution
}

func distributionFromLabels(clientset kubernetes.Interface) types.Distribution {
	nodes, _ := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	for _, node := range nodes.Items {
		for k, v := range node.ObjectMeta.Labels {
			if k == "kurl.sh/cluster" && v == "true" {
				return types.Kurl
			} else if k == "microk8s.io/cluster" && v == "true" {
				return types.MicroK8s
			}
			if k == "kubernetes.azure.com/role" {
				return types.AKS
			}
			if k == "minikube.k8s.io/version" {
				return types.Minikube
			}
		}
	}
	return types.UnknownDistribution
}

func distributionFromVersion(k8sVersion string) types.Distribution {
	switch {
	case strings.Contains(k8sVersion, "-gke."):
		return types.GKE
	case strings.Contains(k8sVersion, "-eks-"):
		return types.EKS
	case strings.Contains(k8sVersion, "+rke2"):
		return types.RKE2
	case strings.Contains(k8sVersion, "+k3s"):
		return types.K3s
	case strings.Contains(k8sVersion, "+k0s"):
		return types.K0s
	default:
		return types.UnknownDistribution
	}
}
