package helm

import (
	"os"
	"strconv"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

func IsHelmManaged() bool {
	return os.Getenv("IS_HELM_MANAGED") == "true"
}

func GetReleaseName() string {
	return os.Getenv("HELM_RELEASE_NAME")
}

func GetReleaseNamespace() string {
	return os.Getenv("HELM_RELEASE_NAMESPACE")
}

func GetReleaseRevision() int {
	hr, _ := strconv.Atoi(os.Getenv("HELM_RELEASE_REVISION"))
	return hr
}

func GetParentChartURL() string {
	return os.Getenv("HELM_PARENT_CHART_URL")
}

func GetHelmDriver() string {
	return os.Getenv("HELM_DRIVER")
}

func GetReleaseHistory() ([]*release.Release, error) {
	client := action.NewHistory(cfg)
	releases, err := client.Run(GetReleaseName())
	if err != nil {
		return nil, errors.Wrap(err, "failed to list releases")
	}

	return releases, nil
}

func GetRelease(releaseName string) (*release.Release, error) {
	client := action.NewGet(cfg)
	releases, err := client.Run(releaseName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get release")
	}

	return releases, nil
}
