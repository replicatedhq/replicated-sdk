package replicatedclient

import (
	"context"
	"net/http"
	"net/url"
)

// GetAppInfo returns the current application information.
func (c *Client) GetAppInfo(ctx context.Context) (*AppInfo, error) {
	var info AppInfo
	if err := c.doGet(ctx, "/api/v1/app/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetAppStatus returns the current application status.
func (c *Client) GetAppStatus(ctx context.Context) (*AppStatusResponse, error) {
	var status AppStatusResponse
	if err := c.doGet(ctx, "/api/v1/app/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetAppUpdates returns the list of available upstream updates.
func (c *Client) GetAppUpdates(ctx context.Context) ([]ChannelRelease, error) {
	var updates []ChannelRelease
	if err := c.doGet(ctx, "/api/v1/app/updates", &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// GetAppHistory returns the deployment history (Helm releases).
func (c *Client) GetAppHistory(ctx context.Context) (*AppHistoryResponse, error) {
	var history AppHistoryResponse
	if err := c.doGet(ctx, "/api/v1/app/history", &history); err != nil {
		return nil, err
	}
	return &history, nil
}

// SendCustomAppMetrics sends (overwrites) custom application metrics.
// Data values must be scalars (string, number, bool).
func (c *Client) SendCustomAppMetrics(ctx context.Context, data CustomAppMetricsData) error {
	req := SendCustomAppMetricsRequest{Data: data}
	return c.doSend(ctx, http.MethodPost, "/api/v1/app/custom-metrics", req)
}

// UpdateCustomAppMetrics merges custom application metrics with existing values.
// Data values must be scalars (string, number, bool).
func (c *Client) UpdateCustomAppMetrics(ctx context.Context, data CustomAppMetricsData) error {
	req := SendCustomAppMetricsRequest{Data: data}
	return c.doSend(ctx, http.MethodPatch, "/api/v1/app/custom-metrics", req)
}

// DeleteCustomAppMetricsKey deletes a specific custom metrics key.
func (c *Client) DeleteCustomAppMetricsKey(ctx context.Context, key string) error {
	path := "/api/v1/app/custom-metrics/" + url.PathEscape(key)
	return c.doSend(ctx, http.MethodDelete, path, nil)
}

// SendAppInstanceTags sends instance tags for the application.
func (c *Client) SendAppInstanceTags(ctx context.Context, data InstanceTagData) error {
	req := SendAppInstanceTagsRequest{Data: data}
	return c.doSend(ctx, http.MethodPost, "/api/v1/app/instance-tags", req)
}
