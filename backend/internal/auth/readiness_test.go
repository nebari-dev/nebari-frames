package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// discoveryServer returns an httptest server whose discovery endpoint is gated
// by `up`: it 503s until up is set true, then serves a minimal, valid OIDC
// discovery document (issuer matches the server URL) plus an empty JWKS.
func discoveryServer(t *testing.T, up *atomic.Bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() {
			http.Error(w, "not up", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"keys":[]}`)
	})
	t.Cleanup(srv.Close)
	return srv
}

func TestLazyValidator_BecomesReady(t *testing.T) {
	var up atomic.Bool
	srv := discoveryServer(t, &up)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lv := newLazyValidator(ctx, Config{IssuerURL: srv.URL, ClientID: "test"}, 5*time.Millisecond, 20*time.Millisecond)

	// Before discovery succeeds: not ready, Validate returns ErrNotReady.
	if lv.Ready() {
		t.Fatal("Ready() = true before discovery; want false")
	}
	if _, err := lv.Validate(ctx, "any-token"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Validate before ready = %v; want ErrNotReady", err)
	}

	// Bring the provider up; the background loop should pick it up.
	up.Store(true)
	deadline := time.Now().Add(2 * time.Second)
	for !lv.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("validator did not become ready within 2s")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Once ready, Validate delegates: a garbage token fails verification but is
	// NOT ErrNotReady (proves we are past the readiness gate).
	_, err := lv.Validate(ctx, "garbage")
	if err == nil {
		t.Fatal("Validate(garbage) = nil; want a verification error")
	}
	if errors.Is(err, ErrNotReady) {
		t.Fatalf("Validate(garbage) = ErrNotReady after ready; want a verification error")
	}
}
