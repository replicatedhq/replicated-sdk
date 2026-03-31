package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseReplicatedConfig_ReadOnlyMode(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected bool
	}{
		{
			name:     "readOnlyMode true",
			config:   "readOnlyMode: true",
			expected: true,
		},
		{
			name:     "readOnlyMode false",
			config:   "readOnlyMode: false",
			expected: false,
		},
		{
			name:     "readOnlyMode not set defaults to false",
			config:   "appName: test-app",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			rc, err := ParseReplicatedConfig([]byte(tt.config))
			req.NoError(err)
			req.Equal(tt.expected, rc.ReadOnlyMode)
		})
	}
}
