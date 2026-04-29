package store

import (
	"sync/atomic"

	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

// storePtr holds the package-level Store as an atomic.Pointer so that
// handler goroutines (calling GetStore on every request) can safely
// observe stores written by bootstrapCritical's InitInMemory call.
//
// Pre-Phase-1, the HTTP listener didn't accept traffic until after
// InitInMemory had returned, so a plain interface variable was safe by
// virtue of happens-before via process startup ordering. With the
// listener-up-before-bootstrap refactor, request handlers and the
// bootstrap goroutine race on this variable: an unsynchronized interface
// (itab + data pointer = two words) can be torn under concurrent
// reads/writes.
//
// We store *Store rather than Store directly because atomic.Pointer
// requires a pointer type. SetStore(nil) is preserved as "clear the
// store" (used by test cleanup) by storing a nil *Store.
var storePtr atomic.Pointer[Store]

var _ Store = (*InMemoryStore)(nil)

type Store interface {
	GetReplicatedID() string
	GetAppID() string
	GetLicense() licensewrapper.LicenseWrapper
	SetLicense(license licensewrapper.LicenseWrapper)
	GetLicenseFields() licensetypes.LicenseFields
	SetLicenseFields(licenseFields licensetypes.LicenseFields)
	IsDevLicense() bool
	GetAppSlug() string
	GetAppName() string
	GetChannelID() string
	GetChannelName() string
	GetChannelSequence() int64
	GetReleaseSequence() int64
	GetReleaseCreatedAt() string
	GetReleaseNotes() string
	GetVersionLabel() string
	GetReplicatedAppEndpoint() string
	GetReleaseImages() []string
	GetNamespace() string
	GetAppStatus() appstatetypes.AppStatus
	SetAppStatus(status appstatetypes.AppStatus)
	// Pod image tracking
	SetPodImages(namespace string, podUID string, images []appstatetypes.ImageInfo)
	DeletePodImages(namespace string, podUID string)
	GetRunningImages() map[string][]string
	GetUpdates() []upstreamtypes.ChannelRelease
	SetUpdates(updates []upstreamtypes.ChannelRelease)
	GetReportAllImages() bool
	GetReadOnlyMode() bool
}

// SetStore atomically installs s as the package-level store. Passing nil
// clears the store; subsequent GetStore calls will return a fresh empty
// InMemoryStore until a non-nil value is installed. Test cleanup paths
// rely on the nil-clear behavior.
func SetStore(s Store) {
	if s == nil {
		storePtr.Store(nil)
		return
	}
	storePtr.Store(&s)
}

// GetStore returns the currently installed store. If no store has been
// installed yet (or after SetStore(nil)), it returns a fresh empty
// InMemoryStore so handlers always have a usable, non-panicking target.
// Note that the empty-store fallback is per-call and not persisted: the
// pod is briefly observable in this state if the listener accepts a
// request before bootstrapCritical's InitInMemory completes, which is
// the same window /healthz reports as Starting → 503.
func GetStore() Store {
	p := storePtr.Load()
	if p == nil {
		return &InMemoryStore{}
	}
	return *p
}
