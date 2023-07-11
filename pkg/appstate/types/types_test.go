package types

import (
	"testing"
)

func TestMinState(t *testing.T) {
	tests := []struct {
		name string
		ss   []State
		want State
	}{
		{
			name: "ready",
			ss:   []State{StateReady, StateReady},
			want: StateReady,
		},
		{
			name: "updating",
			ss:   []State{StateUpdating, StateReady},
			want: StateUpdating,
		},
		{
			name: "degraded",
			ss:   []State{StateReady, StateDegraded, StateUpdating, StateReady},
			want: StateDegraded,
		},
		{
			name: "unavailable",
			ss:   []State{StateUnavailable, StateDegraded, StateUpdating, StateReady},
			want: StateUnavailable,
		},
		{
			name: "missing",
			ss:   []State{StateUnavailable, StateDegraded, StateMissing, StateUpdating, StateReady},
			want: StateMissing,
		},
		{
			name: "none",
			ss:   []State{},
			want: StateMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MinState(tt.ss...); got != tt.want {
				t.Errorf("MinState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetState(t *testing.T) {
	tests := []struct {
		name           string
		resourceStates []ResourceState
		want           State
	}{
		{
			name: "ready",
			resourceStates: []ResourceState{
				{
					Kind:      "Deployment",
					Name:      "ready-1",
					Namespace: "default",
					State:     StateReady,
				},
				{
					Kind:      "Deployment",
					Name:      "ready-2",
					Namespace: "default",
					State:     StateReady,
				},
			},
			want: StateReady,
		},
		{
			name: "updating",
			resourceStates: []ResourceState{
				{
					Kind:      "Deployment",
					Name:      "updating-1",
					Namespace: "default",
					State:     StateUpdating,
				},
				{
					Kind:      "Deployment",
					Name:      "ready-1",
					Namespace: "default",
					State:     StateReady,
				},
			},
			want: StateUpdating,
		},
		{
			name: "degraded",
			resourceStates: []ResourceState{
				{
					Kind:      "Deployment",
					Name:      "ready-1",
					Namespace: "default",
					State:     StateReady,
				},
				{
					Kind:      "Deployment",
					Name:      "degraded-1",
					Namespace: "default",
					State:     StateDegraded,
				},
				{
					Kind:      "Deployment",
					Name:      "updating-1",
					Namespace: "default",
					State:     StateUpdating,
				},
			},
			want: StateDegraded,
		},
		{
			name: "unavailable",
			resourceStates: []ResourceState{
				{
					Kind:      "Deployment",
					Name:      "ready-1",
					Namespace: "default",
					State:     StateReady,
				},
				{
					Kind:      "Deployment",
					Name:      "degraded-1",
					Namespace: "default",
					State:     StateDegraded,
				},
				{
					Kind:      "Deployment",
					Name:      "unavailable-1",
					Namespace: "default",
					State:     StateUnavailable,
				},
				{
					Kind:      "Deployment",
					Name:      "updating-1",
					Namespace: "default",
					State:     StateUpdating,
				},
			},
			want: StateUnavailable,
		},
		{
			name: "missing",
			resourceStates: []ResourceState{
				{
					Kind:      "Deployment",
					Name:      "ready-1",
					Namespace: "default",
					State:     StateReady,
				},
				{
					Kind:      "Deployment",
					Name:      "degraded-1",
					Namespace: "default",
					State:     StateDegraded,
				},
				{
					Kind:      "Deployment",
					Name:      "missing-1",
					Namespace: "default",
					State:     StateMissing,
				},
				{
					Kind:      "Deployment",
					Name:      "unavailable-1",
					Namespace: "default",
					State:     StateUnavailable,
				},
				{
					Kind:      "Deployment",
					Name:      "updating-1",
					Namespace: "default",
					State:     StateUpdating,
				},
			},
			want: StateMissing,
		},
		{
			name:           "none",
			resourceStates: []ResourceState{},
			want:           StateMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetState(tt.resourceStates); got != tt.want {
				t.Errorf("GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}
