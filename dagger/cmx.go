package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
)

type DistributionVersion struct {
	Distribution string
	Version      string
}

func listCMXDistributionsAndVersions(
	ctx context.Context,
	opServiceAccount *dagger.Secret,
) ([]DistributionVersion, error) {
	replicatedServiceAccount := mustGetSecret(ctx, opServiceAccount, "Replicated", "service_account", VaultDeveloperAutomation)

	ctr := dag.Container().From("replicated/vendor-cli:latest").
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedServiceAccount).
		WithExec([]string{"/replicated", "cluster", "versions", "--output", "json"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return nil, err
	}

	type ReplicatedClusterVersion struct {
		ShortName     string   `json:"short_name"`
		Versions      []string `json:"versions"`
		InstanceTypes []string `json:"instance_types"`
		NodesMax      int      `json:"nodes_max"`
	}
	replicatedClusterVersions := []ReplicatedClusterVersion{}
	if err := json.Unmarshal([]byte(out), &replicatedClusterVersions); err != nil {
		return nil, err
	}

	versionsToInclude := map[string][]string{
		"gke": {},
		"eks": {},
		// "openshift": {},
		// "oke":       {},
	}

	for includedDistribution := range versionsToInclude {
		for _, clusterVersion := range replicatedClusterVersions {
			if clusterVersion.ShortName == includedDistribution {
				versionsToInclude[includedDistribution] = clusterVersion.Versions
			}
		}
	}

	// CMX has a several patterns to list versions,
	// we need to handle distributions separately for each distribution

	result := []DistributionVersion{}
	for distribution, versions := range versionsToInclude {
		switch distribution {
		// TODO add k3s in and handle their patch releases in CMX
		default:
			result = append(result, DistributionVersion{
				Distribution: distribution,
				Version:      versions[len(versions)-1],
			})
		}
	}
	return result, nil
}
