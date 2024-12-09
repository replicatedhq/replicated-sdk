package meta

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type replicatedMetadataSecretKey string

func (m replicatedMetadataSecretKey) String() string {
	return string(m)
}

const (
	ReplicatedMetadataSecretName string = "replicated-meta-data"
)

var replicatedSecretLock = sync.Mutex{}

func save(ctx context.Context, clientset kubernetes.Interface, namespace string, key replicatedMetadataSecretKey, data interface{}) error {

	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	encodedData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal instance tags")
	}

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, ReplicatedMetadataSecretName, metav1.GetOptions{})
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
				Name:      string(ReplicatedMetadataSecretName),
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
				string(key): encodedData,
			},
		}

		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create meta secret")
		}
		return nil
	}

	if existingSecret.Data == nil {
		existingSecret.Data = map[string][]byte{}
	}

	existingSecret.Data[string(key)] = encodedData

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to update replicated-meta-data secret with key %s", key)
	}

	return nil
}

var (
	ErrReplicatedMetadataNotFound = errors.New("replicated metadata not found")
)

func get(ctx context.Context, clientset kubernetes.Interface, namespace string, key replicatedMetadataSecretKey, v interface{}) error {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, ReplicatedMetadataSecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get replicated-meta-data secret")
	}

	if kuberneteserrors.IsNotFound(err) {
		return ErrReplicatedMetadataNotFound
	}

	if len(secret.Data) == 0 {
		return ErrReplicatedMetadataNotFound
	}

	dataBytes, ok := secret.Data[string(key)]
	if !ok || len(dataBytes) == 0 {
		return errors.Wrapf(ErrReplicatedMetadataNotFound, "key (%s) not found in secret", key)
	}

	if err := json.Unmarshal(dataBytes, v); err != nil {
		logger.Infof("failed to unmarshal secret data for key %s: %v", key, err)
		return ErrReplicatedMetadataNotFound
	}

	return nil
}
