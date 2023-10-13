package appstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

func normalizeStatusInformers(informers []types.StatusInformer, targetNamespace string) (next []types.StatusInformer) {
	for _, informer := range informers {
		informer.Kind = getResourceKindCommonName(informer.Kind)
		if informer.Namespace == "" {
			informer.Namespace = targetNamespace
		}
		next = append(next, informer)
	}
	return
}

func filterStatusInformersByResourceKind(informers []types.StatusInformer, kind string) (next []types.StatusInformer) {
	for _, informer := range informers {
		if informer.Kind == kind {
			next = append(next, informer)
		}
	}
	return
}

func buildResourceStatesFromStatusInformers(informers []types.StatusInformer) types.ResourceStates {
	next := types.ResourceStates{}
	for _, informer := range informers {
		next = append(next, types.ResourceState{
			Kind:      informer.Kind,
			Name:      informer.Name,
			Namespace: informer.Namespace,
			State:     types.StateMissing,
		})
	}
	sort.Sort(next)
	return next
}

func resourceStatesApplyNew(resourceStates types.ResourceStates, resourceState types.ResourceState) (next types.ResourceStates) {
	for _, r := range resourceStates {
		if resourceState.Kind == r.Kind &&
			resourceState.Namespace == r.Namespace &&
			resourceState.Name == r.Name &&
			resourceState.State != r.State {
			next = append(next, resourceState)
		} else {
			next = append(next, r)
		}
	}
	sort.Sort(next)
	return
}

func GenerateStatusInformersForManifest(manifest string) []types.StatusInformerString {
	logger.Info("Generating status informers from Helm release")

	informers := []types.StatusInformerString{}

	for _, doc := range strings.Split(manifest, "\n---\n") {
		// check if the document is empty
		var obj map[string]interface{}
		err := yaml.Unmarshal([]byte(doc), &obj)
		if err != nil {
			logger.Debugf("Failed to unmarshal document to generate a status informer: %v: %v", doc, err)
			continue
		}
		if len(obj) == 0 {
			continue
		}

		unstructured := &unstructured.Unstructured{}
		_, gvk, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(doc), nil, unstructured)
		if err != nil {
			logger.Debugf("Failed to decode document to generate a status informer: %v: %v", doc, err)
			continue
		}

		namespace := unstructured.GetNamespace()
		kind := strings.ToLower(gvk.Kind)
		name := unstructured.GetName()

		switch kind {
		case "deployment", "statefulset", "daemonset", "service", "ingress", "persistentvolumeclaim":
			informer := fmt.Sprintf("%s/%s", strings.ToLower(gvk.Kind), name)
			if namespace != "" {
				informer = fmt.Sprintf("%s/%s", namespace, informer)
			}
			informers = append(informers, types.StatusInformerString(informer))
		default:
			logger.Debugf("unsupported informer for %s/%s/%s", namespace, kind, name)
		}
	}

	return informers
}
