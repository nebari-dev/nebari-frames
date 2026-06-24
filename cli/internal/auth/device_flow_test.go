package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
)

// testJWT is a JWT with payload {"sub":"user-1","email":"test@example.com","exp":9999999999}
const testJWT = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJleHAiOjk5OTk5OTk5OTl9.sig"

func setupMockServers(t *testing.T, tokenResponses []string) (skillsctlURL string, cleanup func()) {
	t.Helper()

	// Create OIDC provider mock
	var tokenCallCount atomic.Int32
	oidcMux := http.NewServeMux()
	oidcServer := httptest.NewUnstartedServer(oidcMux)
	oidcServer.Start()

	oidcMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": oidcServer.URL + "/device",
			"token_endpoint":                oidcServer.URL + "/token",
		})
	})
	oidcMux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "test-device-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          oidcServer.URL + "/verify",
			"verification_uri_complete": oidcServer.URL + "/verify?code=ABCD-EFGH",
			"expires_in":                300,
			"interval":                  1,
		})
	})
	oidcMux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		idx := int(tokenCallCount.Add(1)) - 1
		w.Header().Set("Content-Type", "application/json")
		if idx < len(tokenResponses) {
			_, _ = w.Write([]byte(tokenResponses[idx]))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id_token": testJWT,
		})
	})

	// Create skillsctl server mock
	skillsctlMux := http.NewServeMux()
	skillsctlMux.HandleFunc("/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enabled":    true,
			"issuer_url": oidcServer.URL,
			"client_id":  "test-client",
		})
	})
	skillsctlServer := httptest.NewServer(skillsctlMux)

	cleanup = func() {
		oidcServer.Close()
		skillsctlServer.Close()
	}

	return skillsctlServer.URL, cleanup
}

func setupDisabledServer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestStartDeviceFlow_Success(t *testing.T) {
	serverURL, cleanup := setupMockServers(t, nil)
	defer cleanup()

	pending, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pending.UserCode != "ABCD-EFGH" {
		t.Errorf("expected user code ABCD-EFGH, got %q", pending.UserCode)
	}
	if pending.VerificationURIComplete == "" {
		t.Error("expected non-empty verification_uri_complete")
	}
	if pending.DeviceCode != "test-device-code" {
		t.Errorf("expected device code test-device-code, got %q", pending.DeviceCode)
	}
}

func TestPollForToken_ImmediateSuccess(t *testing.T) {
	serverURL, cleanup := setupMockServers(t, nil)
	defer cleanup()

	pending, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	result, err := auth.PollForToken(context.Background(), pending, time.Millisecond)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.IDToken == "" {
		t.Error("expected non-empty ID token")
	}
	if result.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", result.Email)
	}
	if result.Expiry.IsZero() {
		t.Error("expected non-zero expiry")
	}
}

func TestPollForToken_PendingThenSuccess(t *testing.T) {
	pending1 := `{"error":"authorization_pending"}`
	pending2 := `{"error":"authorization_pending"}`
	serverURL, cleanup := setupMockServers(t, []string{pending1, pending2})
	defer cleanup()

	pending, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	result, err := auth.PollForToken(context.Background(), pending, time.Millisecond)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.IDToken == "" {
		t.Error("expected non-empty ID token")
	}
}

func TestPollForToken_AccessDenied(t *testing.T) {
	denied := `{"error":"access_denied"}`
	serverURL, cleanup := setupMockServers(t, []string{denied})
	defer cleanup()

	pending, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = auth.PollForToken(context.Background(), pending, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if err.Error() != "authentication denied" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPollForToken_ExpiredToken(t *testing.T) {
	expired := `{"error":"expired_token"}`
	serverURL, cleanup := setupMockServers(t, []string{expired})
	defer cleanup()

	pending, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = auth.PollForToken(context.Background(), pending, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestStartDeviceFlow_UsesDeviceClientID(t *testing.T) {
	// Create OIDC provider mock that verifies client_id
	var receivedClientID string
	oidcMux := http.NewServeMux()
	oidcServer := httptest.NewUnstartedServer(oidcMux)
	oidcServer.Start()
	defer oidcServer.Close()

	oidcMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": oidcServer.URL + "/device",
			"token_endpoint":                oidcServer.URL + "/token",
		})
	})
	oidcMux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		receivedClientID = r.FormValue("client_id")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "test-device-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          oidcServer.URL + "/verify",
			"verification_uri_complete": oidcServer.URL + "/verify?code=ABCD-EFGH",
			"expires_in":                300,
			"interval":                  1,
		})
	})

	// Skillsctl server returns device_client_id
	skillsctlMux := http.NewServeMux()
	skillsctlMux.HandleFunc("/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enabled":          true,
			"issuer_url":       oidcServer.URL,
			"client_id":        "confidential-client",
			"device_client_id": "device-flow-client",
		})
	})
	skillsctlServer := httptest.NewServer(skillsctlMux)
	defer skillsctlServer.Close()

	pending, err := auth.StartDeviceFlow(context.Background(), skillsctlServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedClientID != "device-flow-client" {
		t.Errorf("expected device authorization to use device_client_id %q, got %q", "device-flow-client", receivedClientID)
	}
	if pending.ClientID != "device-flow-client" {
		t.Errorf("expected pending.ClientID to be device_client_id %q, got %q", "device-flow-client", pending.ClientID)
	}
}

func TestStartDeviceFlow_AuthDisabled(t *testing.T) {
	serverURL := setupDisabledServer(t)

	_, err := auth.StartDeviceFlow(context.Background(), serverURL)
	if err == nil {
		t.Fatal("expected error for disabled auth")
	}
	if err != auth.ErrAuthDisabled {
		t.Errorf("expected ErrAuthDisabled, got: %v", err)
	}
}

func TestDecodeJWTClaims(t *testing.T) {
	email, exp := auth.DecodeJWTClaims(testJWT)
	if email != "test@example.com" {
		t.Errorf("expected test@example.com, got %q", email)
	}
	if exp.IsZero() {
		t.Error("expected non-zero expiry")
	}
}

func TestDecodeJWTClaims_Invalid(t *testing.T) {
	email, exp := auth.DecodeJWTClaims("not-a-jwt")
	if email != "" {
		t.Errorf("expected empty email, got %q", email)
	}
	if !exp.IsZero() {
		t.Error("expected zero expiry")
	}
}
