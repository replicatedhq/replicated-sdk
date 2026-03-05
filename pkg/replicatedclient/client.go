package replicatedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is an HTTP client for the Replicated SDK API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	licenseID  string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithLicenseID sets the license ID sent in the Authorization header.
func WithLicenseID(id string) Option {
	return func(c *Client) {
		c.licenseID = id
	}
}

// New creates a new Replicated SDK client.
// baseURL should include the scheme and host, e.g. "http://localhost:3000".
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError is returned when the server responds with a non-2xx status code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("replicated-sdk: HTTP %d: %s", e.StatusCode, e.Body)
}

// doRequest builds and executes an HTTP request, returning the response.
// The caller is responsible for closing the response body.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("replicated-sdk: create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.licenseID != "" {
		req.Header.Set("Authorization", c.licenseID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("replicated-sdk: execute request: %w", err)
	}

	return resp, nil
}

// doGet performs a GET request and decodes the JSON response into dest.
func (c *Client) doGet(ctx context.Context, path string, dest interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("replicated-sdk: decode response: %w", err)
		}
	}
	return nil
}

// doSend performs a request with a JSON body and checks for a successful status.
func (c *Client) doSend(ctx context.Context, method, path string, payload interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("replicated-sdk: encode request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}

	return nil
}

// readAPIError reads the response body and returns an *APIError.
func readAPIError(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(bodyBytes)),
	}
}
