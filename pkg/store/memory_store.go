package store

import (
	"context"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/segmentio/ksuid"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	replicatedConfigMapName = "replicated"
)

var (
	store Store
)

type InMemoryStore struct {
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
}

type InitInMemoryStoreOptions struct {
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

func InitInMemory(options InitInMemoryStoreOptions) error {
	verifiedLicense, err := sdklicense.VerifySignature(options.License)
	if err != nil {
		return errors.Wrap(err, "failed to verify license signature")
	}

	if !util.IsAirgap() {
		// sync license
		logger.Info("syncing license with server to retrieve latest version")
		licenseData, err := sdklicense.GetLatestLicense(verifiedLicense, options.ReplicatedAppEndpoint)
		if err != nil {
			return errors.Wrap(err, "failed to get latest license")
		}
		verifiedLicense = licenseData.License
	}

	// check license expiration
	expired, err := sdklicense.LicenseIsExpired(verifiedLicense)
	if err != nil {
		return errors.Wrapf(err, "failed to check if license is expired")
	}
	if expired {
		return errors.New("license is expired")
	}

	// generate / retrieve sdk and app ids
	replicatedID, appID, err := generateIDs(options.Namespace)
	if err != nil {
		return errors.Wrap(err, "failed to generate ids")
	}

	store = &InMemoryStore{
		replicatedID:          replicatedID,
		appID:                 appID,
		license:               verifiedLicense,
		licenseFields:         options.LicenseFields,
		appSlug:               verifiedLicense.Spec.AppSlug,
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
	}

	return nil
}

func GetStore() Store {
	if store == nil {
		return &InMemoryStore{}
	}

	return store
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
	if s.appStatus.State == "" {
		// initialize with none state so that subsequent changes will trigger reporting
		return appstatetypes.AppStatus{
			AppSlug:  s.appSlug,
			Sequence: s.releaseSequence,
			State:    appstatetypes.StateMissing,
		}
	}
	return s.appStatus
}

func (s *InMemoryStore) SetAppStatus(status appstatetypes.AppStatus) {
	s.appStatus = status
}

func generateIDs(namespace string) (string, string, error) {
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
