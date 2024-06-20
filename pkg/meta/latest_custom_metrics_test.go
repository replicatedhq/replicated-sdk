package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mergeCustomAppMetrics(tst *testing.T) {
	tests := []struct {
		name            string
		existingMetrics map[string]interface{}
		inboundMetrics  map[string]interface{}
		overwrite       bool
		assertFn        func(*testing.T, map[string]interface{})
	}{
		{
			name:            "should return empty if both are empty",
			existingMetrics: map[string]interface{}{},
			inboundMetrics:  map[string]interface{}{},
			overwrite:       false,
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				assert.NotNil(t, actual)
				assert.Empty(t, actual)
			},
		},
		{
			name:            "should tolerate nil value on existingMetrics",
			existingMetrics: nil,
			inboundMetrics:  map[string]interface{}{"numProjects": 10},
			overwrite:       false,
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				expected := map[string]interface{}{"numProjects": 10}
				assert.Equal(t, expected, actual)
			},
		},
		{
			name:            "should tolerate nil value on inboundMetrics",
			existingMetrics: map[string]interface{}{"numProjects": 10},
			inboundMetrics:  nil,
			overwrite:       false,
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				expected := map[string]interface{}{"numProjects": 10}
				assert.Equal(t, expected, actual)
			},
		},
		{
			name:            "should return inboundMetrics when overwrite is true",
			existingMetrics: map[string]interface{}{"numProjects": 10, "token": "1234"},
			inboundMetrics:  map[string]interface{}{"newProjects": 100, "newToken": 10},
			overwrite:       true, // overwrites existing metric data with inbound metrics data
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				expected := map[string]interface{}{"newProjects": 100, "newToken": 10}
				assert.Equal(t, expected, actual)
			},
		},
		{
			name:            "should return merged data when overwrite is false",
			existingMetrics: map[string]interface{}{"numProjects": 10, "token": "1234"},
			inboundMetrics:  map[string]interface{}{"numProjects": 66666, "numPeople": 100},
			overwrite:       false,
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				expected := map[string]interface{}{"numPeople": 100, "numProjects": 66666, "token": "1234"}
				assert.Equal(t, expected, actual)
			},
		},
		{
			name:            "should delete existing metric key when the corresponding inboundMetrics value is nil",
			existingMetrics: map[string]interface{}{"numProjects": 10, "token": "1234"},
			inboundMetrics:  map[string]interface{}{"numProjects": nil}, // delete numProjects
			overwrite:       false,
			assertFn: func(t *testing.T, actual map[string]interface{}) {
				expected := map[string]interface{}{"token": "1234"}
				assert.Equal(t, expected, actual)
			},
		},
	}

	for _, tt := range tests {
		m := mergeCustomAppMetrics(tt.existingMetrics, tt.inboundMetrics, tt.overwrite)
		tt.assertFn(tst, m)
	}
}
