package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
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
	sdkImage := fmt.Sprintf("%s/%s:%s", imageRegistry, imageRepository, imageTag)
	fmt.Printf("Image pushed to %s\n", sdkImage)

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

	// Resolve app ID for replicated-sdk-e2e (used by vendor API checks)
	appID, err := getAppID(ctx, opServiceAccount, "replicated-sdk-e2e")
	if err != nil {
		return err
	}

	cmxDistributions, err := listCMXDistributionsAndVersions(ctx, opServiceAccount)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, distribution := range cmxDistributions {
		wg.Add(1)
		go func(distribution DistributionVersion) {
			defer wg.Done()
			if err := e2e(ctx, source, opServiceAccount, appID, customerID, sdkImage, licenseID, distribution.Distribution, distribution.Version, channelSlug); err != nil {
				panic(fmt.Sprintf("E2E test failed for distribution %s %s: %v", distribution.Distribution, distribution.Version, err))
			}
		}(distribution)
	}
	wg.Wait()

	return nil
}

// BuildInfo contains all the information needed to run e2e tests
type BuildInfo struct {
	ImageRegistry   string                 `json:"imageRegistry"`
	ImageRepository string                 `json:"imageRepository"`
	ImageTag        string                 `json:"imageTag"`
	Chart           string                 `json:"chart"`
	ChannelSlug     string                 `json:"channelSlug"`
	CustomerID      string                 `json:"customerID"`
	LicenseID       string                 `json:"licenseID"`
	AppID           string                 `json:"appID"`
	Distributions   []DistributionVersion  `json:"distributions"`
}

// BuildForE2E runs unit tests, pact tests, builds image and chart, creates test release and customer
// Returns a file containing JSON with all the information needed to run e2e tests
func (m *ReplicatedSdk) BuildForE2E(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,
) (*dagger.File, error) {
	if err := testUnit(ctx, source); err != nil {
		return nil, err
	}

	if err := testPact(ctx, source, opServiceAccount); err != nil {
		return nil, err
	}

	imageRegistry, imageRepository, imageTag, err := buildAndPushImageToTTL(ctx, source)
	if err != nil {
		return nil, err
	}
	sdkImage := fmt.Sprintf("%s/%s:%s", imageRegistry, imageRepository, imageTag)
	fmt.Printf("Image pushed to %s\n", sdkImage)

	chart, err := buildAndPushChartToTTL(ctx, source, imageRegistry, imageRepository, imageTag)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Chart pushed to %s\n", chart)

	channelSlug, err := createAppTestRelease(ctx, source, opServiceAccount, chart)
	if err != nil {
		return nil, err
	}

	customerID, licenseID, err := createCustomer(ctx, channelSlug, opServiceAccount)
	if err != nil {
		return nil, err
	}
	fmt.Println(customerID, licenseID)

	// Resolve app ID for replicated-sdk-e2e (used by vendor API checks)
	appID, err := getAppID(ctx, opServiceAccount, "replicated-sdk-e2e")
	if err != nil {
		return nil, err
	}

	cmxDistributions, err := listCMXDistributionsAndVersions(ctx, opServiceAccount)
	if err != nil {
		return nil, err
	}

	buildInfo := BuildInfo{
		ImageRegistry:   imageRegistry,
		ImageRepository: imageRepository,
		ImageTag:        imageTag,
		Chart:           chart,
		ChannelSlug:     channelSlug,
		CustomerID:      customerID,
		LicenseID:       licenseID,
		AppID:           appID,
		Distributions:   cmxDistributions,
	}

	buildInfoJSON, err := json.Marshal(buildInfo)
	if err != nil {
		return nil, err
	}

	// Create a file with the build info
	buildInfoFile := dag.Directory().WithNewFile("build-info.json", string(buildInfoJSON)).File("build-info.json")

	return buildInfoFile, nil
}

// RunSingleE2E runs e2e tests for a single distribution/version
func (m *ReplicatedSdk) RunSingleE2E(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,

	// The distribution to test (e.g., "eks", "gke", "openshift", "oke")
	distribution string,

	// The version to test
	version string,

	// The build info file containing app ID, customer ID, license ID, etc.
	buildInfoFile *dagger.File,
) error {
	// Read and parse the build info file
	buildInfoJSON, err := buildInfoFile.Contents(ctx)
	if err != nil {
		return fmt.Errorf("failed to read build info file: %w", err)
	}

	var buildInfo BuildInfo
	if err := json.Unmarshal([]byte(buildInfoJSON), &buildInfo); err != nil {
		return fmt.Errorf("failed to parse build info: %w", err)
	}

	sdkImage := fmt.Sprintf("%s/%s:%s", buildInfo.ImageRegistry, buildInfo.ImageRepository, buildInfo.ImageTag)

	return e2e(ctx, source, opServiceAccount, buildInfo.AppID, buildInfo.CustomerID, sdkImage, buildInfo.LicenseID, distribution, version, buildInfo.ChannelSlug)
}
