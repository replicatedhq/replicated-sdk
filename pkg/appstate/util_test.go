package appstate

import (
	"reflect"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
)

func TestGenerateStatusInformersForManifest(t *testing.T) {
	type args struct {
		manifest string
	}
	tests := []struct {
		name    string
		args    args
		want    []types.StatusInformerString
		wantErr bool
	}{
		{
			name: "empty manifest",
			args: args{
				manifest: "",
			},
			want:    []types.StatusInformerString{},
			wantErr: false,
		},
		{
			name: "single resource",
			args: args{
				manifest: `apiVersion: v1
kind: Deployment
metadata:
  name: test
  namespace: default
`,
			},
			want: []types.StatusInformerString{"default/deployment/test"},
		},
		{
			name: "multiple resources",
			args: args{
				manifest: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: default
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
  namespace: otherns
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
---
apiVersion: v1
kind: Service
metadata:
  name: test
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test
`,
			},
			want: []types.StatusInformerString{
				"default/deployment/test",
				"otherns/statefulset/test",
				"daemonset/test",
				"service/test",
				"persistentvolumeclaim/test",
				"ingress/test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateStatusInformersForManifest(tt.args.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateStatusInformersForManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateStatusInformersForManifest() = %v, want %v", got, tt.want)
			}
		})
	}
}
