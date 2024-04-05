package tags

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/tags/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	InstanceMetadataSecretName = "replicated-meta-data"
	InstanceTagSecretKey       = "instance-tag-data"
)

var replicatedSecretLock = sync.Mutex{}

func Save(ctx context.Context, clientset kubernetes.Interface, namespace string, tdata types.InstanceTagData) error {

	replicatedSecretLock.Lock()
	defer replicatedSecretLock.Unlock()

	encodedTagData, err := tdata.MarshalBase64()
	if err != nil {
		return errors.Wrap(err, "failed to marshal instance tags")
	}

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, InstanceMetadataSecretName, metav1.GetOptions{})
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
				Name:      InstanceMetadataSecretName,
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
				InstanceTagSecretKey: encodedTagData,
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

	existingSecret.Data[InstanceTagSecretKey] = encodedTagData

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update instance-tags secret")
	}

	return nil
}

var (
	ErrInstanceTagDataIsEmpty        = errors.New("instance tag data is empty")
	ErrInstanceTagDataSecretNotFound = errors.New("instance tag secret not found")
)

func Get(ctx context.Context, clientset kubernetes.Interface, namespace string) (*types.InstanceTagData, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, InstanceMetadataSecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get instance-tags secret")
	}

	if kuberneteserrors.IsNotFound(err) {
		return nil, ErrInstanceTagDataSecretNotFound
	}

	if len(secret.Data) == 0 {
		return nil, ErrInstanceTagDataIsEmpty
	}

	tagDataBytes, ok := secret.Data[InstanceTagSecretKey]
	if !ok || len(tagDataBytes) == 0 {
		return nil, ErrInstanceTagDataIsEmpty
	}

	tagData := &types.InstanceTagData{}
	if err := tagData.UnmarshalBase64(tagDataBytes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal instance tags")
	}

	return tagData, nil
}
