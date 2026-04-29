package store

import (
	"sync"
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

// fallbackStore is the single empty InMemoryStore that GetStore returns
// when no real store has been installed yet. Lazily created on first
// access so unused code paths (especially tests that never call any
// store function) don't pay the allocation. We deliberately reuse the
// same instance across all "store not yet installed" GetStore calls so
// that a sequence like `store.GetStore().SetX(v); store.GetStore().GetX()`
// observes its own write rather than seeing a fresh zero-value store on
// the second call. This window is observable in the resilient-mode
// timeout path, where the pod is marked Ready (and accepts traffic)
// before bootstrapCritical has had a chance to install the real store
// via InitInMemory.
var (
	fallbackStoreOnce sync.Once
	fallbackStore     *InMemoryStore
)

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

// GetStore returns the currently installed store, or a single shared
// empty fallback InMemoryStore when none has been installed yet. The
// fallback is process-wide (allocated lazily on first need) rather than
// per-call so that mutations performed against it persist across
// consecutive GetStore reads — handlers that do, e.g.,
// `s := GetStore(); s.SetX(v); s2 := GetStore(); _ = s2.GetX()` must
// observe their own writes even before InitInMemory installs the real
// store. Once SetStore is called with a real store, GetStore returns
// that real store and the fallback becomes orphaned; any writes that
// landed on the fallback during the bootstrap window are lost. That is
// an accepted trade-off: the alternative (blocking handlers behind
// bootstrap) is exactly the CrashLoopBackOff regression Phase 1 set
// out to fix.
func GetStore() Store {
	if p := storePtr.Load(); p != nil {
		return *p
	}
	fallbackStoreOnce.Do(func() {
		fallbackStore = &InMemoryStore{}
	})
	return fallbackStore
}
