package k8sutil

import (
	"net/http"
	"net/url"

	meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// proxyBypassFlags wraps ConfigFlags to force-disable HTTP proxy usage
// for Kubernetes API requests by setting rest.Config.Proxy to nil.
type proxyBypassFlags struct {
	base *genericclioptions.ConfigFlags
}

func ProxyBypassRESTClientGetter(base *genericclioptions.ConfigFlags) genericclioptions.RESTClientGetter {
	return &proxyBypassFlags{base: base}
}

func (p *proxyBypassFlags) ToRESTConfig() (*rest.Config, error) {
	cfg, err := p.base.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	// Ensure QPS/Burst defaults and disable proxy
	if cfg.QPS == 0 {
		cfg.QPS = DEFAULT_K8S_CLIENT_QPS
	}
	if cfg.Burst == 0 {
		cfg.Burst = DEFAULT_K8S_CLIENT_BURST
	}
	cfg.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	return cfg, nil
}

func (p *proxyBypassFlags) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return p.base.ToDiscoveryClient()
}

func (p *proxyBypassFlags) ToRESTMapper() (meta.RESTMapper, error) {
	return p.base.ToRESTMapper()
}

func (p *proxyBypassFlags) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return p.base.ToRawKubeConfigLoader()
}
