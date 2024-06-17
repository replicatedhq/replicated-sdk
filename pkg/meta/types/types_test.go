package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceTagData(t *testing.T) {
	tests := []struct {
		name     string
		initFn   func(tdata *InstanceTagData)
		assertFn func(t *testing.T, tdata *InstanceTagData)
	}{
		{
			name: "IsEmpty returns true when there are no tags",
			initFn: func(tdata *InstanceTagData) {
				tdata.Tags = map[string]string{}
			},
			assertFn: func(t *testing.T, tdata *InstanceTagData) {
				assert.True(t, tdata.IsEmpty())
			},
		},
		{
			name: "IsEmpty returns false when there is at least one tag",
			initFn: func(tdata *InstanceTagData) {
				tdata.Tags = map[string]string{
					"key": "value",
				}
			},
			assertFn: func(t *testing.T, tdata *InstanceTagData) {
				assert.False(t, tdata.IsEmpty())
			},
		},
		{
			name: "should marshal correctly to base64",
			initFn: func(tdata *InstanceTagData) {
				tdata.Force = true
				tdata.Tags = map[string]string{
					"key": "value",
				}
			},
			assertFn: func(t *testing.T, tdata *InstanceTagData) {
				b, err := tdata.MarshalBase64()
				assert.NoError(t, err)
				assert.Equal(t, "eyJmb3JjZSI6dHJ1ZSwidGFncyI6eyJrZXkiOiJ2YWx1ZSJ9fQ==", string(b))
			},
		},
		{
			name:   "should unmarshal struct correctly from base64",
			initFn: func(tdata *InstanceTagData) {},
			assertFn: func(t *testing.T, tdata *InstanceTagData) {
				err := tdata.UnmarshalBase64([]byte("eyJmb3JjZSI6dHJ1ZSwidGFncyI6eyJrZXkiOiJ2YWx1ZSJ9fQ=="))
				assert.NoError(t, err)

				expected := &InstanceTagData{
					Force: true,
					Tags: map[string]string{
						"key": "value",
					},
				}

				assert.Equal(t, expected, tdata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdata := &InstanceTagData{}
			tt.initFn(tdata)
			tt.assertFn(t, tdata)
		})
	}
}
