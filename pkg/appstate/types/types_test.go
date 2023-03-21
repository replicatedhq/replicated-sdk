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
