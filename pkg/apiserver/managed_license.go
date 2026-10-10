package apiserver

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
	"k8s.io/client-go/kubernetes"
)

// Everything in this file applies only to installations that opted in to the
// SDK-managed license, which the chart signals by setting stateSecretName.
// Every function returns immediately for any other installation, so the legacy
// license flow in bootstrap.go behaves exactly as it did before this feature.

// configureManagedLicense wires up the manager for SDK-managed installations.
// It returns nil for a legacy installation, which leaves licensestate.Current()
// nil and every managed-license code path switched off.
func configureManagedLicense(clientset kubernetes.Interface, params APIServerParams) *licensestate.Manager {
	if params.StateSecretName == "" {
		licensestate.Configure(nil)
		return nil
	}
	manager := licensestate.NewManager(clientset, params.Namespace, params.StateSecretName, params.ImagePullSecretName)
	manager.SetRegistryDomains(params.RegistryDomains)
	// ApplyLicense updates the in-memory license itself, so a rotation does not
	// leave this process serving the credential it replaced.
	licensestate.Configure(manager)
	return manager
}

// managedLicenseBytes returns the license an SDK-managed installation should
// run with, binding to Portal first when this is its first start. It returns
// nil for a legacy installation, so the caller keeps the license Helm rendered.
func managedLicenseBytes(manager *licensestate.Manager, params APIServerParams) ([]byte, error) {
	if manager == nil {
		return nil, nil
	}
	state, err := manager.LoadOnlineState(params.Context)
	if err != nil {
		return nil, errors.Wrap(err, "load SDK-owned license state")
	}
	if needsInitialLicense(state) && params.BootstrapSecretName != "" {
		state, err = manager.BindFromBootstrapSecret(params.Context, params.ReplicatedAppEndpoint, params.BootstrapSecretName)
		if err != nil {
			return nil, errors.Wrap(err, "bind SDK from bootstrap Secret")
		}
	}
	return state.ActiveLicense, nil
}

// A process can stop after staging its first license but before activating
// it. Retry the saved bootstrap operation; "Staging" is not a bound install.
func needsInitialLicense(state *licensestate.State) bool {
	return len(state.ActiveLicense) == 0 && state.Status != licensestate.StatusManualRecoveryRequired
}

// persistManagedLicense records the verified license as the active generation.
// Legacy installations are never enrolled: manager is nil for them.
func persistManagedLicense(manager *licensestate.Manager, params APIServerParams, artifact []byte, appSlug, customerID string) error {
	if manager == nil || len(artifact) == 0 {
		return nil
	}
	if _, err := manager.ApplyLicense(params.Context, artifact, licensestate.ApplyOptions{
		ApplicationID: appSlug,
		CustomerID:    customerID,
	}); err != nil {
		return errors.Wrap(err, "persist verified license in SDK-owned state")
	}
	return nil
}
