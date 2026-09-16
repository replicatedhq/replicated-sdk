package replicatedclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("http://localhost:3000")
	if c.baseURL != "http://localhost:3000" {
		t.Fatalf("expected baseURL http://localhost:3000, got %s", c.baseURL)
	}
	if c.licenseID != "" {
		t.Fatal("expected empty licenseID")
	}
}

func TestNew_TrailingSlash(t *testing.T) {
	c := New("http://localhost:3000/")
	if c.baseURL != "http://localhost:3000" {
		t.Fatalf("expected trailing slash stripped, got %s", c.baseURL)
	}
}

func TestWithLicenseID(t *testing.T) {
	c := New("http://localhost:3000", WithLicenseID("test-license-id"))
	if c.licenseID != "test-license-id" {
		t.Fatalf("expected licenseID test-license-id, got %s", c.licenseID)
	}
}

func TestWithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := New("http://localhost:3000", WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Fatal("expected custom http client")
	}
}

func TestAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthzResponse{Version: "1.0.0"})
	}))
	defer srv.Close()

	c := New(srv.URL, WithLicenseID("my-license"))
	_, err := c.Healthz(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "my-license" {
		t.Fatalf("expected Authorization header 'my-license', got %q", gotAuth)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("something went wrong"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Healthz(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", apiErr.StatusCode)
	}
	if apiErr.Body != "something went wrong" {
		t.Fatalf("expected body 'something went wrong', got %q", apiErr.Body)
	}
}

// --- Healthz ---

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("expected path /healthz, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthzResponse{Version: "1.2.3"})
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Healthz(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %s", resp.Version)
	}
}

// --- License ---

func TestGetLicenseInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/license/info" {
			t.Fatalf("expected path /api/v1/license/info, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LicenseInfo{
			LicenseID:    "lic-123",
			AppSlug:      "my-app",
			CustomerName: "Acme Corp",
			LicenseType:  "prod",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	info, err := c.GetLicenseInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.LicenseID != "lic-123" {
		t.Fatalf("expected licenseID lic-123, got %s", info.LicenseID)
	}
	if info.CustomerName != "Acme Corp" {
		t.Fatalf("expected customerName Acme Corp, got %s", info.CustomerName)
	}
}

func TestGetLicenseFields(t *testing.T) {
	expected := LicenseFields{
		"seat_count": {
			Name:      "seat_count",
			Title:     "Seat Count",
			Value:     float64(50),
			ValueType: "Integer",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/license/fields" {
			t.Fatalf("expected path /api/v1/license/fields, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := New(srv.URL)
	fields, err := c.GetLicenseFields(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields["seat_count"].Title != "Seat Count" {
		t.Fatalf("expected title Seat Count, got %s", fields["seat_count"].Title)
	}
}

func TestGetLicenseField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/license/fields/seat_count" {
			t.Fatalf("expected path /api/v1/license/fields/seat_count, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LicenseField{
			Name:      "seat_count",
			Title:     "Seat Count",
			Value:     float64(50),
			ValueType: "Integer",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	field, err := c.GetLicenseField(context.Background(), "seat_count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "seat_count" {
		t.Fatalf("expected name seat_count, got %s", field.Name)
	}
}

func TestGetLicenseField_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`"license field \"missing\" not found"`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetLicenseField(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", apiErr.StatusCode)
	}
}

// --- App ---

func TestGetAppInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/info" {
			t.Fatalf("expected path /api/v1/app/info, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AppInfo{
			InstanceID:  "inst-1",
			AppSlug:     "my-app",
			AppName:     "My App",
			AppStatus:   StateReady,
			ChannelName: "Stable",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	info, err := c.GetAppInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.InstanceID != "inst-1" {
		t.Fatalf("expected instanceID inst-1, got %s", info.InstanceID)
	}
	if info.AppStatus != StateReady {
		t.Fatalf("expected status ready, got %s", info.AppStatus)
	}
}

func TestGetAppStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/status" {
			t.Fatalf("expected path /api/v1/app/status, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AppStatusResponse{
			AppStatus: AppStatus{
				AppSlug: "my-app",
				State:   StateReady,
				ResourceStates: []ResourceState{
					{Kind: "Deployment", Name: "web", Namespace: "default", State: StateReady},
				},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	status, err := c.GetAppStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.AppStatus.State != StateReady {
		t.Fatalf("expected state ready, got %s", status.AppStatus.State)
	}
	if len(status.AppStatus.ResourceStates) != 1 {
		t.Fatalf("expected 1 resource state, got %d", len(status.AppStatus.ResourceStates))
	}
}

func TestGetAppUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/updates" {
			t.Fatalf("expected path /api/v1/app/updates, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ChannelRelease{
			{VersionLabel: "2.0.0", ReleaseNotes: "Major update"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	updates, err := c.GetAppUpdates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].VersionLabel != "2.0.0" {
		t.Fatalf("expected versionLabel 2.0.0, got %s", updates[0].VersionLabel)
	}
}

func TestGetAppHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/history" {
			t.Fatalf("expected path /api/v1/app/history, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AppHistoryResponse{
			Releases: []Release{
				{VersionLabel: "1.0.0", DeployedAt: "2025-01-01T00:00:00Z"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	history, err := c.GetAppHistory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history.Releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(history.Releases))
	}
	if history.Releases[0].VersionLabel != "1.0.0" {
		t.Fatalf("expected versionLabel 1.0.0, got %s", history.Releases[0].VersionLabel)
	}
}

func TestSendCustomAppMetrics(t *testing.T) {
	var gotMethod string
	var gotBody SendCustomAppMetricsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/custom-metrics" {
			t.Fatalf("expected path /api/v1/app/custom-metrics, got %s", r.URL.Path)
		}
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode("")
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.SendCustomAppMetrics(context.Background(), CustomAppMetricsData{
		"active_users": float64(42),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotBody.Data["active_users"] != float64(42) {
		t.Fatalf("expected active_users=42, got %v", gotBody.Data["active_users"])
	}
}

func TestUpdateCustomAppMetrics(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode("")
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.UpdateCustomAppMetrics(context.Background(), CustomAppMetricsData{
		"active_users": float64(50),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("expected PATCH, got %s", gotMethod)
	}
}

func TestDeleteCustomAppMetricsKey(t *testing.T) {
	var gotPath string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.DeleteCustomAppMetricsKey(context.Background(), "old_metric")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/api/v1/app/custom-metrics/old_metric" {
		t.Fatalf("expected path /api/v1/app/custom-metrics/old_metric, got %s", gotPath)
	}
}

func TestSendAppInstanceTags(t *testing.T) {
	var gotBody SendAppInstanceTagsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/instance-tags" {
			t.Fatalf("expected path /api/v1/app/instance-tags, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode("")
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.SendAppInstanceTags(context.Background(), InstanceTagData{
		Tags: map[string]string{"env": "production"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody.Data.Tags["env"] != "production" {
		t.Fatalf("expected env=production, got %s", gotBody.Data.Tags["env"])
	}
}

// --- Integration ---

func TestGetIntegrationStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/integration/status" {
			t.Fatalf("expected path /api/v1/integration/status, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(IntegrationStatusResponse{IsEnabled: true})
	}))
	defer srv.Close()

	c := New(srv.URL)
	status, err := c.GetIntegrationStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsEnabled {
		t.Fatal("expected integration to be enabled")
	}
}

func TestPostIntegrationMockData(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/integration/mock-data" {
			t.Fatalf("expected path /api/v1/integration/mock-data, got %s", r.URL.Path)
		}
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := New(srv.URL)
	mockData := map[string]interface{}{
		"appStatus": "ready",
	}
	err := c.PostIntegrationMockData(context.Background(), mockData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
}

func TestGetIntegrationMockData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/integration/mock-data" {
			t.Fatalf("expected path /api/v1/integration/mock-data, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"appStatus":"ready","version":"v1"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	data, err := c.GetIntegrationMockData(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal raw json: %v", err)
	}
	if parsed["appStatus"] != "ready" {
		t.Fatalf("expected appStatus=ready, got %v", parsed["appStatus"])
	}
}

func TestGetIntegrationMockData_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetIntegrationMockData(context.Background())
	if err == nil {
		t.Fatal("expected error for 403")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", apiErr.StatusCode)
	}
}

// --- Context Cancellation ---

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthzResponse{Version: "1.0.0"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := New(srv.URL)
	_, err := c.Healthz(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
