package upstream

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	reporting "github.com/replicatedhq/kots-sdk/pkg/reporting"
	types "github.com/replicatedhq/kots-sdk/pkg/upstream/types"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

func ListPendingChannelReleases(license *kotsv1beta1.License, currentCursor types.ReplicatedCursor) ([]types.ChannelRelease, error) {
	u, err := url.Parse(license.Spec.Endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse endpoint from license")
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

	url := fmt.Sprintf("%s://%s/release/%s/pending?%s", u.Scheme, hostname, license.Spec.AppSlug, urlValues.Encode())

	// build the request body
	reportingInfo := reporting.GetReportingInfo()

	marshalledRS, err := json.Marshal(reportingInfo.ResourceStates)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal resource states")
	}
	reqPayload := map[string]interface{}{
		"resource_states": string(marshalledRS),
	}
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request payload")
	}

	req, err := util.NewRequest("GET", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to call newrequest")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID)))))
	reporting.InjectReportingInfoHeaders(req, reportingInfo)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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
