package replicatedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PostIntegrationMockData creates or updates mock data for integration mode.
// body should be a JSON-serializable value matching the mock data schema (v1 or v2).
func (c *Client) PostIntegrationMockData(ctx context.Context, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("replicated-sdk: encode mock data: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/integration/mock-data", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	return nil
}

// GetIntegrationMockData retrieves the current mock data as raw JSON.
// The caller can unmarshal the result into the appropriate mock data version struct.
func (c *Client) GetIntegrationMockData(ctx context.Context) (json.RawMessage, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/integration/mock-data", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("replicated-sdk: read response body: %w", err)
	}
	return json.RawMessage(data), nil
}

// GetIntegrationStatus returns whether integration mode is enabled.
func (c *Client) GetIntegrationStatus(ctx context.Context) (*IntegrationStatusResponse, error) {
	var status IntegrationStatusResponse
	if err := c.doGet(ctx, "/api/v1/integration/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}
