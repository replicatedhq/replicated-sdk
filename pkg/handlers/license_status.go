package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

var sdkAPIEndpoint string

// ConfigureSDKAPIEndpoint supplies the endpoint used by the explicit
// retry action. It is process configuration, not mutable license state.
func ConfigureSDKAPIEndpoint(endpoint string) {
	sdkAPIEndpoint = strings.TrimRight(endpoint, "/")
}

// LicenseStatus is the non-secret projection consumed by EC and
// Helm-oriented tooling. Neither license bytes, private keys, nor image
// credentials can escape through this endpoint.
type LicenseStatus struct {
	Status                 licensestate.Status `json:"status"`
	InstallationID         string              `json:"installationID,omitempty"`
	ApplicationID          string              `json:"applicationID,omitempty"`
	CustomerID             string              `json:"customerID,omitempty"`
	ActiveGeneration       int64               `json:"activeGeneration,omitempty"`
	ActiveFingerprint      string              `json:"activeFingerprint,omitempty"`
	CredentialGeneration   int64               `json:"credentialGeneration,omitempty"`
	CredentialExpiresAt    *time.Time          `json:"credentialExpiresAt,omitempty"`
	LastSuccessfulExchange *time.Time          `json:"lastSuccessfulExchange,omitempty"`
	LastSynchronization    *time.Time          `json:"lastSynchronization,omitempty"`
	LastError              string              `json:"lastError,omitempty"`
}

func GetLicenseStatus(w http.ResponseWriter, r *http.Request) {
	manager := licensestate.Current()
	if manager == nil {
		JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "license manager is not initialized"})
		return
	}
	state, err := manager.LoadOnlineState(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load license status"})
		return
	}
	JSON(w, http.StatusOK, LicenseStatus{
		Status:                 state.Status,
		InstallationID:         state.InstallationID,
		ApplicationID:          state.ApplicationID,
		CustomerID:             state.CustomerID,
		ActiveGeneration:       state.ActiveGeneration,
		ActiveFingerprint:      state.ActiveLicenseFingerprint,
		CredentialGeneration:   state.CredentialGeneration,
		CredentialExpiresAt:    state.CredentialExpiresAt,
		LastSuccessfulExchange: state.LastSuccessfulExchange,
		LastSynchronization:    state.LastSynchronization,
		LastError:              state.LastError,
	})
}

// SyncLicense lets Console or Helm tooling request an
// immediate retry of the same installation-key rotation/credential refresh
// loop that runs periodically. It never accepts a license or writes state
// itself.
func SyncLicense(w http.ResponseWriter, r *http.Request) {
	if util.IsAirgap() {
		JSON(w, http.StatusConflict, map[string]string{"error": "airgap licenses must be replaced by uploading a license file"})
		return
	}
	manager := licensestate.Current()
	if manager == nil || sdkAPIEndpoint == "" {
		JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "license synchronization is not configured"})
		return
	}
	state, _, err := manager.SynchronizePortalRotation(r.Context(), sdkAPIEndpoint)
	if err != nil {
		JSON(w, http.StatusBadGateway, map[string]string{"error": "license synchronization failed"})
		return
	}
	JSON(w, http.StatusOK, LicenseStatus{
		Status: state.Status, InstallationID: state.InstallationID, ApplicationID: state.ApplicationID, CustomerID: state.CustomerID,
		ActiveGeneration: state.ActiveGeneration, ActiveFingerprint: state.ActiveLicenseFingerprint, CredentialGeneration: state.CredentialGeneration,
		CredentialExpiresAt: state.CredentialExpiresAt, LastSuccessfulExchange: state.LastSuccessfulExchange, LastSynchronization: state.LastSynchronization, LastError: state.LastError,
	})
}
