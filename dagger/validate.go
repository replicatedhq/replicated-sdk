package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"sync"
)

// Validate runs the complete validation pipeline or specific components
// This is the primary entry point for all CI/CD operations
func (m *ReplicatedSdk) Validate(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,
) error {
	if err := testUnit(ctx, source); err != nil {
		return err
	}

	if err := testPact(ctx, source, opServiceAccount); err != nil {
		return err
	}

	imageRegistry, imageRepository, imageTag, err := buildAndPushImageToTTL(ctx, source)
	if err != nil {
		return err
	}
	fmt.Printf("Image pushed to %s/%s:%s\n", imageRegistry, imageRepository, imageTag)

	chart, err := buildAndPushChartToTTL(ctx, source, imageRegistry, imageRepository, imageTag)
	if err != nil {
		return err
	}
	fmt.Printf("Chart pushed to %s\n", chart)

	channelSlug, err := createAppTestRelease(ctx, source, opServiceAccount, chart)
	if err != nil {
		return err
	}

	customerID, licenseID, err := createCustomer(ctx, channelSlug, opServiceAccount)
	if err != nil {
		return err
	}
	fmt.Println(customerID, licenseID)

	cmxDistributions, err := listCMXDistributionsAndVersions(ctx, opServiceAccount)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, distribution := range cmxDistributions {
		wg.Add(1)
		go func(distribution DistributionVersion) {
			defer wg.Done()
			if err := e2e(ctx, source, opServiceAccount, licenseID, distribution.Distribution, distribution.Version, channelSlug); err != nil {
				panic(fmt.Sprintf("E2E test failed for distribution %s %s: %v", distribution.Distribution, distribution.Version, err))
			}
		}(distribution)
	}
	wg.Wait()

	return nil
}
