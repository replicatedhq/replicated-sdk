package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
)

func FormatChartname(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		return "MISSING"
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func FormatAppVersion(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		return "MISSING"
	}
	return c.AppVersion()
}
