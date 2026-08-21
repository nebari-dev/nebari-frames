package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/branding"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
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
			name:     "unknown client route falls back to SPA index",
			path:     "/not-found",
			wantCode: http.StatusOK,
		},
		{
			name:     "missing asset returns 404",
			path:     "/assets/missing.js",
			wantCode: http.StatusNotFound,
		},
	}

	srv := server.New(frames.NewService(store.NewMemory()), nil, auth.Config{}, branding.Config{}, true, nil) // dev mode
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
		name            string
		cfg             auth.Config
		wantEnabled     bool
		wantIssuer      string
		wantClientID    string
		wantDevClientID string
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
			srv := server.New(frames.NewService(store.NewMemory()), nil, tt.cfg, branding.Config{}, true, nil)
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
	srv := server.New(frames.NewService(store.NewMemory()), nil, auth.Config{IssuerURL: "https://oidc.example", ClientID: "web"}, branding.Config{}, true, nil)
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

func TestServer_Branding(t *testing.T) {
	tests := []struct {
		name     string
		cfg      branding.Config
		wantBody string
	}{
		{
			name:     "unbranded serves an empty document",
			cfg:      branding.Config{},
			wantBody: `{}`,
		},
		{
			name: "branded document is served as-is",
			cfg: branding.Config{
				Title:       "Acme Frames",
				LogoURL:     "https://cdn.acme.example/logo.svg",
				LogoURLDark: "https://cdn.acme.example/logo-dark.svg",
				FaviconURL:  "/favicon.svg",
				Theme:       branding.Theme{"light": {"primary": "#0066cc"}},
			},
			wantBody: `{"title":"Acme Frames","logoUrl":"https://cdn.acme.example/logo.svg",` +
				`"logoUrlDark":"https://cdn.acme.example/logo-dark.svg","faviconUrl":"/favicon.svg",` +
				`"theme":{"light":{"primary":"#0066cc"}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := server.New(frames.NewService(store.NewMemory()), nil, auth.Config{}, tt.cfg, true, nil)
			ts := httptest.NewServer(srv.Handler())
			t.Cleanup(ts.Close)

			resp, err := http.Get(ts.URL + "/config.json")
			if err != nil {
				t.Fatalf("get /config.json: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			// Branding must never be cached, or a rebrand would wait for the
			// browser to drop a stale copy.
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if got := strings.TrimSpace(string(body)); got != tt.wantBody {
				t.Fatalf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

func TestServer_Branding_MethodNotAllowed(t *testing.T) {
	srv := server.New(frames.NewService(store.NewMemory()), nil, auth.Config{}, branding.Config{}, true, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/config.json", nil)
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
}

// fakeReadiness is a TokenValidator that reports a fixed readiness state.
type fakeReadiness struct{ ready bool }

func (f fakeReadiness) Validate(context.Context, string) (*auth.Claims, error) {
	return nil, auth.ErrNotReady
}
func (f fakeReadiness) Ready() bool { return f.ready }

// plainValidator implements only TokenValidator (not auth.ReadinessValidator).
// Used to exercise the defensive always-ready fallback in readinessFunc.
type plainValidator struct{}

func (plainValidator) Validate(_ context.Context, _ string) (*auth.Claims, error) {
	return nil, nil
}

func TestServer_Readyz(t *testing.T) {
	tests := []struct {
		name      string
		validator auth.TokenValidator
		devMode   bool
		wantCode  int
	}{
		{name: "dev mode is ready", validator: nil, devMode: true, wantCode: http.StatusOK},
		{name: "auth ready", validator: fakeReadiness{ready: true}, devMode: false, wantCode: http.StatusOK},
		{name: "auth not ready", validator: fakeReadiness{ready: false}, devMode: false, wantCode: http.StatusServiceUnavailable},
		// plainValidator does not implement ReadinessValidator, so readinessFunc
		// falls through to the fail-closed branch and reports not-ready (503):
		// /readyz only goes green once a readiness-aware validator confirms it.
		{name: "plain validator without ReadinessValidator is not ready", validator: plainValidator{}, devMode: false, wantCode: http.StatusServiceUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := server.New(frames.NewService(store.NewMemory()), tc.validator, auth.Config{}, branding.Config{}, tc.devMode, nil)
			ts := httptest.NewServer(srv.Handler())
			t.Cleanup(ts.Close)
			resp, err := http.Get(ts.URL + "/readyz")
			if err != nil {
				t.Fatalf("get /readyz: %v", err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("/readyz = %d, want %d", resp.StatusCode, tc.wantCode)
			}
		})
	}
}

// An authenticated caller with no write permission can still make the server
// read a request body. Without a cap the whole thing is buffered before RBAC or
// any content limit is consulted, so a single request can exhaust the memory of
// a deployment that is pinned to one replica.
func TestConnectRejectsOversizedRequestBody(t *testing.T) {
	srv := server.New(frames.NewService(store.NewMemory()), nil, auth.Config{}, branding.Config{}, true, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Valid JSON, so a rejection cannot be mistaken for a parse failure: without
	// the cap this body is read in full and the request proceeds to auth.
	padding := strings.Repeat("a", server.MaxRequestBytes+1024)
	body := []byte(`{"changelog":"` + padding + `"}`)
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/frames.v1.FrameService/PublishFrame", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(resp.Body)
	// Connect reports an over-large message as resource_exhausted. Anything else
	// (including a parse or auth error) means the body was read in full first.
	if !strings.Contains(string(payload), "resource_exhausted") {
		t.Fatalf("want resource_exhausted for an over-large body, got status %d body %s",
			resp.StatusCode, string(payload)[:min(200, len(payload))])
	}
}
