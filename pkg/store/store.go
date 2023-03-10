package store

import (
	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/kots-sdk/pkg/appstate/types"
	kotslicense "github.com/replicatedhq/kots-sdk/pkg/license"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/replicatedapp"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

var (
	store *Store
)

type Store struct {
	license         *kotsv1beta1.License
	appSlug         string
	channelID       string
	channelName     string
	channelSequence int64
	releaseSequence int64
	appStatus       appstatetypes.AppStatus
	statusInformers []appstatetypes.StatusInformerString
}

type InitStoreOptions struct {
	License         *kotsv1beta1.License
	ChannelID       string
	ChannelName     string
	ChannelSequence int64
	ReleaseSequence int64
	StatusInformers []string
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

	informers := []appstatetypes.StatusInformerString{}
	for _, informer := range options.StatusInformers {
		informers = append(informers, appstatetypes.StatusInformerString(informer))
	}

	store = &Store{
		license:         verifiedLicense,
		appSlug:         verifiedLicense.Spec.AppSlug,
		channelID:       options.ChannelID,
		channelName:     options.ChannelName,
		channelSequence: options.ChannelSequence,
		releaseSequence: options.ReleaseSequence,
		statusInformers: informers,
	}

	return nil
}

func GetStore() *Store {
	if store == nil {
		return &Store{}
	}

	return store
}

func (s *Store) GetLicense() *kotsv1beta1.License {
	return s.license
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

func (s *Store) GetStatusInformers() []appstatetypes.StatusInformerString {
	return s.statusInformers
}
