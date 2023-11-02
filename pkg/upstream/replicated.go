package upstream

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	types "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

func GetUpdates(sdkStore store.Store, license *kotsv1beta1.License, currentCursor types.ReplicatedCursor) ([]types.ChannelRelease, error) {
	endpoint := sdkStore.GetReplicatedAppEndpoint()
	if endpoint == "" {
		endpoint = license.Spec.Endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse endpoint")
	}

	hostname := u.Hostname()
	if u.Port() != "" {
		hostname = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
	}

	// build the request url query params
	channelSequenceStr := fmt.Sprintf("%d", currentCursor.ChannelSequence)
	if currentCursor.ChannelID != license.Spec.ChannelID {
		// channel has changed, so we need to reset the channel sequence
		channelSequenceStr = ""
	}

	urlValues := url.Values{}
	urlValues.Set("channelSequence", channelSequenceStr)
	urlValues.Add("licenseSequence", fmt.Sprintf("%d", license.Spec.LicenseSequence))
	urlValues.Add("isSemverSupported", "true")
	urlValues.Add("sortOrder", "desc")

	url := fmt.Sprintf("%s://%s/release/%s/pending?%s", u.Scheme, hostname, license.Spec.AppSlug, urlValues.Encode())

	instanceData := report.GetInstanceData(sdkStore)

	// build the request body
	reqPayload := map[string]interface{}{}
	if err := report.InjectInstanceDataPayload(reqPayload, instanceData); err != nil {
		return nil, errors.Wrap(err, "failed to inject instance data payload")
	}
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request payload")
	}

	req, err := util.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID)))))
	req.Header.Set("Content-Type", "application/json")

	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		if len(body) > 0 {
			return nil, util.ActionableError{Message: string(body)}
		}
		return nil, errors.Errorf("unexpected result from get request: %d", resp.StatusCode)
	}

	var channelReleases struct {
		ChannelReleases []types.ChannelRelease `json:"channelReleases"`
	}
	if err := json.Unmarshal(body, &channelReleases); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	return channelReleases.ChannelReleases, nil
}
