package util

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/segmentio/ksuid"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	replicatedConfigMapName = "replicated"
)

func GenerateIDs(namespace string) (string, string, error) {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get clientset")
	}

	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), replicatedConfigMapName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return "", "", errors.Wrap(err, "failed to get replicated configmap")
	}

	replicatedID := ""
	appID := ""

	if kuberneteserrors.IsNotFound(err) {
		replicatedID = ksuid.New().String()
		appID = ksuid.New().String()

		configmap := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      replicatedConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"replicated-id": replicatedID,
				"app-id":        appID,
			},
		}

		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &configmap, metav1.CreateOptions{})
		if err != nil {
			return "", "", errors.Wrap(err, "failed to create replicated configmap")
		}
	} else {
		replicatedID = cm.Data["replicated-id"]
		appID = cm.Data["app-id"]
	}

	return replicatedID, appID, nil
}
