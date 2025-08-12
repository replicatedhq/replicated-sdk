package types

import (
	"errors"
	"regexp"
	"time"
)

var (
	StateReady       State = "ready"
	StateUpdating    State = "updating"
	StateDegraded    State = "degraded"
	StateUnavailable State = "unavailable"
	StateMissing     State = "missing"

	StatusInformerRegexp = regexp.MustCompile(`^(?:([^\/]+)\/)?([^\/]+)\/([^\/]+)$`)
)

type AppInformersArgs struct {
	AppSlug   string
	Sequence  int64
	Informers []StatusInformerString
}

type StatusInformerString string

type StatusInformer struct {
	Kind      string
	Name      string
	Namespace string
}

func (s StatusInformerString) Parse() (i StatusInformer, err error) {
	matches := StatusInformerRegexp.FindStringSubmatch(string(s))
	if len(matches) != 4 {
		err = errors.New("status informer format string incorrect")
		return
	}
	i.Namespace = matches[1]
	i.Kind = matches[2]
	i.Name = matches[3]
	return
}

type AppStatus struct {
	AppSlug        string         `json:"appSlug" yaml:"appSlug"`
	ResourceStates ResourceStates `json:"resourceStates" yaml:"resourceStates" hash:"set"`
	UpdatedAt      time.Time      `json:"updatedAt" yaml:"updatedAt" hash:"ignore"`
	State          State          `json:"state" yaml:"state"`
	Sequence       int64          `json:"sequence" yaml:"sequence"`
}

type ResourceStates []ResourceState

type ResourceState struct {
	Kind      string `json:"kind" yaml:"kind"`
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	State     State  `json:"state" yaml:"state"`
}

type State string

// ImageInfo describes a container image and its content digest
// extracted from a Pod status.
type ImageInfo struct {
	Name string
	SHA  string
}

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
