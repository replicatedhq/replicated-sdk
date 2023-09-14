package pact

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pact-foundation/pact-go/dsl"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

func TestSendCustomApplicationMetrics(t *testing.T) {
	// Happy path only

	channelSequence := int64(1)
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"key1_string":         "val1",
			"key2_int":            5,
			"key3_float":          1.5,
			"key4_numeric_string": "1.6",
		},
	}
	customMetricsData, _ := json.Marshal(data)
	license := &v1beta1.License{
		Spec: v1beta1.LicenseSpec{
			LicenseID: "sdk-license-customer-0-license",
			AppSlug:   "sdk-license-app",
			Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
			ChannelID: "sdk-license-app-nightly",
		},
	}

	clientWriter := httptest.NewRecorder()
	clientRequest := &http.Request{
		Body: io.NopCloser(bytes.NewBuffer(customMetricsData)),
	}

	pactInteraction := func() {
		pact.
			AddInteraction().
			Given("Send valid custom metrics").
			UponReceiving("A request to send custom metrics").
			WithRequest(dsl.Request{
				Method: http.MethodPost,
				Headers: dsl.MapMatcher{
					"User-Agent":                             dsl.String("Replicated-SDK/v0.0.0-unknown"),
					"Authorization":                          dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID))))),
					"X-Replicated-DownstreamChannelID":       dsl.String(license.Spec.ChannelID),
					"X-Replicated-DownstreamChannelSequence": dsl.String(fmt.Sprintf("%d", channelSequence)),
				},
				Path: dsl.String("/application/custom-metrics"),
				Body: data,
			}).
			WillRespondWith(dsl.Response{
				Status: http.StatusOK,
			})
	}
	t.Run("Send valid custom metrics", func(t *testing.T) {
		pactInteraction()

		storeOptions := store.InitInMemoryStoreOptions{
			License:               license,
			LicenseFields:         nil,
			ReplicatedAppEndpoint: license.Spec.Endpoint,
			ChannelID:             license.Spec.ChannelID,
			ChannelSequence:       channelSequence,
		}
		if err := store.InitInMemory(storeOptions); err != nil {
			t.Fatalf("Error on InitInMemory: %v", err)
		}
		defer store.SetStore(nil)

		if err := pact.Verify(func() error {
			handlers.SendCustomApplicationMetrics(clientWriter, clientRequest)
			if clientWriter.Code != http.StatusOK {
				return fmt.Errorf("expected status code %d but got %d", http.StatusOK, clientWriter.Code)
			}
			return nil
		}); err != nil {
			t.Fatalf("Error on Verify: %v", err)
		}
	})
}
