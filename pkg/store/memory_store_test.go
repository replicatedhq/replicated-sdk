package store

import (
	"testing"

	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStore_RunningImagesAggregation(t *testing.T) {
	s := &InMemoryStore{}

	// initially empty
	require.Len(t, s.GetRunningImages(), 0)

	// add pod1 in ns1 with two images
	s.SetPodImages("ns1", "pod1", []appstatetypes.ImageInfo{
		{Name: "nginx", SHA: "sha256:a"},
		{Name: "redis", SHA: "sha256:b"},
	})
	got := s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:a"}, got["nginx"])
	require.ElementsMatch(t, []string{"sha256:b"}, got["redis"])

	// add pod2 in ns1 with overlapping image and new one
	s.SetPodImages("ns1", "pod2", []appstatetypes.ImageInfo{
		{Name: "nginx", SHA: "sha256:a"}, // duplicate sha for nginx
		{Name: "postgres", SHA: "sha256:c"},
	})
	got = s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:a"}, got["nginx"]) // still unique
	require.ElementsMatch(t, []string{"sha256:b"}, got["redis"])
	require.ElementsMatch(t, []string{"sha256:c"}, got["postgres"])

	// add pod3 in ns2 with another sha for nginx
	s.SetPodImages("ns2", "pod3", []appstatetypes.ImageInfo{
		{Name: "nginx", SHA: "sha256:d"},
	})
	got = s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:a", "sha256:d"}, got["nginx"]) // multiple shas aggregated

	// delete pod2
	s.DeletePodImages("ns1", "pod2")
	got = s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:a", "sha256:d"}, got["nginx"]) // unchanged for nginx
	require.Nil(t, got["postgres"])                                          // postgres removed

	// delete pod1
	s.DeletePodImages("ns1", "pod1")
	got = s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:d"}, got["nginx"]) // only ns2's sha remains
	require.Nil(t, got["redis"])                                 // removed

	// delete pod3
	s.DeletePodImages("ns2", "pod3")
	got = s.GetRunningImages()
	require.Len(t, got, 0)
}

func TestInMemoryStore_SetPodImages_FiltersByReleaseImages_NameTagOnly(t *testing.T) {
	s := &InMemoryStore{
		releaseImages: []string{
			"nginx:latest@sha256:abcdefg", // allow any digest for nginx:latest
			"busybox:musl",                // allow by tag without digest
		},
	}

	s.SetPodImages("ns1", "pod1", []appstatetypes.ImageInfo{
		{Name: "nginx:latest", SHA: "sha256:111"},
		{Name: "nginx:latest", SHA: "sha256:222"},
		{Name: "busybox:musl", SHA: "sha256:333"},
		{Name: "redis:7", SHA: "sha256:444"}, // not allowed
	})

	got := s.GetRunningImages()
	// both nginx digests should be included since name:tag matches
	require.ElementsMatch(t, []string{"sha256:111", "sha256:222"}, got["nginx:latest"])
	// busybox is included
	require.ElementsMatch(t, []string{"sha256:333"}, got["busybox:musl"])
	// redis should be excluded entirely
	require.Nil(t, got["redis:7"])
}

func TestInMemoryStore_SetPodImages_NoFilterWhenNoReleaseImages(t *testing.T) {
	s := &InMemoryStore{}

	s.SetPodImages("ns1", "pod1", []appstatetypes.ImageInfo{
		{Name: "nginx:1.29-alpine", SHA: "sha256:aaa"},
		{Name: "redis:7", SHA: "sha256:bbb"},
	})

	got := s.GetRunningImages()
	require.ElementsMatch(t, []string{"sha256:aaa"}, got["nginx:1.29-alpine"])
	require.ElementsMatch(t, []string{"sha256:bbb"}, got["redis:7"])
}
