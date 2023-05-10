package appstate

import (
	"sort"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
)

func resourceStatesApplyNew(resourceStates types.ResourceStates, resourceState types.ResourceState) (next types.ResourceStates) {
	next = make(types.ResourceStates, len(resourceStates))
	copy(next, resourceStates)

	index := -1
	for i, r := range next {
		if resourceState.Kind == r.Kind &&
			resourceState.Namespace == r.Namespace &&
			resourceState.Name == r.Name {
			index = i
			break
		}
	}

	if index == -1 {
		next = append(next, resourceState)
	} else {
		next[index] = resourceState
	}

	sort.Sort(next)
	return
}
