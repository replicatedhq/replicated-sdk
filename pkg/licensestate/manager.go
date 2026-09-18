package licensestate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Manager serializes local license applications. Kubernetes resource versions
// also protect individual Secret writes from concurrent updates.
type Manager struct {
	client              kubernetes.Interface
	namespace           string
	stateSecretName     string
	imagePullSecretName string
	// registryDomains are supplied by the installer, which knows whether this
	// installation pulls from the Replicated proxy, a customer registry, or an
	// airgap registry. Rotation rewrites the credential for these same hosts.
	registryDomains []string
	mu              sync.Mutex
}

// SetRegistryDomains configures the hosts written into the image pull Secret.
func (m *Manager) SetRegistryDomains(domains []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registryDomains = append([]string(nil), domains...)
}

func (m *Manager) RegistryDomains() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.registryDomains...)
}

func NewManager(client kubernetes.Interface, namespace, stateSecretName, imagePullSecretName string) *Manager {
	if stateSecretName == "" {
		stateSecretName = DefaultStateSecretName
	}
	if imagePullSecretName == "" {
		imagePullSecretName = DefaultImagePullSecretName
	}
	return &Manager{client: client, namespace: namespace, stateSecretName: stateSecretName, imagePullSecretName: imagePullSecretName}
}

func (m *Manager) StateSecretName() string     { return m.stateSecretName }
func (m *Manager) ImagePullSecretName() string { return m.imagePullSecretName }

// PrepareBinding durably creates the SDK keypair before the Portal bind call.
// It never replaces an existing key. Key-loss recovery is not supported here.
func (m *Manager) PrepareBinding(ctx context.Context, installationID string) (*State, error) {
	if installationID == "" {
		return nil, errors.New("Portal-assigned installation ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for attempts := 0; attempts < 5; attempts++ {
		secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "get SDK license state Secret")
		}
		if apierrors.IsNotFound(err) {
			secret = nil
		}
		state := &State{Version: 1, Status: StatusUnbound}
		if err == nil {
			raw := secret.Data[stateSecretDataKey]
			if len(raw) == 0 {
				return nil, errors.New("SDK license state Secret is missing state.json")
			}
			if err := json.Unmarshal(raw, state); err != nil {
				return nil, errors.Wrap(err, "decode SDK license state")
			}
		}
		if state.InstallationID != "" {
			if state.InstallationID != installationID {
				return nil, errors.New("bootstrap license belongs to a different installation")
			}
			if len(state.PrivateKey) != ed25519.PrivateKeySize || len(state.PublicKey) != ed25519.PublicKeySize {
				return nil, errors.New("SDK installation key is missing; manual recovery is required")
			}
			return state, nil
		}
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, errors.Wrap(err, "generate installation identity")
		}
		state.InstallationID, state.PrivateKey, state.PublicKey = installationID, privateKey, publicKey
		_, err = m.writeState(ctx, secret, state)
		if apierrors.IsConflict(errors.Cause(err)) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return state, nil
	}
	return nil, errors.New("concurrent SDK identity initialization prevented binding")
}

// Load returns Unbound state if the SDK-owned Secret does not exist. It never
// synthesizes a new identity during a read, so accidental loss cannot silently
// become a different installation.
func (m *Manager) Load(ctx context.Context) (*State, error) {
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return &State{Version: 1, Status: StatusUnbound}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "get SDK license state Secret")
	}
	raw := secret.Data[stateSecretDataKey]
	if len(raw) == 0 {
		return nil, errors.New("SDK license state Secret is missing state.json")
	}
	state := &State{}
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, errors.Wrap(err, "decode SDK license state")
	}
	if state.Version != 1 {
		return nil, fmt.Errorf("unsupported SDK license state version %d", state.Version)
	}
	return state, nil
}

// LoadOnlineState reports lost identity without recreating it. A shared pull
// Secret containing an active license proves this is not an unbound install,
// but it is not a backup of the private SDK state and cannot restore the key.
func (m *Manager) LoadOnlineState(ctx context.Context) (*State, error) {
	state, err := m.Load(ctx)
	if err != nil {
		return nil, err
	}
	lost := state.InstallationID != "" && (len(state.PrivateKey) != ed25519.PrivateKeySize || len(state.PublicKey) != ed25519.PublicKeySize)
	if state.Status == StatusUnbound && state.InstallationID == "" {
		pull, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.imagePullSecretName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "check existing SDK installation")
		}
		lost = err == nil && len(pull.Data[ActiveLicenseDataKey]) > 0
	}
	if lost {
		state.Status = StatusManualRecoveryRequired
		state.LastError = "SDK installation state or private key is missing; manual recovery is required"
	}
	return state, nil
}

func fingerprint(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ApplyOptions carries the caller's expected identity and rotation metadata.
// Identity must match the signed license; these fields cannot override it.
// Legacy licenses stay outside this manager.
type ApplyOptions struct {
	ApplicationID        string
	CustomerID           string
	TargetInstallationID string
	RotationID           string
	CredentialGeneration int64
	Now                  func() time.Time
}

func (o ApplyOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// ApplyLicense validates, stages, and activates a signed
// artifact. A failure before the final state write leaves the active license
// and image credentials untouched.
func (m *Manager) ApplyLicense(ctx context.Context, artifact []byte, options ApplyOptions) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(artifact) == 0 {
		return nil, errors.New("license artifact is required")
	}
	wrapper, err := sdklicense.LoadLicenseFromBytes(artifact)
	if err != nil {
		return nil, errors.Wrap(err, "parse candidate license")
	}
	if err := wrapper.VerifySignature(); err != nil {
		return nil, errors.Wrap(err, "verify candidate license signature")
	}
	expired, err := sdklicense.LicenseIsExpired(wrapper)
	if err != nil || expired {
		return nil, errors.New("candidate license is expired or has an invalid expiration")
	}
	options, err = options.withSignedIdentity(wrapper.GetAppSlug(), wrapper.GetCustomerID())
	if err != nil {
		return nil, err
	}
	installationMetadata := sdklicense.InstallationMetadataFromLicense(wrapper)
	if installationMetadata.Present {
		if options.TargetInstallationID != "" && options.TargetInstallationID != installationMetadata.InstallationID {
			return nil, errors.New("candidate license installation ID does not match request")
		}
		options.TargetInstallationID = installationMetadata.InstallationID
		if options.CredentialGeneration != 0 && options.CredentialGeneration != installationMetadata.CredentialGeneration {
			return nil, errors.New("candidate license credential generation does not match request")
		}
		options.CredentialGeneration = installationMetadata.CredentialGeneration
	} else {
		return nil, errors.New("candidate license has no installation token")
	}
	if options.CustomerID == "" {
		return nil, errors.New("candidate license has no immutable customer identity")
	}

	for attempts := 0; attempts < 5; attempts++ {
		secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "get SDK license state Secret")
		}
		if apierrors.IsNotFound(err) {
			secret = nil
		}
		state := &State{Version: 1, Status: StatusUnbound}
		if err == nil {
			if raw := secret.Data[stateSecretDataKey]; len(raw) == 0 {
				return nil, errors.New("SDK license state Secret is missing state.json")
			} else if err := json.Unmarshal(raw, state); err != nil {
				return nil, errors.Wrap(err, "decode SDK license state")
			}
		}
		if state.ActiveGeneration == wrapper.GetLicenseSequence() && state.ActiveLicenseFingerprint == fingerprint(artifact) {
			// Repeated delivery of the same signed artifact does not change the
			// active generation, but it is the only moment the SDK revisits the
			// pull Secret it owns. Helm no longer renders that Secret, so if it
			// was deleted or edited, this is what puts it back; otherwise the
			// cluster can sit on a missing credential while state reads healthy.
			if _, err := m.stageImagePullSecretUpdate(ctx, state.ActiveImageCredentials, state.ActiveLicense); err != nil {
				return nil, errors.Wrap(err, "reconcile image pull Secret")
			}
			store.GetStore().SetLicense(wrapper)
			return state, nil
		}
		if err := validateCandidate(state, wrapper.GetLicenseSequence(), options); err != nil {
			return nil, err
		}
		if state.InstallationID == "" {
			return nil, errors.New("initial online binding requires a Portal binding grant")
		}
		if len(state.PrivateKey) != ed25519.PrivateKeySize {
			return nil, errors.New("SDK installation key is missing; manual recovery is required")
		}
		establishIdentityAnchors(state, options)

		candidate := *state
		candidate.CandidateLicense = append([]byte(nil), artifact...)
		candidate.CandidateGeneration = wrapper.GetLicenseSequence()
		candidate.CandidateCredentialGeneration = options.CredentialGeneration
		candidate.RotationID = options.RotationID
		candidate.Status = StatusStaging
		candidate.LastError = ""
		// Persist the stage before preparing image credentials. This makes an interrupted apply
		// observable and, critically, leaves Active* unchanged on failure.
		stagedSecret, err := m.writeState(ctx, secret, &candidate)
		if err != nil {
			if apierrors.IsConflict(errors.Cause(err)) {
				continue
			}
			return nil, err
		}
		imageCredentials := candidate.ActiveImageCredentials
		{
			imageCredentials, err = licenseDockerConfig(&candidate, m.registryDomains)
			if err != nil {
				m.recordFailure(ctx, stagedSecret, &candidate, err)
				return nil, errors.Wrap(err, "prepare candidate image credentials")
			}
			if len(imageCredentials) == 0 {
				m.recordFailure(ctx, stagedSecret, &candidate, errors.New("candidate image credentials are empty"))
				return nil, errors.New("candidate image credentials are empty")
			}
		}

		// Keep the staged copy separate: failures must not persist the active
		// fields we are about to prepare for the successor.
		staged := candidate
		now := options.now()
		candidate.ActiveLicense = append([]byte(nil), artifact...)
		candidate.ActiveGeneration = candidate.CandidateGeneration
		candidate.ActiveLicenseFingerprint = fingerprint(artifact)
		candidate.ActiveImageCredentials = append([]byte(nil), imageCredentials...)
		if options.CredentialGeneration != 0 {
			candidate.CredentialGeneration = options.CredentialGeneration
		}
		candidate.CredentialExpiresAt = nil
		candidate.CandidateLicense = nil
		candidate.CandidateGeneration = 0
		candidate.CandidateCredentialGeneration = 0
		if candidate.RotationID == "" {
			candidate.Status = StatusCurrent
		} else {
			candidate.Status = StatusActivated
			candidate.RotationActivatedAt = &now
		}
		candidate.LastSuccessfulExchange = &now
		candidate.LastSynchronization = &now
		candidate.LastError = ""

		// The pull Secret is deliberately updated before active state. If it
		// fails, the durable state remains staged and workloads retain their old
		// credential. A successor is valid during the Portal overlap window, so
		// this ordering avoids an active license that cannot pull an image.
		//
		// The two Secrets cannot be committed atomically. Retain a snapshot so
		// a failed active-state write can restore the previous pull credential.
		pullSecretSnapshot, err := m.stageImagePullSecretUpdate(ctx, candidate.ActiveImageCredentials, candidate.ActiveLicense)
		if err != nil {
			m.recordFailure(ctx, stagedSecret, &staged, err)
			return nil, err
		}
		if _, err := m.writeState(ctx, stagedSecret, &candidate); err != nil {
			// A timeout can arrive after Kubernetes committed the update. Read
			// it back before rolling back credentials or reporting failure.
			observed, readErr := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
			if readErr != nil {
				return nil, errors.New("could not determine whether license activation was saved; retry the same license")
			}
			var observedState State
			if json.Unmarshal(observed.Data[stateSecretDataKey], &observedState) != nil {
				return nil, errors.New("could not read license activation state; manual inspection is required")
			}
			if observedState.ActiveLicenseFingerprint == candidate.ActiveLicenseFingerprint && observedState.CredentialGeneration == candidate.CredentialGeneration {
				store.GetStore().SetLicense(wrapper)
				return &observedState, nil
			}
			if observed.ResourceVersion != stagedSecret.ResourceVersion || !reflect.DeepEqual(observed.Data, stagedSecret.Data) {
				return nil, errors.New("license state changed during activation; retry with the current license")
			}
			rollbackErr := m.restoreImagePullSecret(ctx, pullSecretSnapshot)
			if rollbackErr != nil {
				err = errors.Wrapf(err, "activate SDK license state (also failed to restore managed image pull Secret: %v)", rollbackErr)
			} else {
				err = errors.Wrap(err, "activate SDK license state")
			}
			if apierrors.IsConflict(errors.Cause(err)) {
				continue
			}
			m.recordFailure(ctx, stagedSecret, &staged, err)
			return nil, err
		}
		// All delivery paths update the running SDK only after the active
		// license has been saved. Portal confirmation happens after this returns.
		store.GetStore().SetLicense(wrapper)
		return &candidate, nil
	}
	return nil, errors.New("concurrent SDK license state updates prevented activation")
}

// recordFailure leaves the active generation untouched while making a failed
// candidate visible to Console and Portal. It is deliberately best effort: the
// original error is always returned to the caller even if Kubernetes is
// temporarily unavailable while recording diagnostics.
func (m *Manager) recordFailure(ctx context.Context, existing *corev1.Secret, candidate *State, cause error) {
	failed := *candidate
	failed.Status = StatusReplacementFailed
	if terminal := licenseStatusFromPortalError(cause); terminal != "" {
		failed.Status = terminal
	}
	failed.LastError = "License replacement failed; the previous license remains active."
	_, _ = m.writeState(ctx, existing, &failed)
}

func validateCandidate(state *State, generation int64, options ApplyOptions) error {
	if state.ApplicationID != "" && state.ApplicationID != options.ApplicationID {
		return errors.New("candidate license belongs to a different application")
	}
	if state.CustomerID != "" && state.CustomerID != options.CustomerID {
		return errors.New("candidate license belongs to a different customer")
	}
	if state.InstallationID != "" && options.TargetInstallationID != "" && state.InstallationID != options.TargetInstallationID {
		return errors.New("candidate license belongs to a different installation")
	}
	if state.ActiveGeneration > generation {
		return errors.New("candidate license generation is older than active generation")
	}
	if state.ActiveGeneration == generation && state.ActiveLicenseFingerprint != "" && options.CredentialGeneration <= state.CredentialGeneration {
		return errors.New("candidate license generation is already active")
	}
	if options.CredentialGeneration != 0 && options.CredentialGeneration < state.CredentialGeneration {
		return errors.New("candidate credential generation is older than active generation")
	}
	return nil
}

// Caller-supplied identity is an expectation, never an override for the
// identity in the verified license. Otherwise passing current state here
// would hide a replacement license for a different customer or app.
func (o ApplyOptions) withSignedIdentity(app, customer string) (ApplyOptions, error) {
	if app == "" || customer == "" {
		return o, errors.New("candidate license has no signed customer or application identity")
	}
	if o.ApplicationID != "" && o.ApplicationID != app {
		return o, errors.New("candidate license belongs to a different application")
	}
	if o.CustomerID != "" && o.CustomerID != customer {
		return o, errors.New("candidate license belongs to a different customer")
	}
	o.ApplicationID, o.CustomerID = app, customer
	return o, nil
}

// establishIdentityAnchors records the signed license identity once the
// candidate has passed validateCandidate. PrepareBinding creates the keypair
// before the Portal returns the first artifact, so an initial online bind has
// an InstallationID but intentionally has no anchors yet.
func establishIdentityAnchors(state *State, options ApplyOptions) {
	if state.ApplicationID == "" {
		state.ApplicationID = options.ApplicationID
	}
	if state.CustomerID == "" {
		state.CustomerID = options.CustomerID
	}
}

func (m *Manager) writeState(ctx context.Context, existing *corev1.Secret, state *State) (*corev1.Secret, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, errors.Wrap(err, "encode SDK license state")
	}
	stateSecret := existing
	if stateSecret == nil {
		stateSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: m.stateSecretName, Namespace: m.namespace}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{}}
	} else {
		stateSecret = stateSecret.DeepCopy()
	}
	if stateSecret.Data == nil {
		stateSecret.Data = map[string][]byte{}
	}
	stateSecret.Data[stateSecretDataKey] = raw
	if existing == nil {
		created, err := m.client.CoreV1().Secrets(m.namespace).Create(ctx, stateSecret, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "create SDK license state Secret")
		}
		return created, nil
	}
	updated, err := m.client.CoreV1().Secrets(m.namespace).Update(ctx, stateSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "update SDK license state Secret")
	}
	return updated, nil
}

// pullSecretSnapshot is the complete pre-activation state of one SDK-managed
// image pull Secret. A nil Secret means this attempt created the Secret.
type pullSecretSnapshot struct {
	Secret  *corev1.Secret
	Written *corev1.Secret
}

// stageImagePullSecretUpdate updates only the SDK namespace's pull Secret.
// The snapshot lets activation restore it if saving the active state fails.
func (m *Manager) stageImagePullSecretUpdate(ctx context.Context, credentials, licenseArtifact []byte) (*pullSecretSnapshot, error) {
	if len(credentials) == 0 && len(licenseArtifact) == 0 {
		return nil, nil
	}
	snapshot, err := m.imagePullSecretSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	written, err := m.writeImagePullSecret(ctx, snapshot, credentials, licenseArtifact)
	if err != nil {
		return nil, err
	}
	snapshot.Written = written
	return snapshot, nil
}

func (m *Manager) imagePullSecretSnapshot(ctx context.Context) (*pullSecretSnapshot, error) {
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.imagePullSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return &pullSecretSnapshot{}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "read SDK-managed image pull Secret")
	}
	return &pullSecretSnapshot{Secret: secret.DeepCopy()}, nil
}

func (m *Manager) restoreImagePullSecret(ctx context.Context, snapshot *pullSecretSnapshot) error {
	if snapshot == nil {
		return nil
	}
	current, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.imagePullSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Do not recreate something another writer deleted.
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "read managed image pull Secret for rollback")
	}
	if snapshot.Written == nil || current.UID != snapshot.Written.UID || current.ResourceVersion != snapshot.Written.ResourceVersion || !reflect.DeepEqual(current.Data, snapshot.Written.Data) {
		return errors.New("managed image pull Secret changed after activation attempt; refusing to overwrite it")
	}
	if snapshot.Secret == nil {
		err = m.client.CoreV1().Secrets(m.namespace).Delete(ctx, current.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{UID: &current.UID, ResourceVersion: &current.ResourceVersion},
		})
	} else {
		// Only restore the exact object written by this attempt.
		restored := snapshot.Secret.DeepCopy()
		restored.ResourceVersion = current.ResourceVersion
		_, err = m.client.CoreV1().Secrets(m.namespace).Update(ctx, restored, metav1.UpdateOptions{})
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "restore SDK-managed image pull Secret")
	}
	return nil
}

func (m *Manager) writeImagePullSecret(ctx context.Context, snapshot *pullSecretSnapshot, credentials, licenseArtifact []byte) (*corev1.Secret, error) {
	data := map[string][]byte{}
	if len(credentials) > 0 {
		data[corev1.DockerConfigJsonKey] = credentials
	}
	if len(licenseArtifact) > 0 {
		wrapper, err := sdklicense.LoadLicenseFromBytes(licenseArtifact)
		if err != nil {
			return nil, errors.Wrap(err, "parse active license for pull Secret")
		}
		metadata := sdklicense.InstallationMetadataFromLicense(wrapper)
		if metadata.Present {
			// Publish the signed license, never a locally rewritten copy. EC can
			// use this for joins without reading the private installation key.
			data[InstallationTokenDataKey] = []byte(wrapper.GetLicenseID())
			data[ActiveLicenseDataKey] = append([]byte(nil), licenseArtifact...)
		}
	}
	var written *corev1.Secret
	var err error
	if snapshot.Secret == nil {
		if len(data[corev1.DockerConfigJsonKey]) == 0 {
			data[corev1.DockerConfigJsonKey] = []byte(`{"auths":{}}`)
		}
		written, err = m.client.CoreV1().Secrets(m.namespace).Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: m.imagePullSecretName, Namespace: m.namespace}, Type: corev1.SecretTypeDockerConfigJson, Data: data}, metav1.CreateOptions{})
	} else {
		// Use the version that was snapshotted, not a second GET that could
		// hide an intervening update and leave rollback with stale contents.
		pullSecret := snapshot.Secret.DeepCopy()
		pullSecret.Type = corev1.SecretTypeDockerConfigJson
		if pullSecret.Data == nil {
			pullSecret.Data = map[string][]byte{}
		}
		for key, value := range data {
			pullSecret.Data[key] = value
		}
		written, err = m.client.CoreV1().Secrets(m.namespace).Update(ctx, pullSecret, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, errors.Wrapf(err, "update SDK-managed image pull Secret in namespace %s", m.namespace)
	}
	return written, nil
}
