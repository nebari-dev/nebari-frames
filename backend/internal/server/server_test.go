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
		name        string
		cfg         auth.Config
		wantEnabled bool
		wantIssuer  string
	}{
		{"enabled", auth.Config{IssuerURL: "https://oidc.example", ClientID: "web", DeviceClientID: "cli"}, true, "https://oidc.example"},
		{"dev mode", auth.Config{}, false, ""},
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
				Enabled   bool   `json:"enabled"`
				IssuerURL string `json:"issuer_url"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Enabled != tt.wantEnabled || got.IssuerURL != tt.wantIssuer {
				t.Fatalf("got %+v, want enabled=%v issuer=%q", got, tt.wantEnabled, tt.wantIssuer)
			}
		})
	}
}
