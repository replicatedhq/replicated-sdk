package store

import (
	"context"

	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/kots-sdk/pkg/appstate/types"
	"github.com/replicatedhq/kots-sdk/pkg/k8sutil"
	kotslicense "github.com/replicatedhq/kots-sdk/pkg/license"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/replicatedapp"
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
	appSlug         string
	channelID       string
	channelName     string
	channelSequence int64
	releaseSequence int64
	appStatus       appstatetypes.AppStatus
}

type InitStoreOptions struct {
	License         *kotsv1beta1.License
	ChannelID       string
	ChannelName     string
	ChannelSequence int64
	ReleaseSequence int64
}

func Init(options InitStoreOptions) error {
	verifiedLicense, err := kotslicense.VerifySignature(options.License)
	if err != nil {
		return errors.Wrap(err, "failed to verify license signature")
	}

	if !util.IsAirgap() {
		// sync license
		logger.Info("syncing license with server to retrieve latest version")
		licenseData, err := replicatedapp.GetLatestLicense(verifiedLicense)
		if err != nil {
			return errors.Wrap(err, "failed to get latest license")
		}
		verifiedLicense = licenseData.License
	}

	// check license expiration
	expired, err := kotslicense.LicenseIsExpired(verifiedLicense)
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
		appSlug:         verifiedLicense.Spec.AppSlug,
		channelID:       options.ChannelID,
		channelName:     options.ChannelName,
		channelSequence: options.ChannelSequence,
		releaseSequence: options.ReleaseSequence,
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
	s.license = license
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
