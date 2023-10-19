package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func GetLegacyReplicatedConfigMapName() string {
	return "replicated-sdk"
}

func GetReplicatedSecretName() string {
	if sn := os.Getenv("REPLICATED_SECRET_NAME"); sn != "" {
		return sn
	}
	return "replicated"
}

func GetReplicatedDeploymentName() string {
	if dn := os.Getenv("REPLICATED_DEPLOYMENT_NAME"); dn != "" {
		return dn
	}
	return "replicated"
}

func GetReplicatedDeploymentUID(clientset kubernetes.Interface, namespace string) (apimachinerytypes.UID, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), GetReplicatedDeploymentName(), metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get replicated deployment")
	}

	return deployment.ObjectMeta.UID, nil
}

func GetReplicatedAndAppIDs(clientset kubernetes.Interface, namespace string) (string, string, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), GetLegacyReplicatedConfigMapName(), metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return "", "", errors.Wrap(err, "failed to get replicated-sdk configmap")
	}

	replicatedID := ""
	appID := ""

	if kuberneteserrors.IsNotFound(err) {
		uid, err := GetReplicatedDeploymentUID(clientset, namespace)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to get replicated deployment uid")
		}
		replicatedID = string(uid)
		appID = string(uid)
	} else {
		replicatedID = cm.Data["replicated-sdk-id"]
		appID = cm.Data["app-id"]
	}

	return replicatedID, appID, nil
}

func WarnOnOutdatedReplicatedVersion() error {
	currSemver, err := semver.ParseTolerant(buildversion.Version())
	if err != nil {
		logger.Infof("Not checking for outdated Replicated version because the current version (%s) is not a valid semver", buildversion.Version())
		return nil
	}

	latestVersion, err := getLatestReplicatedVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get latest replicated version")
	}

	latestSemver, err := semver.ParseTolerant(latestVersion)
	if err != nil {
		return errors.Wrap(err, "failed to parse latest replicated version")
	}

	if currSemver.LT(latestSemver) {
		minWidth := 0
		tabWidth := 0
		padding := 0
		padChar := byte('!')

		w := tabwriter.NewWriter(os.Stderr, minWidth, tabWidth, padding, padChar, tabwriter.TabIndent)
		defer w.Flush()

		fmtColumns := "%s\t%s\t%s\n"
		fmt.Fprintf(w, fmtColumns, "", "", "")
		fmt.Fprintf(w, fmtColumns, "!", "", "!")
		fmt.Fprintf(w, fmtColumns, "!", fmt.Sprintf(" You are running an outdated version of Replicated (%s). The latest version is %s. ", buildversion.Version(), latestVersion), "!")
		fmt.Fprintf(w, fmtColumns, "!", "", "!")
		fmt.Fprintf(w, fmtColumns, "", "", "")
	}

	return nil
}

func getLatestReplicatedVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/replicatedhq/replicated-sdk/tags")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve tags: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	type GitHubTag struct {
		Name string `json:"name"`
	}
	var tags []GitHubTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response body")
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	return tags[0].Name, nil
}
