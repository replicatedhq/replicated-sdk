package replicatedclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewUpstream(t *testing.T) {
	c := NewUpstream("lic-123")
	if c.endpoint != "https://replicated.app" {
		t.Fatalf("expected default endpoint, got %s", c.endpoint)
	}
	if c.licenseID != "lic-123" {
		t.Fatalf("expected licenseID lic-123, got %s", c.licenseID)
	}
}

func TestNewUpstream_WithEndpoint(t *testing.T) {
	c := NewUpstream("lic-123", WithEndpoint("https://custom.replicated.app/"))
	if c.endpoint != "https://custom.replicated.app" {
		t.Fatalf("expected trailing slash stripped, got %s", c.endpoint)
	}
}

func TestNewUpstream_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := NewUpstream("lic-123", WithUpstreamHTTPClient(custom))
	if c.httpClient != custom {
		t.Fatal("expected custom http client")
	}
}

func TestUpstream_BasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LicenseFields{})
	}))
	defer srv.Close()

	c := NewUpstream("my-license", WithEndpoint(srv.URL))
	_, err := c.GetLicenseFields(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUser != "my-license" {
		t.Fatalf("expected basic auth user 'my-license', got %q", gotUser)
	}
	if gotPass != "my-license" {
		t.Fatalf("expected basic auth pass 'my-license', got %q", gotPass)
	}
}

// --- License Fields ---

func TestUpstream_GetLicenseFields(t *testing.T) {
	expected := LicenseFields{
		"seat_count": {
			Name:      "seat_count",
			Title:     "Seat Count",
			Value:     float64(50),
			ValueType: "Integer",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/fields" {
			t.Fatalf("expected path /license/fields, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := NewUpstream("lic-123", WithEndpoint(srv.URL))
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

func TestUpstream_GetLicenseField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/field/seat_count" {
			t.Fatalf("expected path /license/field/seat_count, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
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

	c := NewUpstream("lic-123", WithEndpoint(srv.URL))
	field, err := c.GetLicenseField(context.Background(), "seat_count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "seat_count" {
		t.Fatalf("expected name seat_count, got %s", field.Name)
	}
	if field.Value != float64(50) {
		t.Fatalf("expected value 50, got %v", field.Value)
	}
}

func TestUpstream_GetLicenseField_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := NewUpstream("lic-123", WithEndpoint(srv.URL))
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

func TestUpstream_GetLicenseFields_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid license"))
	}))
	defer srv.Close()

	c := NewUpstream("bad-license", WithEndpoint(srv.URL))
	_, err := c.GetLicenseFields(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", apiErr.StatusCode)
	}
}

func TestUpstream_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LicenseFields{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewUpstream("lic-123", WithEndpoint(srv.URL))
	_, err := c.GetLicenseFields(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
