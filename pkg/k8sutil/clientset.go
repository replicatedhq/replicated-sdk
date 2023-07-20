package k8sutil

import (
	"strconv"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	DEFAULT_K8S_CLIENT_QPS   = 100
	DEFAULT_K8S_CLIENT_BURST = 100
)

var KubernetesConfigFlags *genericclioptions.ConfigFlags

func init() {
	KubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
}

func AddFlags(flags *flag.FlagSet) {
	KubernetesConfigFlags.AddFlags(flags)
}

func GetClientset() (*kubernetes.Clientset, error) {
	cfg, err := GetClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cluster config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes clientset")
	}

	return clientset, nil
}

func GetClusterConfig() (*rest.Config, error) {
	var cfg *rest.Config
	var err error

	if KubernetesConfigFlags != nil {
		cfg, err = KubernetesConfigFlags.ToRESTConfig()
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
		}
	} else {
		cfg, err = config.GetConfig()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get config")
		}
	}

	cfg.QPS = DEFAULT_K8S_CLIENT_QPS
	cfg.Burst = DEFAULT_K8S_CLIENT_BURST

	return cfg, nil
}

func GetK8sVersion(clientset kubernetes.Interface) (string, error) {
	k8sVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", errors.Wrap(err, "failed to get kubernetes server version")
	}
	return k8sVersion.GitVersion, nil
}

func GetK8sMinorVersion(clientset kubernetes.Interface) (int, error) {
	k8sVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return -1, errors.Wrap(err, "failed to get kubernetes server version")
	}

	k8sMinorVersion, err := strconv.Atoi(k8sVersion.Minor)
	if err != nil {
		return -1, errors.Wrap(err, "failed to convert k8s minor version to int")
	}
	return k8sMinorVersion, nil
}
