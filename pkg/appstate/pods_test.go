package appstate

import (
	"testing"

	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/stretchr/testify/assert"
)

func TestExtractImageInfo(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		expected appstatetypes.ImageInfo
	}{
		{
			name:     "image with SHA digest",
			imageRef: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: appstatetypes.ImageInfo{
				Name: "registry.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "image with tag only",
			imageRef: "registry.com/my-app:latest",
			expected: appstatetypes.ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
		{
			name:     "image with multiple @ symbols",
			imageRef: "registry@company.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: appstatetypes.ImageInfo{
				Name: "registry@company.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "simple image name with SHA",
			imageRef: "nginx@sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			expected: appstatetypes.ImageInfo{
				Name: "nginx",
				SHA:  "sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			},
		},
		{
			name:     "image without registry",
			imageRef: "my-app:v1.0.0",
			expected: appstatetypes.ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageInfo(tt.imageRef)
			assert.Equal(t, tt.expected, result)
		})
	}
}
