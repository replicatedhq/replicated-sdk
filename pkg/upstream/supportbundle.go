package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type SupportBundleUploadURL struct {
	BundleID  string `json:"bundle_id"`
	UploadURL string `json:"upload_url"`
}

type markUploadedRequest struct {
	ChannelID string `json:"channel_id"`
}

type markUploadedResponse struct {
	Slug string `json:"slug"`
}

func GetSupportBundleUploadURL(sdkStore store.Store) (*SupportBundleUploadURL, error) {
	wrapper := sdkStore.GetLicense()

	endpoint := sdkStore.GetReplicatedAppEndpoint()
	if endpoint == "" {
		endpoint = wrapper.GetEndpoint()
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse endpoint")
	}

	hostname := u.Hostname()
	if u.Port() != "" {
		hostname = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
	}

	reqURL := fmt.Sprintf("%s://%s/v3/supportbundle/upload-url", u.Scheme, hostname)

	req, err := util.NewRequest("POST", reqURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.SetBasicAuth(wrapper.GetLicenseID(), wrapper.GetLicenseID())

	instanceData := report.GetInstanceData(sdkStore)
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := util.HttpClient().Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
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
		return nil, errors.Errorf("unexpected status code from upload url request: %d", resp.StatusCode)
	}

	var uploadURL SupportBundleUploadURL
	if err := json.Unmarshal(body, &uploadURL); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal upload url response")
	}

	return &uploadURL, nil
}

func UploadToS3(uploadURL string, body io.Reader, contentLength int64) error {
	req, err := http.NewRequest("PUT", uploadURL, body)
	if err != nil {
		return errors.Wrap(err, "failed to create S3 upload request")
	}

	req.ContentLength = contentLength

	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to upload to S3")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		if len(respBody) > 0 {
			return errors.Errorf("failed to upload to S3: %s", string(respBody))
		}
		return errors.Errorf("failed to upload to S3: status %d", resp.StatusCode)
	}

	return nil
}

func MarkSupportBundleUploaded(sdkStore store.Store, bundleID string) (string, error) {
	wrapper := sdkStore.GetLicense()

	endpoint := sdkStore.GetReplicatedAppEndpoint()
	if endpoint == "" {
		endpoint = wrapper.GetEndpoint()
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse endpoint")
	}

	hostname := u.Hostname()
	if u.Port() != "" {
		hostname = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
	}

	reqURL := fmt.Sprintf("%s://%s/v3/supportbundle/%s/uploaded", u.Scheme, hostname, bundleID)

	payload := markUploadedRequest{
		ChannelID: sdkStore.GetChannelID(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal request payload")
	}

	req, err := util.NewRequest("POST", reqURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	req.SetBasicAuth(wrapper.GetLicenseID(), wrapper.GetLicenseID())
	req.Header.Set("Content-Type", "application/json")

	instanceData := report.GetInstanceData(sdkStore)
	report.InjectInstanceDataHeaders(req, instanceData)

	resp, err := util.HttpClient().Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		if len(body) > 0 {
			return "", util.ActionableError{Message: string(body)}
		}
		return "", errors.Errorf("unexpected status code from mark uploaded request: %d", resp.StatusCode)
	}

	var uploadedResp markUploadedResponse
	if err := json.Unmarshal(body, &uploadedResp); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal mark uploaded response")
	}

	return uploadedResp.Slug, nil
}
