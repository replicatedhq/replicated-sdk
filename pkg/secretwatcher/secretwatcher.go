package secretwatcher

import (
	"context"
	"log"

	"github.com/replicatedhq/replicated-sdk/pkg/config"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Start watches the replicated Secret for changes and updates the store with
// the latest version-related fields when the Secret is updated. This ensures
// that all SDK replicas converge on the same version data after a Helm upgrade,
// even if they were initialized from an older version of the Secret.
func Start(ctx context.Context, clientset kubernetes.Interface, namespace string, secretName string) {
	listwatch := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", secretName).String()
			return clientset.CoreV1().Secrets(namespace).List(ctx, options)
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", secretName).String()
			return clientset.CoreV1().Secrets(namespace).Watch(ctx, options)
		},
	}

	informer := cache.NewSharedInformer(listwatch, &corev1.Secret{}, 0)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			secret, ok := new.(*corev1.Secret)
			if !ok {
				return
			}
			handleSecretUpdate(secret)
		},
	})

	log.Printf("Starting secret watcher for %s/%s", namespace, secretName)
	informer.Run(ctx.Done())
}

func handleSecretUpdate(secret *corev1.Secret) {
	configData, ok := secret.Data["config.yaml"]
	if !ok {
		return
	}

	cfg, err := config.ParseReplicatedConfig(configData)
	if err != nil {
		log.Printf("Failed to parse config from secret update: %v", err)
		return
	}

	sdkStore := store.GetStore()

	sdkStore.SetChannelID(cfg.ChannelID)
	sdkStore.SetChannelName(cfg.ChannelName)
	sdkStore.SetChannelSequence(cfg.ChannelSequence)
	sdkStore.SetVersionLabel(cfg.VersionLabel)
	sdkStore.SetReleaseSequence(cfg.ReleaseSequence)
	sdkStore.SetReleaseCreatedAt(cfg.ReleaseCreatedAt)
	sdkStore.SetReleaseNotes(cfg.ReleaseNotes)
}
