package replicatedclient

import (
	"context"
	"net/url"
)

// GetLicenseInfo returns the current license information.
func (c *Client) GetLicenseInfo(ctx context.Context) (*LicenseInfo, error) {
	var info LicenseInfo
	if err := c.doGet(ctx, "/api/v1/license/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetLicenseFields returns all custom license fields.
func (c *Client) GetLicenseFields(ctx context.Context) (LicenseFields, error) {
	var fields LicenseFields
	if err := c.doGet(ctx, "/api/v1/license/fields", &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// GetLicenseField returns a specific custom license field by name.
func (c *Client) GetLicenseField(ctx context.Context, fieldName string) (*LicenseField, error) {
	var field LicenseField
	path := "/api/v1/license/fields/" + url.PathEscape(fieldName)
	if err := c.doGet(ctx, path, &field); err != nil {
		return nil, err
	}
	return &field, nil
}
