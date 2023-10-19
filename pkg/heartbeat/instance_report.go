package heartbeat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	heartbeattypes "github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	InstanceReportSecretName = "replicated-instance-report"
	InstanceReportSecretKey  = "report"
	InstanceReportEventLimit = 4000
)

var instanceReportMtx = sync.Mutex{}

func CreateInstanceReportEvent(clientset kubernetes.Interface, namespace string, event heartbeattypes.InstanceReportEvent) error {
	instanceReportMtx.Lock()
	defer instanceReportMtx.Unlock()

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), InstanceReportSecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get airgap instance report secret")
	} else if kuberneteserrors.IsNotFound(err) {
		instanceReport := &heartbeattypes.InstanceReport{
			Events: []heartbeattypes.InstanceReportEvent{event},
		}
		data, err := EncodeInstanceReport(instanceReport)
		if err != nil {
			return errors.Wrap(err, "failed to encode instance report")
		}

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
				Name:      InstanceReportSecretName,
				Namespace: namespace,
				// since this secret is created by the replicated deployment, we should set the owner reference
				// so that it is deleted when the replicated deployment is deleted
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
				InstanceReportSecretKey: data,
			},
		}

		_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create airgap instance report secret")
		}

		return nil
	}

	if existingSecret.Data == nil {
		existingSecret.Data = map[string][]byte{}
	}

	existingInstanceReport := &heartbeattypes.InstanceReport{}
	if existingSecret.Data[InstanceReportSecretKey] != nil {
		existingInstanceReport, err = DecodeInstanceReport(existingSecret.Data[InstanceReportSecretKey])
		if err != nil {
			return errors.Wrap(err, "failed to load existing instance report")
		}
	}

	existingInstanceReport.Events = append(existingInstanceReport.Events, event)
	if len(existingInstanceReport.Events) > InstanceReportEventLimit {
		existingInstanceReport.Events = existingInstanceReport.Events[len(existingInstanceReport.Events)-InstanceReportEventLimit:]
	}

	data, err := EncodeInstanceReport(existingInstanceReport)
	if err != nil {
		return errors.Wrap(err, "failed to encode existing instance report")
	}

	existingSecret.Data[InstanceReportSecretKey] = data

	_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update airgap instance report secret")
	}

	return nil
}

func EncodeInstanceReport(r *heartbeattypes.InstanceReport) ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal instance report")
	}
	compressedData, err := util.GzipData(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to gzip instance report")
	}
	encodedData := base64.StdEncoding.EncodeToString(compressedData)

	return []byte(encodedData), nil
}

func DecodeInstanceReport(encodedData []byte) (*heartbeattypes.InstanceReport, error) {
	decodedData, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode instance report")
	}
	decompressedData, err := util.GunzipData(decodedData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to gunzip instance report")
	}

	instanceReport := &heartbeattypes.InstanceReport{}
	if err := json.Unmarshal(decompressedData, instanceReport); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal instance report")
	}

	return instanceReport, nil
}
