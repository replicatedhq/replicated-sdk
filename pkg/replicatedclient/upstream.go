package replicatedclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultUpstreamEndpoint = "https://replicated.app"

// UpstreamClient talks directly to the Replicated upstream API (replicated.app)
// without requiring a local SDK service.
type UpstreamClient struct {
	endpoint   string
	licenseID  string
	httpClient *http.Client
}

// UpstreamOption configures an UpstreamClient.
type UpstreamOption func(*UpstreamClient)

// WithEndpoint overrides the default upstream endpoint (https://replicated.app).
func WithEndpoint(endpoint string) UpstreamOption {
	return func(c *UpstreamClient) {
		c.endpoint = strings.TrimRight(endpoint, "/")
	}
}

// WithUpstreamHTTPClient sets a custom http.Client for upstream requests.
func WithUpstreamHTTPClient(hc *http.Client) UpstreamOption {
	return func(c *UpstreamClient) {
		c.httpClient = hc
	}
}

// NewUpstream creates an UpstreamClient that talks directly to the Replicated API.
// The licenseID is used for Basic Auth (licenseID:licenseID).
func NewUpstream(licenseID string, opts ...UpstreamOption) *UpstreamClient {
	c := &UpstreamClient{
		endpoint:   defaultUpstreamEndpoint,
		licenseID:  licenseID,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// doGet performs an authenticated GET request and decodes the JSON response into dest.
func (c *UpstreamClient) doGet(ctx context.Context, path string, dest interface{}) error {
	reqURL := c.endpoint + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("replicated-sdk upstream: create request: %w", err)
	}

	req.SetBasicAuth(c.licenseID, c.licenseID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("replicated-sdk upstream: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("replicated-sdk upstream: decode response: %w", err)
		}
	}
	return nil
}

// GetLicenseFields returns all custom license fields from the upstream API.
func (c *UpstreamClient) GetLicenseFields(ctx context.Context) (LicenseFields, error) {
	var fields LicenseFields
	if err := c.doGet(ctx, "/license/fields", &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// GetLicenseField returns a specific custom license field by name from the upstream API.
func (c *UpstreamClient) GetLicenseField(ctx context.Context, fieldName string) (*LicenseField, error) {
	var field LicenseField
	path := "/license/field/" + url.PathEscape(fieldName)
	if err := c.doGet(ctx, path, &field); err != nil {
		return nil, err
	}
	return &field, nil
}
