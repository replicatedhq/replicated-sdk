package main

import (
	"bytes"
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// CONSTANTS & CONFIGURATION
// =============================================================================

const (
	// SecureBuild API endpoint
	SecureBuildAPIEndpoint = "https://securebuild.com"
)

// Centralized Registry and Image Configuration
// Single source of truth for all registry URLs, image names, and hosts
// This solves the SecureBuild image naming architecture problem where different
// registries require different image names but SecureBuild API only supports single names per call
const (
	// Registry namespaces (full paths where SecureBuild will push images)
	DockerHubRegistry            = "index.docker.io/replicated"              // Docker Hub public registry
	ReplicatedStagingRegistry    = "registry.staging.replicated.com/library" // Replicated staging environment
	ReplicatedProductionRegistry = "registry.replicated.com/library"         // Replicated production environment
	TTLRegistry                  = "ttl.sh/replicated"                       // TTL.sh for development builds
	CVE0Registry                 = "cve0.io"                                 // CVE0.io uses different path structure (no namespace)

	// Image names per registry type (the core architectural difference)
	// Different registries require different image names - this is why we need separate SecureBuild API calls
	DockerHubImageName  = "replicated-sdk"       // Docker Hub uses this name
	ReplicatedImageName = "replicated-sdk-image" // Replicated registries use this name (staging + production)
	DevImageName        = "replicated-sdk"       // Development builds use Docker Hub naming convention
	CVE0ImageName       = "replicated-sdk"       // CVE0.io uses Docker Hub naming convention

	// Registry hosts for credential configuration (hostnames without paths)
	// Used for SecureBuild external registry configuration - needs just the hostname
	DockerHubHost            = "index.docker.io"                 // Docker Hub hostname for auth
	ReplicatedStagingHost    = "registry.staging.replicated.com" // Replicated staging hostname for auth
	ReplicatedProductionHost = "registry.replicated.com"         // Replicated production hostname for auth
	TTLHost                  = "ttl.sh"                          // TTL.sh hostname for auth

	// Registry usernames for external registry configuration
	// TTL.sh uses a standard username for all builds
	TTLUsername = "replicated" // TTL.sh username for external registry configuration

	// 1Password item names and field names for SecureBuild API tokens
	// Different environments use different tokens stored in different vaults
	SecureBuildDevTokenItem  = "SecureBuild-Dev-Token"                 // Development token item name
	SecureBuildProdTokenItem = "SecureBuild-Replicated-SDK-Prod-Token" // Production token item name
	SecureBuildAPITokenField = "APIToken"                              // 1Password field name for API tokens

	// File paths for build configuration
	APKOConfigFile = "deploy/apko-securebuild.yaml" // APKO configuration file path

	// APKO configuration template values
	APKOVersionTemplate = "VERSION: 1.0.0" // Template version string to replace in APKO config

	// SecureBuild API endpoint paths
	CustomAPKOEndpoint       = "/api/v1/custom-apko"              // SecureBuild custom APKO submission endpoint
	ExternalRegistryEndpoint = "/api/v1/custom-external-registry" // SecureBuild external registry configuration endpoint

	// TTL.sh credential configuration
	TTLPasswordSecret = "ttl-password" // Secret name for TTL.sh password
	TTLPassword       = "nopass"       // TTL.sh uses placeholder password
)

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

// ValidationError represents structured validation errors with actionable suggestions
type ValidationError struct {
	Type        string   // Error type (e.g., "version_collision", "credential_access_failed")
	Message     string   // Human-readable error message
	Suggestions []string // Actionable suggestions for resolving the error
}

func (e ValidationError) Error() string {
	if len(e.Suggestions) > 0 {
		return fmt.Sprintf("%s\nSuggestions:\n- %s", e.Message, strings.Join(e.Suggestions, "\n- "))
	}
	return e.Message
}

// RegistryConfig defines the configuration for a specific registry publication
// This encapsulates all the differences between Docker Hub and Replicated registries
type RegistryConfig struct {
	Name               string // Human-readable name for logging (e.g., "Docker Hub", "Replicated Staging")
	Host               string // Registry hostname for credential configuration (e.g., "index.docker.io")
	RegistryNamespace  string // Full registry path for image pushing (e.g., "index.docker.io/replicated")
	ImageName          string // Image name for this registry (e.g., "replicated-sdk" vs "replicated-sdk-image")
	CredentialUsername string // 1Password item name for username
	CredentialPassword string // 1Password field name for password
	IsReplicated       bool   // Whether this is a Replicated registry (affects credential logic)
}

// SecureBuildClient handles interaction with SecureBuild API
type SecureBuildClient struct {
	apiEndpoint string
	apiToken    *dagger.Secret
	httpClient  *http.Client
}

// CustomAPKORequest represents the SecureBuild API request for custom APKO builds
type CustomAPKORequest struct {
	Name         string   `json:"name"`
	Tags         []string `json:"tags"`
	Config       string   `json:"config"` // base64-encoded apko.yaml
	Readme       string   `json:"readme"`
	RegistryURLs []string `json:"registry_urls"`
}

// CustomAPKOResponse represents the SecureBuild API response for custom APKO builds
type CustomAPKOResponse struct {
	Success             bool     `json:"success"`
	Message             string   `json:"message"`
	CustomImageID       string   `json:"customImageId"`
	CustomAPKOID        string   `json:"customApkoId"`
	CustomAPKOVersionID string   `json:"customApkoVersionId"`
	Name                string   `json:"name"`
	Tags                []string `json:"tags"`
}

// ExternalRegistry represents a configured external registry in SecureBuild
type ExternalRegistry struct {
	CustomImageID string `json:"customImageId"`
	RegistryURL   string `json:"registryUrl"`
	Username      string `json:"username"`
}

// =============================================================================
// PHASE 1: VALIDATION FUNCTIONS
// =============================================================================

// validateCredentialAccess tests 1Password access before starting expensive operations
// This prevents failures after time-consuming setup steps
func validateCredentialAccess(ctx context.Context, opServiceAccount *dagger.Secret, environment string) error {
	fmt.Printf("🔍 Phase 1 Validation: Testing credential access for %s environment\n", environment)

	// Test 1Password service account token access
	fmt.Printf("  Testing 1Password service account access...\n")

	// Try to access a known secret to verify credentials work
	var testVault Vault
	var testItem string

	if environment == SecureBuildEnvDev {
		testVault = VaultDeveloperAutomation
		testItem = SecureBuildDevTokenItem
	} else {
		testVault = VaultDeveloperAutomationProduction
		testItem = SecureBuildProdTokenItem
	}

	// Attempt to retrieve the secret using mustGetSecret pattern, but with error handling
	// We'll use a safer approach to test credential access
	var credentialError error
	func() {
		defer func() {
			if r := recover(); r != nil {
				// If mustGetSecret panics, we know credentials don't work
				credentialError = ValidationError{
					Type:    "credential_access_failed",
					Message: fmt.Sprintf("Failed to access 1Password credentials for %s environment: %v", environment, r),
					Suggestions: []string{
						"Verify OP_SERVICE_ACCOUNT_TOKEN is correctly set",
						"Check that service account has access to required vault: " + string(testVault),
						"Ensure secret item exists: " + testItem,
						"Verify service account token is not expired",
					},
				}
			}
		}()

		// Test by getting the item UUID first (this tests basic access)
		_ = mustGetItemUUID(ctx, opServiceAccount, testItem, testVault)
	}()

	if credentialError != nil {
		return credentialError
	}

	fmt.Printf("✅ Credential validation passed: 1Password access confirmed\n")
	return nil
}

// =============================================================================
// SECUREBUILD CLIENT & API LAYER
// =============================================================================

// NewSecureBuildClient creates a new SecureBuild API client
func NewSecureBuildClient(apiEndpoint string, apiToken *dagger.Secret) *SecureBuildClient {
	return &SecureBuildClient{
		apiEndpoint: apiEndpoint,
		apiToken:    apiToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SubmitCustomAPKO submits a custom APKO configuration to SecureBuild
func (c *SecureBuildClient) SubmitCustomAPKO(ctx context.Context, request CustomAPKORequest) (*CustomAPKOResponse, error) {
	// Prepare request body
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Get the API token value
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+CustomAPKOEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response CustomAPKOResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// CreateExternalRegistry registers an external registry with SecureBuild for the given custom image
func (c *SecureBuildClient) CreateExternalRegistry(ctx context.Context, customImageID, registryURL, username, password string) error {
	// Prepare request body
	requestData := map[string]interface{}{
		"customImageId": customImageID,
		"host":          registryURL,
		"username":      username,
		"password":      password,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal external registry request: %w", err)
	}

	// Get the API token value
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+ExternalRegistryEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create external registry request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("external registry API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read external registry response body: %w", err)
	}

	// 409 Conflict means registry already exists for this team - that's OK
	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryURL)
		return nil
	}

	// Handle 400 Bad Request with "already exists" message (some API implementations)
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "already exists") {
		fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryURL)
		return nil
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("external registry API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// listExternalRegistries fetches all configured external registries for the team
func (c *SecureBuildClient) listExternalRegistries(ctx context.Context) ([]ExternalRegistry, error) {
	// Get the API token value
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiEndpoint+ExternalRegistryEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create registries list request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+tokenValue)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registries list API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registries response: %w", err)
	}

	// Try to unmarshal - the API might return an object with a registries array inside
	var response struct {
		Registries []ExternalRegistry `json:"registries"`
	}

	// First try object with registries array
	if err := json.Unmarshal(body, &response); err == nil {
		return response.Registries, nil
	}

	// Fallback: try direct array unmarshaling
	var registries []ExternalRegistry
	if err := json.Unmarshal(body, &registries); err != nil {
		return nil, fmt.Errorf("failed to decode registries response (tried both array and object format): %w", err)
	}

	return registries, nil
}

// EnsureExternalRegistry ensures an external registry is configured for the team
// This is called once per registry to set up credentials at the team level
func (c *SecureBuildClient) EnsureExternalRegistry(ctx context.Context, registryHost, username string, password *dagger.Secret) error {
	// Get password value
	passwordValue, err := password.Plaintext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get registry password: %w", err)
	}

	// Check if registry is already configured by listing all registries
	existingRegistries, err := c.listExternalRegistries(ctx)
	if err != nil {
		fmt.Printf("Warning: Could not list existing registries (will attempt to create): %v\n", err)
		// Continue with creation attempt even if listing fails
	} else {
		// Check if our registry host already exists
		for _, registry := range existingRegistries {
			if strings.Contains(registry.RegistryURL, registryHost) {
				fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryHost)
				return nil
			}
		}
	}

	// Registry not found, create it with a placeholder custom image ID
	// The actual custom image ID will be used when we create the APKO config
	err = c.CreateExternalRegistry(ctx, "placeholder", registryHost, username, passwordValue)
	if err != nil {
		return fmt.Errorf("failed to create external registry %s: %w", registryHost, err)
	}

	fmt.Printf("SecureBuild: External registry %s configured successfully\n", registryHost)
	return nil
}

// GetSecureBuildClient creates a SecureBuild client with configuration from 1Password
func (m *ReplicatedSdk) GetSecureBuildClient(ctx context.Context, opServiceAccount *dagger.Secret, environment string) (*SecureBuildClient, error) {
	apiEndpoint := SecureBuildAPIEndpoint

	var apiToken *dagger.Secret

	if environment == SecureBuildEnvDev {
		// Dev: use dev vault and dev token
		apiToken = mustGetSecret(ctx, opServiceAccount, SecureBuildDevTokenItem, SecureBuildAPITokenField, VaultDeveloperAutomation)
	} else {
		// Staging and Production: use production vault and production token
		apiToken = mustGetSecret(ctx, opServiceAccount, SecureBuildProdTokenItem, SecureBuildAPITokenField, VaultDeveloperAutomationProduction)
	}

	return NewSecureBuildClient(apiEndpoint, apiToken), nil
}

// generateAPKOConfig creates the apko.yaml content based on the SecureBuild template
func (m *ReplicatedSdk) generateAPKOConfig(ctx context.Context, source *dagger.Directory, version string) (string, error) {
	// Read the SecureBuild-specific apko.yaml template
	apkoYaml, err := source.File(APKOConfigFile).Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", APKOConfigFile, err)
	}

	// Update version placeholder with proper YAML string quoting
	apkoYaml = strings.Replace(apkoYaml, APKOVersionTemplate, fmt.Sprintf("VERSION: \"%s\"", version), 1)

	// Update package version for both dev and production builds
	// Use dynamic version from GitHub workflow instead of hardcoded values
	// CVE0.io uses semver format: packagename-x.x.x (not melange format with underscores)
	apkoYaml = strings.Replace(
		apkoYaml,
		"    - replicated\n",
		fmt.Sprintf("    - replicated-%s\n", version),
		1,
	)

	return apkoYaml, nil
}

// =============================================================================
// REGISTRY PUBLISHING LOGIC (Phase 1.5)
// =============================================================================

// publishToRegistry handles SecureBuild API calls for any registry using the provided configuration
//
// WHY WE NEED SEPARATE CALLS PER REGISTRY:
// SecureBuild's /api/v1/custom-apko endpoint only accepts a single image name per call, but different
// registries require different image names:
//   - Docker Hub requires: "replicated-sdk"
//   - Replicated registries require: "replicated-sdk-image"
//
// Since SecureBuild cannot handle multiple image names in one call, we must make separate API calls
// for each registry, each with the correct image name for that specific registry.
func (m *ReplicatedSdk) publishToRegistry(ctx context.Context, client *SecureBuildClient, source *dagger.Directory, version string, opServiceAccount *dagger.Secret, environment string, config RegistryConfig) (string, error) {
	fmt.Printf("📦 SecureBuild: Publishing to %s (%s)...\n", config.Name, config.RegistryNamespace)

	// Get credentials based on registry type
	var username string
	var password *dagger.Secret

	if config.IsReplicated {
		// Replicated registries use different password fields for staging vs production
		username = mustGetNonSensitiveSecret(ctx, opServiceAccount, config.CredentialUsername, "library_username", VaultDeveloperAutomationProduction)
		if isProductionEnvironment(environment) {
			password = mustGetSecret(ctx, opServiceAccount, config.CredentialPassword, "library_password", VaultDeveloperAutomationProduction)
		} else {
			password = mustGetSecret(ctx, opServiceAccount, config.CredentialPassword, "staging_library_password", VaultDeveloperAutomationProduction)
		}
	} else {
		// Docker Hub uses standard username/password fields
		username = mustGetNonSensitiveSecret(ctx, opServiceAccount, config.CredentialUsername, "username", VaultDeveloperAutomationProduction)
		password = mustGetSecret(ctx, opServiceAccount, config.CredentialPassword, "password", VaultDeveloperAutomationProduction)
	}

	// Ensure external registry is configured
	err := client.EnsureExternalRegistry(ctx, config.Host, username, password)
	if err != nil {
		return "", fmt.Errorf("failed to configure %s external registry: %w", config.Name, err)
	}

	// Generate APKO configuration
	apkoContent, err := m.generateAPKOConfig(ctx, source, version)
	if err != nil {
		return "", fmt.Errorf("failed to generate APKO config for %s: %w", config.Name, err)
	}
	apkoBase64 := base64.StdEncoding.EncodeToString([]byte(apkoContent))

	// Determine tags
	tags := []string{version}
	if isProductionEnvironment(environment) && !strings.Contains(version, "beta") && !strings.Contains(version, "alpha") {
		tags = append(tags, "latest")
	}

	// Submit APKO configuration to SecureBuild
	fmt.Printf("SecureBuild: Submitting %s APKO config for %s with tags %v\n", config.Name, version, tags)

	response, err := client.SubmitCustomAPKO(ctx, CustomAPKORequest{
		Name:         config.ImageName, // Registry-specific image name (the key difference!)
		Tags:         tags,
		Config:       apkoBase64,
		Readme:       "Replicated SDK container image",
		RegistryURLs: []string{config.RegistryNamespace}, // Registry-specific namespace
	})
	if err != nil {
		return "", fmt.Errorf("failed to submit %s APKO config: %w", config.Name, err)
	}

	imageRef := fmt.Sprintf("%s/%s:%s", config.RegistryNamespace, config.ImageName, version)
	fmt.Printf("✅ SecureBuild: %s build queued (Custom Image ID: %s). Image will be available at: %s\n", config.Name, response.CustomImageID, imageRef)

	return response.CustomImageID, nil
}

// publishToDockerHub handles the SecureBuild API call for Docker Hub registry
func (m *ReplicatedSdk) publishToDockerHub(ctx context.Context, client *SecureBuildClient, source *dagger.Directory, version string, opServiceAccount *dagger.Secret, environment string) (string, error) {
	config := RegistryConfig{
		Name:               "Docker Hub",
		Host:               DockerHubHost,
		RegistryNamespace:  DockerHubRegistry,
		ImageName:          DockerHubImageName,
		CredentialUsername: "Docker Hub Release Account",
		CredentialPassword: "Docker Hub Release Account",
		IsReplicated:       false,
	}
	return m.publishToRegistry(ctx, client, source, version, opServiceAccount, environment, config)
}

// publishToReplicatedRegistry handles the SecureBuild API call for Replicated registries (staging/production)
func (m *ReplicatedSdk) publishToReplicatedRegistry(ctx context.Context, client *SecureBuildClient, source *dagger.Directory, version string, opServiceAccount *dagger.Secret, environment string) (string, error) {
	var config RegistryConfig

	// Configure registry based on environment
	if environment == SecureBuildEnvStaging {
		config = RegistryConfig{
			Name:               "Replicated Staging",
			Host:               ReplicatedStagingHost,
			RegistryNamespace:  ReplicatedStagingRegistry,
			ImageName:          ReplicatedImageName,
			CredentialUsername: "Replicated SDK Publish",
			CredentialPassword: "Replicated SDK Publish",
			IsReplicated:       true,
		}
	} else {
		config = RegistryConfig{
			Name:               "Replicated Production",
			Host:               ReplicatedProductionHost,
			RegistryNamespace:  ReplicatedProductionRegistry,
			ImageName:          ReplicatedImageName,
			CredentialUsername: "Replicated SDK Publish",
			CredentialPassword: "Replicated SDK Publish",
			IsReplicated:       true,
		}
	}

	return m.publishToRegistry(ctx, client, source, version, opServiceAccount, environment, config)
}

// =============================================================================
// MAIN ENTRY POINTS
// =============================================================================

// PublishSecureBuild publishes using SecureBuild instead of direct image building
func (m *ReplicatedSdk) PublishSecureBuild(ctx context.Context, source *dagger.Directory, version string, environment string, opServiceAccount *dagger.Secret) (string, error) {
	fmt.Printf("🚀 SecureBuild: Starting publish with Phase 1 pre-build validations\n")

	// Phase 1: Critical Pre-build Validations
	fmt.Printf("📋 Phase 1: Running critical pre-build validations...\n")

	// Validation 1: Test credential access before expensive operations
	if err := validateCredentialAccess(ctx, opServiceAccount, environment); err != nil {
		return "", fmt.Errorf("credential validation failed: %w", err)
	}

	fmt.Printf("✅ Phase 1 validations completed successfully - proceeding with build\n")
	fmt.Printf("🔧 SecureBuild: Initializing sequential registry publishing...\n")

	// Get SecureBuild client
	client, err := m.GetSecureBuildClient(ctx, opServiceAccount, environment)
	if err != nil {
		return "", fmt.Errorf("failed to create SecureBuild client: %w", err)
	}

	// Phase 1.5: Sequential registry publishing to handle different image names per registry
	// This approach ensures proper image naming: Docker Hub uses "replicated-sdk", Replicated uses "replicated-sdk-image"

	var dockerHubImageID, replicatedImageID string
	var publishResults []string

	// Step 1: Publish to Docker Hub
	fmt.Printf("📦 Step 1/2: Publishing to Docker Hub...\n")
	dockerHubImageID, err = m.publishToDockerHub(ctx, client, source, version, opServiceAccount, environment)
	if err != nil {
		fmt.Printf("❌ Docker Hub publication failed: %v\n", err)
		return "", fmt.Errorf("critical failure: Docker Hub publication failed (no images published): %w", err)
	}
	publishResults = append(publishResults, fmt.Sprintf("Docker Hub (ID: %s)", dockerHubImageID))

	// Step 2: Publish to Replicated registry
	fmt.Printf("📦 Step 2/2: Publishing to Replicated registry...\n")
	replicatedImageID, err = m.publishToReplicatedRegistry(ctx, client, source, version, opServiceAccount, environment)
	if err != nil {
		fmt.Printf("❌ Replicated registry publication failed: %v\n", err)
		fmt.Printf("⚠️  WARNING: Partial failure - Docker Hub succeeded but Replicated registry failed\n")
		fmt.Printf("    Successfully published images:\n")
		fmt.Printf("    - %s\n", publishResults[0])
		fmt.Printf("    Manual intervention may be required to complete Replicated registry publication\n")
		return "", fmt.Errorf("partial failure: Replicated registry publication failed (Docker Hub succeeded, ID: %s): %w", dockerHubImageID, err)
	}
	publishResults = append(publishResults, fmt.Sprintf("Replicated Registry (ID: %s)", replicatedImageID))

	// Return CVE0.io reference (primary image location) for consistency with existing pipeline
	imageRef := fmt.Sprintf("%s/%s:%s", CVE0Registry, CVE0ImageName, version)
	fmt.Printf("🎉 SecureBuild: All registries published successfully!\n")
	for i, result := range publishResults {
		fmt.Printf("   %d. %s\n", i+1, result)
	}
	fmt.Printf("   - Primary CVE0.io reference: %s\n", imageRef)

	return imageRef, nil
}

// BuildDevSecureBuild builds development images using SecureBuild
// This requires USE_SECUREBUILD=true and environment set to dev
// This requires the user to have a SecureBuild account, otherwise we just use the legacy tt.sh
func (m *ReplicatedSdk) BuildDevSecureBuild(ctx context.Context,
	// +defaultPath="/"
	source *dagger.Directory, // Source code directory containing the application to build
	version string, // Version tag for the built image (e.g., "v1.0.0")
	opServiceAccount *dagger.Secret, // 1Password service account secret for authentication
) (string, error) {
	fmt.Printf("🚀 SecureBuild Dev: Starting build with Phase 1 pre-build validations\n")

	// Phase 1: Critical Pre-build Validations for Development
	fmt.Printf("📋 Phase 1: Running critical pre-build validations...\n")

	// For dev builds, we skip some validations but keep the important ones
	// Validation 1: Test credential access before expensive operations
	if err := validateCredentialAccess(ctx, opServiceAccount, SecureBuildEnvDev); err != nil {
		return "", fmt.Errorf("credential validation failed: %w", err)
	}

	// Note: We skip version collision check for dev builds since they use ttl.sh

	fmt.Printf("✅ Phase 1 dev validations completed successfully - proceeding with build\n")
	fmt.Printf("🔧 SecureBuild Dev: Initializing build process...\n")

	// 1. Get SecureBuild client
	client, err := m.GetSecureBuildClient(ctx, opServiceAccount, SecureBuildEnvDev)
	if err != nil {
		return "", fmt.Errorf("failed to create SecureBuild client: %w", err)
	}

	// 2. Ensure team has TTL.sh external registry configured (team-level, persists across builds)
	fmt.Printf("SecureBuild: Ensuring TTL.sh external registry is configured for team...\n")
	err = client.EnsureExternalRegistry(ctx, TTLHost, TTLUsername, dag.SetSecret(TTLPasswordSecret, TTLPassword)) // TTL.sh with placeholder password
	if err != nil {
		return "", fmt.Errorf("failed to configure TTL.sh external registry: %w", err)
	}

	// 3. Generate apko.yaml config for development
	apkoContent, err := m.generateAPKOConfig(ctx, source, version)
	if err != nil {
		return "", fmt.Errorf("failed to generate APKO config: %w", err)
	}

	// Debug: Print the APKO config being sent
	fmt.Printf("=== APKO CONFIG BEING SENT TO SECUREBUILD ===\n%s\n=== END APKO CONFIG ===\n", apkoContent)

	apkoBase64 := base64.StdEncoding.EncodeToString([]byte(apkoContent))

	// 4. Use exact TTL.sh registry URL
	ttlRegistryURL := TTLRegistry

	// 5. Submit APKO configuration with registry URLs (SecureBuild will push to cve0.io + external registries)
	fmt.Printf("SecureBuild: Submitting dev APKO config for %s\n", version)
	response, err := client.SubmitCustomAPKO(ctx, CustomAPKORequest{
		Name:         DevImageName, // "replicated-sdk"
		Tags:         []string{version},
		Config:       apkoBase64,
		Readme:       fmt.Sprintf("Replicated SDK development build %s", version),
		RegistryURLs: []string{ttlRegistryURL}, // Only external registries, cve0.io is automatic
	})
	if err != nil {
		return "", fmt.Errorf("failed to submit dev APKO config: %w", err)
	}

	// Dev builds available on both registries
	imageRef := fmt.Sprintf("%s/%s:%s", CVE0Registry, CVE0ImageName, version)
	ttlImageRef := fmt.Sprintf("%s/%s:%s", ttlRegistryURL, DevImageName, version)
	fmt.Printf("SecureBuild: Dev build submitted (Custom Image ID: %s)\n", response.CustomImageID)
	fmt.Printf("SecureBuild: Image will be available at: %s\n", imageRef)
	fmt.Printf("SecureBuild: Image will also be available at: %s\n", ttlImageRef)

	// Return primary CVE0.io reference for compatibility with build.go parsing logic
	return imageRef, nil
}
