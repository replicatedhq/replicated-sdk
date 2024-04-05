package report

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func InjectInstanceDataPayload(reqPayload map[string]interface{}, instanceData *types.InstanceData) error {
	payload, err := GetInstanceDataPayload(instanceData)
	if err != nil {
		return errors.Wrap(err, "failed to get instance data payload")
	}

	for key, value := range payload {
		reqPayload[key] = value
	}

	return nil
}

func GetInstanceDataPayload(instanceData *types.InstanceData) (map[string]interface{}, error) {
	payload := make(map[string]interface{})

	if instanceData == nil {
		return payload, nil
	}

	// only include resource states if they have been initialized
	if instanceData.ResourceStates != nil {
		marshalledRS, err := json.Marshal(instanceData.ResourceStates)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal resource states")
		}
		payload["resource_states"] = string(marshalledRS)
	}

	return payload, nil
}

func InjectInstanceDataHeaders(req *http.Request, instanceData *types.InstanceData) {
	headers := GetInstanceDataHeaders(instanceData)

	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func GetInstanceDataHeaders(instanceData *types.InstanceData) map[string]string {
	headers := make(map[string]string)

	if instanceData == nil {
		return headers
	}

	headers["X-Replicated-K8sVersion"] = instanceData.K8sVersion
	headers["X-Replicated-ClusterID"] = instanceData.ClusterID
	headers["X-Replicated-InstanceID"] = instanceData.InstanceID

	// only include app status related information if it's been initialized
	if instanceData.AppStatus != "" {
		headers["X-Replicated-AppStatus"] = instanceData.AppStatus
	}

	if instanceData.ChannelID != "" {
		headers["X-Replicated-DownstreamChannelID"] = instanceData.ChannelID
	} else if instanceData.ChannelName != "" {
		headers["X-Replicated-DownstreamChannelName"] = instanceData.ChannelName
	}

	headers["X-Replicated-DownstreamChannelSequence"] = strconv.FormatInt(instanceData.ChannelSequence, 10)

	if instanceData.K8sDistribution != "" {
		headers["X-Replicated-K8sDistribution"] = instanceData.K8sDistribution
	}

	if !instanceData.Tags.IsEmpty() {
		b64, err := instanceData.Tags.MarshalBase64()
		if err != nil {
			logger.Errorf("Failed to base64 encode instance tags into headers: %v: %v", instanceData.Tags, err)
		} else {
			headers["X-Replicated-InstanceTagData"] = string(b64)
		}
	}

	return headers
}

func canReport(clientset kubernetes.Interface, namespace string, license *kotsv1beta1.License) (bool, error) {
	if util.IsDevEnv() && !util.IsDevLicense(license) {
		// don't send reports from our dev env to our production services even if this is a production license
		return false, nil
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), util.GetReplicatedDeploymentName(), metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrap(err, "failed to get replicated deployment")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), os.Getenv("REPLICATED_POD_NAME"), metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrap(err, "failed to get replicated pod")
	}

	var podRevision int
	for _, owner := range pod.ObjectMeta.OwnerReferences {
		if owner.APIVersion != "apps/v1" || owner.Kind != "ReplicaSet" {
			continue
		}

		replicaSet, err := clientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), owner.Name, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "failed to get replicaset %s", owner.Name)
		}

		revision, ok := replicaSet.Annotations["deployment.kubernetes.io/revision"]
		if !ok {
			continue
		}

		parsed, err := strconv.Atoi(revision)
		if err != nil {
			return false, errors.Wrapf(err, "failed to parse revision annotation for replicaset %s", replicaSet.Name)
		}
		podRevision = parsed
	}

	var deploymentRevision int
	if drv, ok := deployment.Annotations["deployment.kubernetes.io/revision"]; ok {
		parsed, err := strconv.Atoi(drv)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse revision annotation for deployment")
		}
		deploymentRevision = parsed
	}

	if podRevision != deploymentRevision {
		// don't report from sdk instances that are not associated with the current deployment revision.
		// this can happen when a rolling update of the replicated deployment is in progress and the pod is terminating.
		logger.Infof("not reporting from sdk instance with deployment reversion (%d) because a newer deployment reversion (%d) was found", podRevision, deploymentRevision)
		return false, nil
	}

	return true, nil
}
