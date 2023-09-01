package helm

import (
	"os"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
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
		if errors.Cause(err) == driver.ErrReleaseNotFound {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to list releases")
	}

	return releases, nil
}

func GetRelease(releaseName string) (*release.Release, error) {
	client := action.NewGet(cfg)
	release, err := client.Run(releaseName)
	if err != nil {
		if errors.Cause(err) == driver.ErrReleaseNotFound {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get release")
	}

	return release, nil
}
