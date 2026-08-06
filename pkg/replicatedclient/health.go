package replicatedclient

import "context"

// Healthz returns the health check response including the server version.
func (c *Client) Healthz(ctx context.Context) (*HealthzResponse, error) {
	var resp HealthzResponse
	if err := c.doGet(ctx, "/healthz", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
