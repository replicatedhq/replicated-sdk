package store

import (
	"context"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	heartbeattypes "github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
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

type InMemoryStore struct {
	clientset             kubernetes.Interface
	replicatedID          string
	appID                 string
	license               *kotsv1beta1.License
	licenseFields         sdklicensetypes.LicenseFields
	appSlug               string
	appName               string
	channelID             string
	channelName           string
	channelSequence       int64
	releaseSequence       int64
	releaseCreatedAt      string
	releaseNotes          string
	versionLabel          string
	replicatedAppEndpoint string
	namespace             string
	appStatus             appstatetypes.AppStatus
	updates               []upstreamtypes.ChannelRelease
}

type InitInMemoryStoreOptions struct {
	Clientset             kubernetes.Interface
	ReplicatedID          string
	AppID                 string
	License               *kotsv1beta1.License
	LicenseFields         sdklicensetypes.LicenseFields
	AppName               string
	ChannelID             string
	ChannelName           string
	ChannelSequence       int64
	ReleaseSequence       int64
	ReleaseCreatedAt      string
	ReleaseNotes          string
	VersionLabel          string
	ReplicatedAppEndpoint string
	Namespace             string
}

func InitInMemory(options InitInMemoryStoreOptions) {
	SetStore(&InMemoryStore{
		clientset:             options.Clientset,
		replicatedID:          options.ReplicatedID,
		appID:                 options.AppID,
		appSlug:               options.License.Spec.AppSlug,
		license:               options.License,
		licenseFields:         options.LicenseFields,
		appName:               options.AppName,
		channelID:             options.ChannelID,
		channelName:           options.ChannelName,
		channelSequence:       options.ChannelSequence,
		releaseSequence:       options.ReleaseSequence,
		releaseCreatedAt:      options.ReleaseCreatedAt,
		releaseNotes:          options.ReleaseNotes,
		versionLabel:          options.VersionLabel,
		replicatedAppEndpoint: options.ReplicatedAppEndpoint,
		namespace:             options.Namespace,
	})
}

func (s *InMemoryStore) GetReplicatedID() string {
	return s.replicatedID
}

func (s *InMemoryStore) GetAppID() string {
	return s.appID
}

func (s *InMemoryStore) GetLicense() *kotsv1beta1.License {
	return s.license
}

func (s *InMemoryStore) SetLicense(license *kotsv1beta1.License) {
	s.license = license.DeepCopy()
}

func (s *InMemoryStore) GetLicenseFields() sdklicensetypes.LicenseFields {
	return s.licenseFields
}

func (s *InMemoryStore) SetLicenseFields(licenseFields sdklicensetypes.LicenseFields) {
	// copy by value not reference
	if licenseFields == nil {
		s.licenseFields = nil
		return
	}
	if s.licenseFields == nil {
		s.licenseFields = sdklicensetypes.LicenseFields{}
	}
	for k, v := range licenseFields {
		s.licenseFields[k] = v
	}
}

func (s *InMemoryStore) IsDevLicense() bool {
	return s.license.Spec.LicenseType == "dev"
}

func (s *InMemoryStore) GetAppSlug() string {
	return s.appSlug
}

func (s *InMemoryStore) GetAppName() string {
	return s.appName
}

func (s *InMemoryStore) GetChannelID() string {
	return s.channelID
}

func (s *InMemoryStore) GetChannelName() string {
	return s.channelName
}

func (s *InMemoryStore) GetChannelSequence() int64 {
	return s.channelSequence
}

func (s *InMemoryStore) GetReleaseSequence() int64 {
	return s.releaseSequence
}

func (s *InMemoryStore) GetReleaseCreatedAt() string {
	return s.releaseCreatedAt
}

func (s *InMemoryStore) GetReleaseNotes() string {
	return s.releaseNotes
}

func (s *InMemoryStore) GetVersionLabel() string {
	return s.versionLabel
}

func (s *InMemoryStore) GetReplicatedAppEndpoint() string {
	return s.replicatedAppEndpoint
}

func (s *InMemoryStore) GetNamespace() string {
	return s.namespace
}

func (s *InMemoryStore) GetAppStatus() appstatetypes.AppStatus {
	return s.appStatus
}

func (s *InMemoryStore) SetAppStatus(status appstatetypes.AppStatus) {
	s.appStatus = status
}

func (s *InMemoryStore) GetUpdates() []upstreamtypes.ChannelRelease {
	return s.updates
}

func (s *InMemoryStore) SetUpdates(updates []upstreamtypes.ChannelRelease) {
	s.updates = updates
}

func (s *InMemoryStore) CreateInstanceReportEvent(event heartbeattypes.InstanceReportEvent) error {
	existingSecret, err := s.clientset.CoreV1().Secrets(s.GetNamespace()).Get(context.TODO(), InstanceReportSecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get airgap instance report secret")
	} else if kuberneteserrors.IsNotFound(err) {
		instanceReport := &heartbeattypes.InstanceReport{
			Events: []heartbeattypes.InstanceReportEvent{event},
		}
		data, err := instanceReport.Encode()
		if err != nil {
			return errors.Wrap(err, "failed to encode instance report")
		}

		uid, err := util.GetReplicatedDeploymentUID(s.clientset, s.GetNamespace())
		if err != nil {
			return errors.Wrap(err, "failed to get replicated deployment uid")
		}

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      InstanceReportSecretName,
				Namespace: s.GetNamespace(),
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

		_, err = s.clientset.CoreV1().Secrets(s.GetNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
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
		existingInstanceReport, err = heartbeattypes.DecodeInstanceReport(existingSecret.Data[InstanceReportSecretKey])
		if err != nil {
			return errors.Wrap(err, "failed to load existing instance report")
		}
	}

	existingInstanceReport.Events = append(existingInstanceReport.Events, event)
	if len(existingInstanceReport.Events) > InstanceReportEventLimit {
		existingInstanceReport.Events = existingInstanceReport.Events[len(existingInstanceReport.Events)-InstanceReportEventLimit:]
	}

	data, err := existingInstanceReport.Encode()
	if err != nil {
		return errors.Wrap(err, "failed to encode existing instance report")
	}

	existingSecret.Data[InstanceReportSecretKey] = data

	_, err = s.clientset.CoreV1().Secrets(s.GetNamespace()).Update(context.TODO(), existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update airgap instance report secret")
	}

	return nil
}
