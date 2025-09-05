package main

import (
	"dagger/replicated-sdk/internal/dagger"
	"strings"
	"time"
)

// CacheBustingExec is a helper function that will add a cache busting env var automatically
// to the container. This is useful when Exec target is a dynamic event acting on an entity outside
// of the container that you absolutely want to re-run every time.
//
// Temporary hack until cache controls are a thing: https://docs.dagger.io/cookbook/#invalidate-cache
func CacheBustingExec(args []string, opts ...dagger.ContainerWithExecOpts) dagger.WithContainerFunc {
	return func(c *dagger.Container) *dagger.Container {
		if c == nil {
			panic("CacheBustingExec requires a container, but container was nil")
		}
		return c.WithEnvVariable("DAGGER_CACHEBUSTER_CBE", time.Now().String()).WithExec(args, opts...)
	}
}

// sanitizeVersionForMelange converts version strings to be compatible with melange/apko packages
// Converts dashes to underscores to follow melange naming conventions
// Examples: "1.6.0-beta.1" → "1_6_0_beta1", "1.6.0-alpha" → "1_6_0_alpha"
func sanitizeVersionForMelange(version string) string {
	v := strings.ReplaceAll(version, "-beta.", "_beta")
	v = strings.ReplaceAll(v, "-alpha.", "_alpha")
	v = strings.ReplaceAll(v, "-", "_") // catch any remaining dashes
	return v
}
