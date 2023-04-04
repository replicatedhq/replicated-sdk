package store

import (
	"context"

	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/kots-sdk/pkg/appstate/types"
	"github.com/replicatedhq/kots-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/kots-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/kots-sdk/pkg/license/types"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/segmentio/ksuid"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kotsSDKConfigMapName = "kots-sdk"
)

var (
	store *Store
)

type Store struct {
	kotsSDKID       string
	appID           string
	license         *kotsv1beta1.License
	licenseFields   sdklicensetypes.LicenseFields
	appSlug         string
	channelID       string
	channelName     string
	channelSequence int64
	releaseSequence int64
	versionLabel    string
	appStatus       appstatetypes.AppStatus
}

type InitStoreOptions struct {
	License         *kotsv1beta1.License
	LicenseFields   sdklicensetypes.LicenseFields
	ChannelID       string
	ChannelName     string
	ChannelSequence int64
	ReleaseSequence int64
	VersionLabel    string
}

func Init(options InitStoreOptions) error {
	verifiedLicense, err := sdklicense.VerifySignature(options.License)
	if err != nil {
		return errors.Wrap(err, "failed to verify license signature")
	}

	if !util.IsAirgap() {
		// sync license
		logger.Info("syncing license with server to retrieve latest version")
		licenseData, err := sdklicense.GetLatestLicense(verifiedLicense)
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
	kotsSDKID, appID, err := generateIDs()
	if err != nil {
		return errors.Wrap(err, "failed to generate ids")
	}

	store = &Store{
		kotsSDKID:       kotsSDKID,
		appID:           appID,
		license:         verifiedLicense,
		licenseFields:   options.LicenseFields,
		appSlug:         verifiedLicense.Spec.AppSlug,
		channelID:       options.ChannelID,
		channelName:     options.ChannelName,
		channelSequence: options.ChannelSequence,
		releaseSequence: options.ReleaseSequence,
		versionLabel:    options.VersionLabel,
	}

	return nil
}

func GetStore() *Store {
	if store == nil {
		return &Store{}
	}

	return store
}

func (s *Store) GetKotsSDKID() string {
	return s.kotsSDKID
}

func (s *Store) GetAppID() string {
	return s.appID
}

func (s *Store) GetLicense() *kotsv1beta1.License {
	return s.license
}

func (s *Store) SetLicense(license *kotsv1beta1.License) {
	s.license = license.DeepCopy()
}

func (s *Store) GetLicenseFields() sdklicensetypes.LicenseFields {
	return s.licenseFields
}

func (s *Store) SetLicenseFields(licenseFields sdklicensetypes.LicenseFields) {
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

func (s *Store) GetAppSlug() string {
	return s.appSlug
}

func (s *Store) GetChannelID() string {
	return s.channelID
}

func (s *Store) GetChannelName() string {
	return s.channelName
}

func (s *Store) GetChannelSequence() int64 {
	return s.channelSequence
}

func (s *Store) GetReleaseSequence() int64 {
	return s.releaseSequence
}

func (s *Store) GetVersionLabel() string {
	return s.versionLabel
}

func (s *Store) GetAppStatus() appstatetypes.AppStatus {
	if s.appStatus.State == "" {
		return appstatetypes.AppStatus{
			AppSlug:  s.appSlug,
			Sequence: s.releaseSequence,
			State:    appstatetypes.StateMissing,
		}
	}
	return s.appStatus
}

func (s *Store) SetAppStatus(status appstatetypes.AppStatus) {
	s.appStatus = status
}

func generateIDs() (string, string, error) {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get clientset")
	}

	cm, err := clientset.CoreV1().ConfigMaps(util.PodNamespace).Get(context.TODO(), kotsSDKConfigMapName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return "", "", errors.Wrap(err, "failed to get kots sdk configmap")
	}

	kotsSDKID := ""
	appID := ""

	if kuberneteserrors.IsNotFound(err) {
		kotsSDKID = ksuid.New().String()
		appID = ksuid.New().String()

		configmap := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      kotsSDKConfigMapName,
				Namespace: util.PodNamespace,
			},
			Data: map[string]string{
				"kots-sdk-id": kotsSDKID,
				"app-id":      appID,
			},
		}

		_, err := clientset.CoreV1().ConfigMaps(util.PodNamespace).Create(context.TODO(), &configmap, metav1.CreateOptions{})
		if err != nil {
			return "", "", errors.Wrap(err, "failed to create kots sdk configmap")
		}
	} else {
		kotsSDKID = cm.Data["kots-sdk-id"]
		appID = cm.Data["app-id"]
	}

	return kotsSDKID, appID, nil
}
