package types

import (
	"time"
)

var (
	StateReady       State = "ready"
	StateUpdating    State = "updating"
	StateDegraded    State = "degraded"
	StateUnavailable State = "unavailable"
	StateMissing     State = "missing"
)

type AppInformersArgs struct {
	AppSlug       string `json:"app_id"`
	Sequence      int64  `json:"sequence"`
	LabelSelector string `json:"label_selector"`
}

type AppStatus struct {
	AppSlug        string         `json:"appSlug"`
	ResourceStates ResourceStates `json:"resourceStates" hash:"set"`
	UpdatedAt      time.Time      `json:"updatedAt" hash:"ignore"`
	State          State          `json:"state"`
	Sequence       int64          `json:"sequence"`
}

type ResourceStates []ResourceState

type ResourceState struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	State     State  `json:"state"`
}

type State string

func GetState(resourceStates []ResourceState) State {
	if len(resourceStates) == 0 {
		return StateMissing
	}
	max := StateReady
	for _, resourceState := range resourceStates {
		max = MinState(max, resourceState.State)
	}
	return max
}

func MinState(ss ...State) (min State) {
	if len(ss) == 0 {
		return StateMissing
	}
	for _, s := range ss {
		if s == StateMissing || min == StateMissing {
			return StateMissing
		} else if s == StateUnavailable || min == StateUnavailable {
			min = StateUnavailable
		} else if s == StateDegraded || min == StateDegraded {
			min = StateDegraded
		} else if s == StateUpdating || min == StateUpdating {
			min = StateUpdating
		} else if s == StateReady || min == StateReady {
			min = StateReady
		}
	}
	return
}

func (a ResourceStates) Len() int {
	return len(a)
}

func (a ResourceStates) Less(i, j int) bool {
	if a[i].Kind < a[j].Kind {
		return true
	}
	if a[i].Name < a[j].Name {
		return true
	}
	if a[i].Namespace < a[j].Namespace {
		return true
	}
	return false
}

func (a ResourceStates) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
