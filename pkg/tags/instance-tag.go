package tags

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/tags/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetInstanceTagDataSecretName() string {
	return "replicated-instance-tag-data"
}

func GetSecretKey() string {
	return "instance-tag-data"
}

var replicatedSecretLock = sync.Mutex{}

func SyncInstanceTags(ctx context.Context, clientset kubernetes.Interface, namespace string, tdata types.InstanceTagData) error {

	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	tagDataBytes, err := json.Marshal(tdata)
	if err != nil {
		return errors.Wrap(err, "failed to marshal instance tags")
	}

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, GetInstanceTagDataSecretName(), metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {

		return errors.Wrap(err, "failed to get instance-tags secret")
	}

	if kuberneteserrors.IsNotFound(err) {
		uid, err := util.GetReplicatedDeploymentUID(clientset, namespace)
		if err != nil {
			return errors.Wrap(err, "failed to get replicated deployment uid")
		}

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      GetInstanceTagDataSecretName(),
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       util.GetReplicatedDeploymentName(),
						UID:        uid,
					},
				},
			},
			Data: map[string][]byte{
				GetSecretKey(): tagDataBytes,
			},
		}

		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create report secret")
		}
		return nil
	}

	if existingSecret.Data == nil {
		existingSecret.Data = map[string][]byte{}
	}

	existingSecret.Data[GetSecretKey()] = tagDataBytes

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update instance-tags secret")
	}

	return nil
}
