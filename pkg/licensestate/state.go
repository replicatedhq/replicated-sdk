// Package licensestate owns the mutable license state for one SDK
// installation. Helm only supplies its stable Secret names; it never renders
// the contents of either Secret.
package licensestate

import (
	"time"
)

const (
	DefaultStateSecretName     = "replicated-sdk-state"
	DefaultImagePullSecretName = "enterprise-pull-secret"
	InstallationTokenDataKey   = "installation-token"
	ActiveLicenseDataKey       = "license.yaml"
	stateSecretDataKey         = "state.json"
)

// Status is intentionally coarse-grained. Detailed error text is kept only in
// the SDK-owned Secret and must not be exposed through charts or manifests.
type Status string

const (
	StatusUnbound                   Status = "Unbound"
	StatusCurrent                   Status = "Current"
	StatusRotationAvailable         Status = "RotationAvailable"
	StatusStaging                   Status = "Staging"
	StatusActivated                 Status = "Activated"
	StatusConfirmed                 Status = "Confirmed"
	StatusManualReplacementRequired Status = "ManualReplacementRequired"
	StatusManualRecoveryRequired    Status = "ManualRecoveryRequired"
	StatusReplacementFailed         Status = "ReplacementFailed"
	StatusSuspended                 Status = "Suspended"
	StatusExpired                   Status = "Expired"
	StatusTerminated                Status = "Terminated"
	StatusRevoked                   Status = "Revoked"
)

// State is serialized as a single Secret value, so an active generation and
// its image credentials change together. PrivateKey is never returned by SDK
// status APIs or copied into a support bundle.
type State struct {
	Version uint8 `json:"version"`

	InstallationID string `json:"installationID,omitempty"`
	ApplicationID  string `json:"applicationID,omitempty"`
	CustomerID     string `json:"customerID,omitempty"`
	PrivateKey     []byte `json:"privateKey,omitempty"`
	PublicKey      []byte `json:"publicKey,omitempty"`

	ActiveLicense            []byte     `json:"activeLicense,omitempty"`
	ActiveGeneration         int64      `json:"activeGeneration,omitempty"`
	ActiveLicenseFingerprint string     `json:"activeLicenseFingerprint,omitempty"`
	ActiveImageCredentials   []byte     `json:"activeImageCredentials,omitempty"`
	CredentialGeneration     int64      `json:"credentialGeneration,omitempty"`
	CredentialExpiresAt      *time.Time `json:"credentialExpiresAt,omitempty"`

	CandidateLicense    []byte `json:"candidateLicense,omitempty"`
	CandidateGeneration int64  `json:"candidateGeneration,omitempty"`
	// CandidateCredentialGeneration tracks the pending token generation until
	// its license becomes active.
	CandidateCredentialGeneration int64      `json:"candidateCredentialGeneration,omitempty"`
	RotationID                    string     `json:"rotationID,omitempty"`
	RotationActivatedAt           *time.Time `json:"rotationActivatedAt,omitempty"`

	Status                 Status     `json:"status"`
	LastSuccessfulExchange *time.Time `json:"lastSuccessfulExchange,omitempty"`
	LastSynchronization    *time.Time `json:"lastSynchronization,omitempty"`
	LastError              string     `json:"lastError,omitempty"`
}
