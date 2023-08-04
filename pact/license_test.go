package pact

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/pact-foundation/pact-go/dsl"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/license"
)

func TestGetLatestLicense(t *testing.T) {
	sdkCustomerLicenseString := `apiVersion: kots.io/v1beta1
kind: License
metadata:
  name: sdklicenseappcustomer0
spec:
  licenseID: sdk-license-customer-0-license
  licenseType: trial
  customerName: SDK License App Customer 0
  appSlug: sdk-license-app
  channelID: sdk-license-app-nightly
  channelName: Nightly
  licenseSequence: 2
  endpoint: http://replicated-app:3000
  entitlements:
    expires_at:
      title: Expiration
      description: License Expiration
      value: '2050-01-01T01:23:46Z'
      valueType: String
      signature: {}
  isKotsInstallEnabled: true
`

	sdkCustomerLicense, err := license.LoadLicenseFromBytes([]byte(sdkCustomerLicenseString))
	if err != nil {
		t.Fatalf("failed to load license from bytes: %v", err)
	}

	type args struct {
		license  *v1beta1.License
		endpoint string
	}
	tests := []struct {
		name            string
		args            args
		pactInteraction func()
		want            *license.LicenseData
		wantErr         bool
	}{
		{
			name: "successful license sync",
			args: args{
				license: &v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-license-customer-0-license",
						AppSlug:   "sdk-license-app",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				},
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists, is not archived, and app exists").
					UponReceiving("A request to get the latest license").
					WithRequest(dsl.Request{
						Method: http.MethodGet,
						Headers: dsl.MapMatcher{
							"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-license-customer-0-license", "sdk-license-customer-0-license"))))),
						},
						Path: dsl.String(fmt.Sprintf("/license/%s", "sdk-license-app")),
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusOK,
						Body:   dsl.Term(sdkCustomerLicenseString, sdkCustomerLicenseString), // can't exact match because the signature changes
					})
			},
			want: &license.LicenseData{
				LicenseBytes: []byte(sdkCustomerLicenseString),
				License:      sdkCustomerLicense,
			},
			wantErr: false,
		},
		{
			name: "no license found",
			args: args{
				license: &v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "not-a-customer-license",
						AppSlug:   "sdk-license-app",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				},
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License does not exist and app exists").
					UponReceiving("A request to get the latest license").
					WithRequest(dsl.Request{
						Method: http.MethodGet,
						Headers: dsl.MapMatcher{
							"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "not-a-customer-license", "not-a-customer-license"))))),
						},
						Path: dsl.String(fmt.Sprintf("/license/%s", "sdk-license-app")),
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusUnauthorized,
					})
			},
			wantErr: true,
		},
		{
			name: "no app found",
			args: args{
				license: &v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-license-customer-0-license",
						AppSlug:   "not-an-app",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				},
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists, is not archived, and app does not exist").
					UponReceiving("A request to get the latest license").
					WithRequest(dsl.Request{
						Method: http.MethodGet,
						Headers: dsl.MapMatcher{
							"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-license-customer-0-license", "sdk-license-customer-0-license"))))),
						},
						Path: dsl.String(fmt.Sprintf("/license/%s", "not-an-app")),
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusUnauthorized,
					})
			},
			wantErr: true,
		},
		{
			name: "license is not for this app",
			args: args{
				license: &v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-license-customer-0-license",
						AppSlug:   "sdk-heartbeat-app",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				},
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists, is not archived, app exists, but it's the wrong app").
					Given("App exists").
					UponReceiving("A request to get the latest license").
					WithRequest(dsl.Request{
						Method: http.MethodGet,
						Headers: dsl.MapMatcher{
							"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-license-customer-0-license", "sdk-license-customer-0-license"))))),
						},
						Path: dsl.String(fmt.Sprintf("/license/%s", "sdk-heartbeat-app")),
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusUnauthorized,
					})
			},
			wantErr: true,
		},
		{
			name: "license is archived",
			args: args{
				license: &v1beta1.License{
					Spec: v1beta1.LicenseSpec{
						LicenseID: "sdk-license-customer-archived-license",
						AppSlug:   "sdk-license-app",
						Endpoint:  fmt.Sprintf("http://%s:%d", pact.Host, pact.Server.Port),
					},
				},
			},
			pactInteraction: func() {
				pact.
					AddInteraction().
					Given("License exists, but is archived, and app exists").
					UponReceiving("A request to get the latest license").
					WithRequest(dsl.Request{
						Method: http.MethodGet,
						Headers: dsl.MapMatcher{
							"User-Agent":    dsl.String("Replicated-SDK/v0.0.0-unknown"),
							"Authorization": dsl.String(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "sdk-license-customer-archived-license", "sdk-license-customer-archived-license"))))),
						},
						Path: dsl.String(fmt.Sprintf("/license/%s", "sdk-license-app")),
					}).
					WillRespondWith(dsl.Response{
						Status: http.StatusForbidden,
					})
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pactInteraction()
			if err := pact.Verify(func() error {
				got, err := license.GetLatestLicense(tt.args.license, tt.args.endpoint)
				if (err != nil) != tt.wantErr {
					t.Errorf("SendAppHeartbeat() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("SendAppHeartbeat() got = %v, want %v", got, tt.want)
				}
				return nil
			}); err != nil {
				t.Fatalf("Error on Verify: %v", err)
			}
		})
	}
}
