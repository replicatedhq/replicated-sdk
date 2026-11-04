package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
)

const maxLicenseArtifactBytes = 4 << 20

// ApplyLicenseRequest is deliberately artifact-only. EC and Helm-oriented
// tooling may deliver a signed replacement, but neither may submit image
// credentials, a mutable license ID, or identity values that could reassign
// the installation. The credential is already inside the signed license.
type ApplyLicenseRequest struct {
	License string `json:"license"`
}

// ApplyLicenseResponse is a non-secret status projection. It allows every
// manual delivery surface to use one operation without learning any SDK-owned
// state contents.
type ApplyLicenseResponse struct {
	Status            licensestate.Status `json:"status"`
	ActiveGeneration  int64               `json:"activeGeneration"`
	ActiveFingerprint string              `json:"activeFingerprint"`
}

// ApplyLicense applies a signed successor through the one SDK-owned
// transaction. Connected initial binding intentionally happens through the
// Portal binding-grant protocol, which registers the SDK-generated public key
// before the first license is activated. An airgap installation has no Portal
// connection at bootstrap: it saves the installation ID from the signed
// license without enrolling a key or calling Portal.
func ApplyLicense(w http.ResponseWriter, r *http.Request) {
	manager := licensestate.Current()
	if manager == nil {
		JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "license manager is not initialized"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLicenseArtifactBytes))
	if err != nil {
		JSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "license artifact is too large"})
		return
	}
	request := ApplyLicenseRequest{}
	if err := json.Unmarshal(body, &request); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid license application request"})
		return
	}
	artifact, err := base64.StdEncoding.DecodeString(strings.TrimSpace(request.License))
	if err != nil || len(artifact) == 0 {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "license must be a base64-encoded signed artifact"})
		return
	}

	state, err := manager.LoadOnlineState(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load license state"})
		return
	}
	// A lost installation key cannot be recovered by uploading a license: the
	// installation could pull images again but never rotate, and Portal would
	// still hold a key nothing can prove. That is an installation failure, and
	// it is reported as one rather than half-repaired here.
	if state.Status == licensestate.StatusManualRecoveryRequired {
		JSON(w, http.StatusConflict, map[string]string{"error": state.LastError})
		return
	}
	if state.Status == licensestate.StatusUnbound || state.InstallationID == "" {
		JSON(w, http.StatusConflict, map[string]string{"error": "initial binding requires a Portal binding grant"})
		return
	}

	state, err = manager.ApplyLicense(r.Context(), artifact, licensestate.ApplyOptions{
		ApplicationID:        state.ApplicationID,
		CustomerID:           state.CustomerID,
		TargetInstallationID: state.InstallationID,
	})
	if err != nil {
		JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "license replacement was rejected"})
		return
	}
	JSON(w, http.StatusOK, ApplyLicenseResponse{
		Status:            state.Status,
		ActiveGeneration:  state.ActiveGeneration,
		ActiveFingerprint: state.ActiveLicenseFingerprint,
	})
}
