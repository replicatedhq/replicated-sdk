package licensestate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultBootstrapSecretName = "replicated-sdk-bootstrap"

type portalArtifactResponse struct {
	License              string `json:"license"`
	InstallationID       string `json:"installationId"`
	RotationID           string `json:"rotationId"`
	CredentialGeneration int64  `json:"credentialGeneration"`
}

type PortalClient struct {
	Endpoint   string
	HTTPClient *http.Client
}

// PortalHTTPError preserves the non-secret license error returned by
// Portal. The SDK uses it only to report actionable local state; it never
// includes a license or credential in the error.
type PortalHTTPError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *PortalHTTPError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("Portal returned HTTP %d (%s)", e.StatusCode, e.Status)
	}
	if e.Message != "" {
		return fmt.Sprintf("Portal returned HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("Portal returned HTTP %d", e.StatusCode)
}

func NewPortalClient(endpoint string) *PortalClient {
	return &PortalClient{Endpoint: strings.TrimRight(endpoint, "/")}
}

func (c *PortalClient) post(ctx context.Context, path string, request interface{}, response interface{}, signingState ...*State) (int, error) {
	if c == nil || c.Endpoint == "" {
		return 0, errors.New("Portal endpoint is not configured")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return 0, errors.Wrap(err, "marshal Portal request")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/sdk"+path, bytes.NewReader(body))
	if err != nil {
		return 0, errors.Wrap(err, "create Portal request")
	}
	req.Header.Set("Content-Type", "application/json")
	if len(signingState) > 0 {
		state := signingState[0]
		if state == nil || len(state.PrivateKey) != ed25519.PrivateKeySize {
			return 0, errors.New("SDK key is missing; manual recovery is required")
		}
		license, err := sdklicense.LoadLicenseFromBytes(state.ActiveLicense)
		if err != nil {
			return 0, errors.New("active installation license is invalid")
		}
		encoded := license.GetLicenseID()
		token, err := installationtoken.Parse(encoded)
		if err != nil || token.InstallationID != state.InstallationID || token.Generation != state.CredentialGeneration {
			return 0, errors.New("active installation credential does not match SDK state")
		}
		proof, err := installationtoken.Sign(ed25519.PrivateKey(state.PrivateKey), encoded, req.Method, req.URL.EscapedPath(), body, time.Now())
		if err != nil {
			return 0, errors.Wrap(err, "sign installation request")
		}
		req.Header.Set("Authorization", "Bearer "+encoded)
		req.Header.Set("X-Replicated-Installation-Timestamp", strconv.FormatInt(proof.Timestamp, 10))
		req.Header.Set("X-Replicated-Installation-Nonce", proof.Nonce)
		req.Header.Set("X-Replicated-Installation-Signature", proof.Signature)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "call Portal")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return resp.StatusCode, errors.Wrap(err, "read Portal response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		portalError := struct {
			Status  string `json:"status"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}{}
		_ = json.Unmarshal(payload, &portalError)
		message := portalError.Message
		if message == "" {
			message = portalError.Error
		}
		return resp.StatusCode, &PortalHTTPError{StatusCode: resp.StatusCode, Status: portalError.Status, Message: message}
	}
	if err := json.Unmarshal(payload, response); err != nil {
		return resp.StatusCode, errors.Wrap(err, "decode Portal response")
	}
	return resp.StatusCode, nil
}

func licenseStatusFromPortalError(err error) Status {
	var portalErr *PortalHTTPError
	if !errors.As(err, &portalErr) {
		return ""
	}
	switch strings.ToLower(portalErr.Status) {
	case "suspended":
		return StatusSuspended
	case "expired":
		return StatusExpired
	case "terminated":
		return StatusTerminated
	case "revoked":
		return StatusRevoked
	}
	switch portalErr.StatusCode {
	case http.StatusUnauthorized:
		return StatusRevoked
	case http.StatusForbidden:
		return StatusTerminated
	default:
		return ""
	}
}

func (m *Manager) BindFromBootstrapSecret(ctx context.Context, endpoint, bootstrapSecretName string) (*State, error) {
	if bootstrapSecretName == "" {
		bootstrapSecretName = DefaultBootstrapSecretName
	}
	state, err := m.LoadOnlineState(ctx)
	if err != nil {
		return nil, err
	}
	if state.Status == StatusManualRecoveryRequired {
		return nil, errors.New(state.LastError)
	}
	if state.Status != StatusUnbound && len(state.ActiveLicense) > 0 {
		if len(state.PrivateKey) != ed25519.PrivateKeySize {
			return nil, errors.New("SDK installation key is missing; manual recovery is required")
		}
		return state, nil
	}
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, bootstrapSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get SDK bootstrap Secret")
	}
	grant := strings.TrimSpace(string(secret.Data["binding-grant"]))
	if grant == "" {
		return nil, errors.New("SDK bootstrap Secret is missing binding-grant")
	}
	initial, err := sdklicense.LoadLicenseFromBytes(secret.Data[ActiveLicenseDataKey])
	if err != nil {
		return nil, errors.New("SDK bootstrap Secret is missing a valid license")
	}
	if err := initial.VerifySignature(); err != nil {
		return nil, errors.New("bootstrap license signature is invalid")
	}
	token, err := installationtoken.Parse(initial.GetLicenseID())
	if err != nil {
		return nil, errors.New("bootstrap license has no installation token")
	}
	state, err = m.PrepareBinding(ctx, token.InstallationID)
	if err != nil {
		return nil, err
	}
	response := portalArtifactResponse{}
	_, err = NewPortalClient(endpoint).post(ctx, "/bind", map[string]string{
		"grant":          grant,
		"installationId": state.InstallationID,
		"publicKey":      base64.StdEncoding.EncodeToString(state.PublicKey),
	}, &response)
	if err != nil {
		return nil, errors.Wrap(err, "consume SDK binding grant")
	}
	if response.InstallationID != state.InstallationID {
		return nil, errors.New("Portal binding response is for a different installation")
	}
	artifact, err := base64.StdEncoding.DecodeString(response.License)
	if err != nil || len(artifact) == 0 {
		return nil, errors.New("Portal binding response has no valid license artifact")
	}
	state, err = m.ApplyLicense(ctx, artifact, ApplyOptions{
		TargetInstallationID: state.InstallationID,
		CredentialGeneration: response.CredentialGeneration,
	})
	if err != nil {
		return nil, errors.Wrap(err, "apply initial SDK license")
	}
	if err := m.client.CoreV1().Secrets(m.namespace).Delete(ctx, bootstrapSecretName, metav1.DeleteOptions{}); err != nil {
		return nil, errors.Wrap(err, "delete consumed SDK bootstrap Secret")
	}
	return state, nil
}

// SynchronizePortalRotation uses the active token and SDK key for each
// request. A retry after local activation confirms the saved successor first.
// SynchronizePortalRotation records when the installation last managed to ask
// Portal whether anything changed. That is a different fact from when its
// license last changed, and it is the one that tells someone reading the status
// whether they are looking at current information.
func (m *Manager) SynchronizePortalRotation(ctx context.Context, endpoint string) (*State, bool, error) {
	state, applied, err := m.synchronizePortalRotation(ctx, endpoint)
	if err != nil {
		return state, applied, err
	}
	if synchronized, markErr := m.markSynchronized(ctx); markErr != nil {
		// The check itself succeeded; failing to record when is not worth
		// turning into a failed synchronization.
		logger.Infof("could not record SDK synchronization time: %v", markErr)
	} else if synchronized != nil && state != nil {
		// Only the timestamp carries over. Some statuses are derived from the
		// live state rather than stored, so replacing the whole struct here
		// would report a lost identity as healthy.
		state.LastSynchronization = synchronized.LastSynchronization
	}
	return state, applied, nil
}

// markSynchronized stamps the check-in time without touching anything else, so
// it cannot disturb a rotation that just completed.
func (m *Manager) markSynchronized(ctx context.Context) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get SDK state to record synchronization")
	}
	current := &State{}
	if err := json.Unmarshal(secret.Data[stateSecretDataKey], current); err != nil {
		return nil, errors.Wrap(err, "decode SDK state to record synchronization")
	}
	now := time.Now().UTC()
	current.LastSynchronization = &now
	if _, err := m.writeState(ctx, secret, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (m *Manager) synchronizePortalRotation(ctx context.Context, endpoint string) (*State, bool, error) {
	state, err := m.LoadOnlineState(ctx)
	if err != nil {
		return nil, false, err
	}
	if state.Status == StatusUnbound || state.Status == StatusManualRecoveryRequired || state.InstallationID == "" {
		return state, false, nil
	}
	if state.Status == StatusActivated && state.RotationID != "" {
		acknowledged, err := m.acknowledgePortalRotation(ctx, endpoint, state)
		if err != nil {
			return state, false, err
		}
		if acknowledged {
			state.Status = StatusConfirmed
			return state, true, nil
		}
		// Portal has already closed this rotation, so it will never accept the
		// acknowledgement. Fall through to the offer rather than retrying the
		// same rejected call forever, which would hide every later rotation.
	}
	portal := NewPortalClient(endpoint)
	offer := portalArtifactResponse{}
	status, err := portal.post(ctx, "/rotation-offer", map[string]string{"installationId": state.InstallationID}, &offer, state)
	if err != nil {
		if status == http.StatusGone || status == http.StatusUnauthorized {
			if err := m.markManualReplacementRequired(ctx); err != nil {
				return nil, false, err
			}
			state.Status = StatusManualReplacementRequired
			return state, false, nil
		}
		return state, false, errors.Wrap(err, "get Portal rotation offer")
	}
	if status == http.StatusNoContent {
		if state.Status == StatusManualReplacementRequired {
			// A later authenticated check-in proves the saved credential still
			// works. Do not keep showing a stale recovery warning.
			if err := m.saveSynchronizationStatus(ctx, state, StatusCurrent); err != nil {
				return state, false, err
			}
			state.Status, state.LastError = StatusCurrent, ""
		}
		return state, false, nil
	}
	if offer.InstallationID != state.InstallationID || offer.RotationID == "" || offer.CredentialGeneration < state.CredentialGeneration {
		return state, false, errors.New("Portal returned an invalid rotation offer")
	}
	artifact, err := base64.StdEncoding.DecodeString(offer.License)
	if err != nil || len(artifact) == 0 {
		return state, false, errors.New("Portal rotation offer has no valid license artifact")
	}
	if offer.CredentialGeneration == state.CredentialGeneration {
		// Manual upload saves the same normal license but has no rotation ID.
		// A check-in supplies that ID, then confirms the credential already
		// saved locally. Do not apply the license a second time.
		state, err = m.attachManuallyAppliedRotation(ctx, state, offer.RotationID, artifact)
		if err != nil {
			return state, false, err
		}
		if acknowledged, err := m.acknowledgePortalRotation(ctx, endpoint, state); err != nil {
			return state, false, err
		} else if acknowledged {
			state.Status = StatusConfirmed
		}
		return state, true, nil
	}
	_ = m.reportPortalRotationStatus(ctx, endpoint, offer.RotationID, ServiceAccountRotationStatusStaging)
	state, err = m.ApplyLicense(ctx, artifact, ApplyOptions{
		ApplicationID: state.ApplicationID, CustomerID: state.CustomerID,
		TargetInstallationID: state.InstallationID, RotationID: offer.RotationID,
		CredentialGeneration: offer.CredentialGeneration,
	})
	if err != nil {
		_ = m.reportPortalRotationStatus(ctx, endpoint, offer.RotationID, ServiceAccountRotationStatusFailed)
		return nil, false, errors.Wrap(err, "apply Portal rotation offer")
	}
	_ = m.reportPortalRotationStatus(ctx, endpoint, offer.RotationID, ServiceAccountRotationStatusActivated)
	if acknowledged, err := m.acknowledgePortalRotation(ctx, endpoint, state); err != nil {
		return state, false, err
	} else if acknowledged {
		state.Status = StatusConfirmed
	}
	return state, true, nil
}

func (m *Manager) attachManuallyAppliedRotation(ctx context.Context, state *State, rotationID string, offered []byte) (*State, error) {
	license, err := sdklicense.LoadLicenseFromBytes(offered)
	if err != nil || license.VerifySignature() != nil {
		return state, errors.New("Portal returned an invalid replacement license")
	}
	active, err := sdklicense.LoadLicenseFromBytes(state.ActiveLicense)
	if err != nil || license.GetLicenseID() != active.GetLicenseID() || license.GetCustomerID() != state.CustomerID || license.GetAppSlug() != state.ApplicationID {
		return state, errors.New("Portal rotation does not match the saved installation credential")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
	if err != nil {
		return state, errors.Wrap(err, "load manual replacement before confirmation")
	}
	current := &State{}
	if err := json.Unmarshal(secret.Data[stateSecretDataKey], current); err != nil {
		return state, errors.Wrap(err, "decode manual replacement state")
	}
	if current.ActiveLicenseFingerprint != state.ActiveLicenseFingerprint || current.CredentialGeneration != state.CredentialGeneration || (current.RotationID != "" && current.RotationID != rotationID) {
		return state, errors.New("SDK state changed before manual replacement confirmation")
	}
	current.RotationID, current.Status = rotationID, StatusActivated
	if _, err := m.writeState(ctx, secret, current); err != nil {
		return state, err
	}
	return current, nil
}

// acknowledgePortalRotation confirms an activated successor. It reports false
// when Portal has already closed the rotation: the local credential is still
// the right one, there is simply nothing left to acknowledge.
func (m *Manager) acknowledgePortalRotation(ctx context.Context, endpoint string, state *State) (bool, error) {
	status, err := NewPortalClient(endpoint).post(ctx, "/rotation-acknowledgement", map[string]interface{}{
		"installationId": state.InstallationID, "rotationId": state.RotationID,
		"credentialGeneration": state.CredentialGeneration, "licenseFingerprint": state.ActiveLicenseFingerprint,
	}, &struct{}{}, state)
	if err != nil {
		if status == http.StatusGone || status == http.StatusNotFound || status == http.StatusConflict {
			return false, nil
		}
		return false, errors.Wrap(err, "confirm Portal rotation")
	}
	return true, m.saveSynchronizationStatus(ctx, state, StatusConfirmed)
}

const (
	ServiceAccountRotationStatusStaging   = "staging"
	ServiceAccountRotationStatusActivated = "activated"
	ServiceAccountRotationStatusFailed    = "failed"
)

// Progress reports carry no license or secret in the body. The same
// authenticated request covers the rotation ID and reported status.
func (m *Manager) reportPortalRotationStatus(ctx context.Context, endpoint, rotationID, rotationStatus string) error {
	if endpoint == "" || rotationID == "" || rotationStatus == "" {
		return errors.New("Portal rotation status requires endpoint, rotation ID, and status")
	}
	state, err := m.Load(ctx)
	if err != nil {
		return err
	}
	_, err = NewPortalClient(endpoint).post(ctx, "/rotation-status", map[string]string{
		"installationId": state.InstallationID, "rotationId": rotationID, "status": rotationStatus,
	}, &struct{}{}, state)
	return err
}

func (m *Manager) markManualReplacementRequired(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "get SDK state for manual replacement status")
	}
	current := &State{}
	if err := json.Unmarshal(secret.Data[stateSecretDataKey], current); err != nil {
		return errors.Wrap(err, "decode SDK state for manual replacement status")
	}
	if current.Status == StatusUnbound {
		return nil
	}
	current.Status = StatusManualReplacementRequired
	current.LastError = "automatic credential rotation window expired; apply the current replacement license"
	_, err = m.writeState(ctx, secret, current)
	return err
}

func (m *Manager) saveSynchronizationStatus(ctx context.Context, state *State, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, err := m.client.CoreV1().Secrets(m.namespace).Get(ctx, m.stateSecretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "get SDK state for rotation confirmation")
	}
	current := &State{}
	if err := json.Unmarshal(secret.Data[stateSecretDataKey], current); err != nil {
		return errors.Wrap(err, "decode SDK state for rotation confirmation")
	}
	if current.RotationID != state.RotationID || current.ActiveLicenseFingerprint != state.ActiveLicenseFingerprint {
		return errors.New("SDK state changed before rotation acknowledgement completed")
	}
	current.Status, current.LastError = status, ""
	_, err = m.writeState(ctx, secret, current)
	return err
}
