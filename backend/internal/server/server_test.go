package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/server"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

func TestServer_Healthz(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{
			name:     "healthz returns 200",
			path:     "/healthz",
			wantCode: http.StatusOK,
		},
		{
			name:     "unknown path returns 404",
			path:     "/not-found",
			wantCode: http.StatusNotFound,
		},
	}

	srv := server.New(store.NewMemory(), nil, auth.Config{}) // nil validator = dev mode
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("get %s: %v", tc.path, err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("%s = %d, want %d", tc.path, resp.StatusCode, tc.wantCode)
			}
		})
	}
}

func TestServer_AuthConfig(t *testing.T) {
	tests := []struct {
		name             string
		cfg              auth.Config
		wantEnabled      bool
		wantIssuer       string
		wantClientID     string
		wantDevClientID  string
	}{
		{
			name:            "enabled",
			cfg:             auth.Config{IssuerURL: "https://oidc.example", ClientID: "web", DeviceClientID: "cli"},
			wantEnabled:     true,
			wantIssuer:      "https://oidc.example",
			wantClientID:    "web",
			wantDevClientID: "cli",
		},
		{
			name:            "dev mode",
			cfg:             auth.Config{},
			wantEnabled:     false,
			wantIssuer:      "",
			wantClientID:    "",
			wantDevClientID: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := server.New(store.NewMemory(), nil, tt.cfg)
			ts := httptest.NewServer(srv.Handler())
			t.Cleanup(ts.Close)
			resp, err := http.Get(ts.URL + "/auth/config")
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			var got struct {
				Enabled        bool   `json:"enabled"`
				IssuerURL      string `json:"issuer_url"`
				ClientID       string `json:"client_id"`
				DeviceClientID string `json:"device_client_id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Enabled != tt.wantEnabled {
				t.Fatalf("enabled = %v, want %v", got.Enabled, tt.wantEnabled)
			}
			if got.IssuerURL != tt.wantIssuer {
				t.Fatalf("issuer_url = %q, want %q", got.IssuerURL, tt.wantIssuer)
			}
			if got.ClientID != tt.wantClientID {
				t.Fatalf("client_id = %q, want %q", got.ClientID, tt.wantClientID)
			}
			if got.DeviceClientID != tt.wantDevClientID {
				t.Fatalf("device_client_id = %q, want %q", got.DeviceClientID, tt.wantDevClientID)
			}
		})
	}
}

func TestServer_AuthConfig_MethodNotAllowed(t *testing.T) {
	srv := server.New(store.NewMemory(), nil, auth.Config{IssuerURL: "https://oidc.example", ClientID: "web"})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	tests := []struct {
		name   string
		method string
	}{
		{name: "POST rejected", method: http.MethodPost},
		{name: "PUT rejected", method: http.MethodPut},
		{name: "DELETE rejected", method: http.MethodDelete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, ts.URL+"/auth/config", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}
}
