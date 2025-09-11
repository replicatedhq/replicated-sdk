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

const (
	// SecureBuild API endpoint
	SecureBuildAPIEndpoint = "https://securebuild.com"
)

// SecureBuildClient handles interaction with SecureBuild API
type SecureBuildClient struct {
	apiEndpoint string
	apiToken    *dagger.Secret
	httpClient  *http.Client
}

// CustomAPKORequest represents the API request to SecureBuild
type CustomAPKORequest struct {
	Name         string   `json:"name"`          // Image name
	Tags         []string `json:"tags"`          // Image tags
	Config       string   `json:"config"`        // base64 encoded YAML
	Readme       string   `json:"readme"`        // Optional description
	RegistryURLs []string `json:"registry_urls"` // Registry URLs for pushing
}

// CustomAPKOResponse represents the API response from SecureBuild
type CustomAPKOResponse struct {
	Success             bool     `json:"success"`
	Message             string   `json:"message"`
	CustomImageID       string   `json:"custom_image_id"`
	CustomAPKOID        string   `json:"custom_apko_id"`
	CustomAPKOVersionID string   `json:"custom_apko_version_id"`
	Name                string   `json:"name"`
	Tags                []string `json:"tags"`
}

// CustomExternalRegistryRequest represents registry credential request
type CustomExternalRegistryRequest struct {
	CustomImageID string `json:"custom_image_id"`
	RegistryURL   string `json:"registry_url"`
	Username      string `json:"username"`
	Password      string `json:"password"`
}

// CustomExternalRegistryResponse represents registry credential response
type CustomExternalRegistryResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	RegistryID string `json:"registry_id"`
}

// NewSecureBuildClient creates a new SecureBuild API client
func NewSecureBuildClient(apiEndpoint string, apiToken *dagger.Secret) *SecureBuildClient {
	return &SecureBuildClient{
		apiEndpoint: apiEndpoint,
		apiToken:    apiToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SubmitCustomAPKO submits an APKO config to SecureBuild
func (c *SecureBuildClient) SubmitCustomAPKO(ctx context.Context, request CustomAPKORequest) (*CustomAPKOResponse, error) {
	// Get the API token value
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	reqJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+"/api/v1/custom-apko", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenValue)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CustomAPKOResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// CreateExternalRegistry configures registry credentials for pushing
func (c *SecureBuildClient) CreateExternalRegistry(ctx context.Context, customImageID, registryURL, username, password string) error {
	// Get the API token value
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	req := CustomExternalRegistryRequest{
		CustomImageID: customImageID,
		RegistryURL:   registryURL,
		Username:      username,
		Password:      password,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal registry request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+"/api/v1/custom-external-registry", bytes.NewBuffer(reqJSON))
	if err != nil {
		return fmt.Errorf("failed to create registry request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenValue)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute registry request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read registry response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registry API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CustomExternalRegistryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal registry response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("registry configuration failed: %s", response.Message)
	}

	fmt.Printf("SecureBuild: Configured registry %s (ID: %s)\n", registryURL, response.RegistryID)
	return nil
}

// ExternalRegistry represents a configured external registry
type ExternalRegistry struct {
	ID   string `json:"id"`
	Host string `json:"host"`
}

// listExternalRegistries returns the list of external registries configured for the team
// Note: lowercase function name makes it internal to avoid Dagger codegen issues
func (c *SecureBuildClient) listExternalRegistries(ctx context.Context) ([]ExternalRegistry, error) {
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.apiEndpoint+"/api/v1/custom-external-registry", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list registries request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenValue)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute list registries request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list registries API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read the raw response to understand the format
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

// EnsureExternalRegistry ensures the team has external registry configured for the given host
// This is a team-level configuration that persists across multiple APKO submissions
func (c *SecureBuildClient) EnsureExternalRegistry(ctx context.Context, registryHost, username string, password *dagger.Secret) error {
	// First, check if registry already exists
	registries, err := c.listExternalRegistries(ctx)
	if err != nil {
		// If we can't list registries, fall back to the POST approach with error handling
		fmt.Printf("SecureBuild: Warning - could not list existing registries, proceeding with registration attempt: %v\n", err)
	} else {
		// Check if our registry host already exists
		for _, registry := range registries {
			if registry.Host == registryHost {
				fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryHost)
				return nil
			}
		}
		fmt.Printf("SecureBuild: Registry %s not found in existing registries, creating new registration\n", registryHost)
	}

	// Get the API token value for POST request
	tokenValue, err := c.apiToken.Plaintext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	// Get password plaintext only when needed
	var passwordValue string
	if password != nil {
		passwordValue, err = password.Plaintext(ctx)
		if err != nil {
			return fmt.Errorf("failed to get registry password: %w", err)
		}
	}

	req := map[string]string{
		"host":     registryHost,
		"username": username,
		"password": passwordValue,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal external registry request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+"/api/v1/custom-external-registry", bytes.NewBuffer(reqJSON))
	if err != nil {
		return fmt.Errorf("failed to create external registry request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenValue)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute external registry request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read external registry response body: %w", err)
	}

	// 409 Conflict means registry already exists for this team - that's OK
	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryHost)
		return nil
	}
	
	// Handle 400 Bad Request with "already exists" message (some API implementations)
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "already exists") {
		fmt.Printf("SecureBuild: External registry %s already configured for team\n", registryHost)
		return nil
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("external registry API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal external registry response: %w", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		if message, ok := response["error"].(string); ok {
			return fmt.Errorf("external registry configuration failed: %s", message)
		}
		return fmt.Errorf("external registry configuration failed: unknown error")
	}

	fmt.Printf("SecureBuild: Successfully configured external registry %s\n", registryHost)
	return nil
}

// GetSecureBuildClient creates a SecureBuild client with configuration from 1Password
func (m *ReplicatedSdk) GetSecureBuildClient(ctx context.Context, opServiceAccount *dagger.Secret, environment string) (*SecureBuildClient, error) {
	apiEndpoint := SecureBuildAPIEndpoint

	var apiToken *dagger.Secret
	
	// TODO: TEMPORARY - LOCAL TESTING ONLY - Remove hardcoded token before production
	// Bypass 1Password for local testing with Docker container
	if apiEndpoint == "http://host.docker.internal:3001" {
		apiToken = dag.SetSecret("local-test-token", "MMOOLvpL7vAxQDMYK188wtgHkgDZj5rxH0dW")
	} else if environment == SecureBuildEnvDev {
		// Dev: use dev vault and dev token
		apiToken = mustGetSecret(ctx, opServiceAccount, "SecureBuild-Dev-Token", "APIToken", VaultDeveloperAutomation)
	} else {
		// Staging and Production: use production vault and production token
		apiToken = mustGetSecret(ctx, opServiceAccount, "SecureBuild-Replicated-SDK-Prod-Token", "APIToken", VaultDeveloperAutomationProduction)
	}

	return NewSecureBuildClient(apiEndpoint, apiToken), nil
}

// generateAPKOConfig creates the apko.yaml content based on the SecureBuild template
func (m *ReplicatedSdk) generateAPKOConfig(ctx context.Context, source *dagger.Directory, version string) (string, error) {
	// Read the SecureBuild-specific apko.yaml template
	apkoYaml, err := source.File("deploy/apko-securebuild.yaml").Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read deploy/apko-securebuild.yaml: %w", err)
	}

	// Update version placeholder with proper YAML string quoting
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: \"%s\"", version), 1)

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

// PublishSecureBuild publishes using SecureBuild instead of direct image building
func (m *ReplicatedSdk) PublishSecureBuild(ctx context.Context, source *dagger.Directory, version string, environment string, opServiceAccount *dagger.Secret) (string, error) {
	// 1. Get SecureBuild client
	client, err := m.GetSecureBuildClient(ctx, opServiceAccount, environment)
	if err != nil {
		return "", fmt.Errorf("failed to create SecureBuild client: %w", err)
	}

	// 2. Get proper registry credentials from 1Password (environment-aware)
	var dockerUsername string
	var dockerPassword *dagger.Secret
	var replicatedUsername string
	var replicatedPassword *dagger.Secret

	if isProductionEnvironment(environment) {
		// Production credentials
		dockerUsername = mustGetNonSensitiveSecret(ctx, opServiceAccount, "Docker Hub Release Account", "username", VaultDeveloperAutomationProduction)
		dockerPassword = mustGetSecret(ctx, opServiceAccount, "Docker Hub Release Account", "password", VaultDeveloperAutomationProduction)
		replicatedUsername = mustGetNonSensitiveSecret(ctx, opServiceAccount, "Replicated SDK Publish", "library_username", VaultDeveloperAutomationProduction)
		replicatedPassword = mustGetSecret(ctx, opServiceAccount, "Replicated SDK Publish", "library_password", VaultDeveloperAutomationProduction)
	} else {
		// Staging credentials (also from production vault)
		dockerUsername = mustGetNonSensitiveSecret(ctx, opServiceAccount, "Docker Hub Release Account", "username", VaultDeveloperAutomationProduction)
		dockerPassword = mustGetSecret(ctx, opServiceAccount, "Docker Hub Release Account", "password", VaultDeveloperAutomationProduction)
		replicatedUsername = mustGetNonSensitiveSecret(ctx, opServiceAccount, "Replicated SDK Publish", "library_username", VaultDeveloperAutomationProduction)
		replicatedPassword = mustGetSecret(ctx, opServiceAccount, "Replicated SDK Publish", "staging_library_password", VaultDeveloperAutomationProduction)
	}

	// 3. Ensure team has all external registries configured (team-level, persists across builds)
	fmt.Printf("SecureBuild: Ensuring Docker Hub external registry is configured for team...\n")
	err = client.EnsureExternalRegistry(ctx, "index.docker.io", dockerUsername, dockerPassword)
	if err != nil {
		return "", fmt.Errorf("failed to configure Docker Hub external registry: %w", err)
	}

	// Determine Replicated registry host and configure it
	var replicatedRegistryHost string
	if environment == SecureBuildEnvStaging {
		replicatedRegistryHost = "registry.staging.replicated.com"
	} else {
		replicatedRegistryHost = "registry.replicated.com"
	}
	
	fmt.Printf("SecureBuild: Ensuring Replicated registry (%s) is configured for team...\n", replicatedRegistryHost)
	err = client.EnsureExternalRegistry(ctx, replicatedRegistryHost, replicatedUsername, replicatedPassword)
	if err != nil {
		return "", fmt.Errorf("failed to configure Replicated registry: %w", err)
	}

	// 4. Generate apko.yaml config
	apkoContent, err := m.generateAPKOConfig(ctx, source, version)
	if err != nil {
		return "", fmt.Errorf("failed to generate APKO config: %w", err)
	}
	apkoBase64 := base64.StdEncoding.EncodeToString([]byte(apkoContent))

	// 4. Define registry URLs for image pushing  
	// Note: SecureBuild appends the image name, so we send just the namespace
	registryURLs := []string{
		"index.docker.io/replicated",  // SecureBuild will append image name
	}
	if environment == SecureBuildEnvStaging {
		registryURLs = append(registryURLs, "registry.staging.replicated.com/library")
	} else {
		registryURLs = append(registryURLs, "registry.replicated.com/library")
	}

	// 5. Determine tags
	tags := []string{version}
	if isProductionEnvironment(environment) && !strings.Contains(version, "beta") && !strings.Contains(version, "alpha") {
		tags = append(tags, "latest")
	}

	// 6. Submit APKO configuration to SecureBuild
	fmt.Printf("SecureBuild: Submitting APKO config for %s with tags %v\n", version, tags)
	
	// Determine image name based on environment for feature parity
	imageName := "replicated-sdk"
	if environment == SecureBuildEnvStaging || environment == SecureBuildEnvProduction {
		imageName = "replicated-sdk-image" // Match Chainguard pipeline naming
	}
	
	response, err := client.SubmitCustomAPKO(ctx, CustomAPKORequest{
		Name:         imageName,
		Tags:         tags,
		Config:       apkoBase64,
		Readme:       "Replicated SDK container image",
		RegistryURLs: registryURLs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to submit APKO config: %w", err)
	}

	fmt.Printf("SecureBuild: Successfully submitted APKO config (Custom Image ID: %s)\n", response.CustomImageID)

	// 7. Note: SecureBuild automatically queues build jobs after APKO submission
	// External registries already configured at team level, no per-image configuration needed
	imageRef := fmt.Sprintf("cve0.io/replicated-sdk:%s", version)
	fmt.Printf("SecureBuild: Build queued. Image will be available at: %s\n", imageRef)
	fmt.Printf("SecureBuild: Also pushing to external registries: %v\n", registryURLs)

	return imageRef, nil
}

// BuildDevSecureBuild builds development images using SecureBuild
func (m *ReplicatedSdk) BuildDevSecureBuild(ctx context.Context,
	// +defaultPath="/"
	source *dagger.Directory, // Source code directory containing the application to build
	version string, // Version tag for the built image (e.g., "v1.0.0")
	opServiceAccount *dagger.Secret, // 1Password service account secret for authentication
) (string, error) {
	// 1. Get SecureBuild client
	client, err := m.GetSecureBuildClient(ctx, opServiceAccount, SecureBuildEnvDev)
	if err != nil {
		return "", fmt.Errorf("failed to create SecureBuild client: %w", err)
	}

	// 2. Ensure team has ttl.sh external registry configured (team-level, persists across builds)
	fmt.Printf("SecureBuild: Ensuring ttl.sh external registry is configured for team...\n")
	err = client.EnsureExternalRegistry(ctx, "ttl.sh", "replicated", dag.SetSecret("ttl-password", "nopass")) // ttl.sh with placeholder password
	if err != nil {
		return "", fmt.Errorf("failed to configure ttl.sh external registry: %w", err)
	}

	// 3. Generate apko.yaml config for development
	apkoContent, err := m.generateAPKOConfig(ctx, source, version)
	if err != nil {
		return "", fmt.Errorf("failed to generate APKO config: %w", err)
	}

	// Debug: Print the APKO config being sent
	fmt.Printf("=== APKO CONFIG BEING SENT TO SECUREBUILD ===\n%s\n=== END APKO CONFIG ===\n", apkoContent)

	apkoBase64 := base64.StdEncoding.EncodeToString([]byte(apkoContent))

	// 4. Use exact TTL.sh registry URL to match Chainguard pipeline
	ttlRegistryURL := "ttl.sh/replicated"
	
	// 5. Submit APKO configuration with registry URLs (SecureBuild will push to cve0.io + external registries)
	fmt.Printf("SecureBuild: Submitting dev APKO config for %s\n", version)
	response, err := client.SubmitCustomAPKO(ctx, CustomAPKORequest{
		Name:         "replicated-sdk",
		Tags:         []string{version},
		Config:       apkoBase64,
		Readme:       fmt.Sprintf("Replicated SDK development build %s", version),
		RegistryURLs: []string{ttlRegistryURL}, // Only external registries, cve0.io is automatic
	})
	if err != nil {
		return "", fmt.Errorf("failed to submit dev APKO config: %w", err)
	}

	// Dev builds available on both registries
	imageRef := fmt.Sprintf("cve0.io/replicated-sdk:%s", version)
	ttlImageRef := fmt.Sprintf("%s/replicated-sdk:%s", ttlRegistryURL, version)
	fmt.Printf("SecureBuild: Dev build submitted (Custom Image ID: %s)\n", response.CustomImageID)
	fmt.Printf("SecureBuild: Image will be available at: %s\n", imageRef)
	fmt.Printf("SecureBuild: Image will also be available at: %s\n", ttlImageRef)

	return ttlImageRef, nil
}
