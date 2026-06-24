package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
)

// stubTokenEndpoint returns an httptest server whose responses are controlled
// by the supplied handler. The handler receives a parsed form body so each
// test can inspect grant_type, refresh_token, client_id without re-parsing.
func stubTokenEndpoint(t *testing.T, handler func(t *testing.T, w http.ResponseWriter, form url.Values)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		handler(t, w, form)
	}))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// jwtWithExp returns a stub JWT whose unverified payload has the given
// expiry. Header and signature are dummy values; nothing in the auth
// package verifies the signature client-side.
func jwtWithExp(t *testing.T, exp time.Time) string {
	t.Helper()
	header := "eyJhbGciOiJub25lIn0"
	payload := struct {
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
	}{
		Email: "test@example.com",
		Exp:   exp.Unix(),
	}
	raw, _ := json.Marshal(payload)
	return header + "." + base64.RawURLEncoding.EncodeToString(raw) + ".sig"
}

func TestCachedTokenCanRefresh(t *testing.T) {
	tests := []struct {
		name string
		tok  *auth.CachedToken
		want bool
	}{
		{"nil", nil, false},
		{"missing refresh token", &auth.CachedToken{TokenEndpoint: "x", ClientID: "c"}, false},
		{"missing endpoint", &auth.CachedToken{RefreshToken: "r", ClientID: "c"}, false},
		{"missing client id", &auth.CachedToken{RefreshToken: "r", TokenEndpoint: "x"}, false},
		{"all present, no expiry", &auth.CachedToken{RefreshToken: "r", TokenEndpoint: "x", ClientID: "c"}, true},
		{"refresh expired", &auth.CachedToken{RefreshToken: "r", TokenEndpoint: "x", ClientID: "c", RefreshExpiry: time.Now().Add(-time.Minute)}, false},
		{"refresh valid", &auth.CachedToken{RefreshToken: "r", TokenEndpoint: "x", ClientID: "c", RefreshExpiry: time.Now().Add(time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.CanRefresh(); got != tt.want {
				t.Errorf("CanRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshIDToken_Success(t *testing.T) {
	newExpiry := time.Now().Add(30 * time.Minute)
	newIDToken := jwtWithExp(t, newExpiry)

	srv := stubTokenEndpoint(t, func(t *testing.T, w http.ResponseWriter, form url.Values) {
		if form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", form.Get("grant_type"))
		}
		if form.Get("refresh_token") != "old-refresh" {
			t.Errorf("refresh_token = %q, want old-refresh", form.Get("refresh_token"))
		}
		if form.Get("client_id") != "device-client" {
			t.Errorf("client_id = %q, want device-client", form.Get("client_id"))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id_token":           newIDToken,
			"refresh_token":      "new-refresh",
			"refresh_expires_in": 1800,
		})
	})
	defer srv.Close()

	in := &auth.CachedToken{
		IDToken:       "old-id",
		Expiry:        time.Now().Add(-time.Minute),
		RefreshToken:  "old-refresh",
		TokenEndpoint: srv.URL,
		ClientID:      "device-client",
	}

	got, err := auth.RefreshIDToken(context.Background(), in)
	if err != nil {
		t.Fatalf("RefreshIDToken: %v", err)
	}
	if got.IDToken != newIDToken {
		t.Errorf("IDToken = %q, want %q", got.IDToken, newIDToken)
	}
	if got.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want new-refresh (rotated)", got.RefreshToken)
	}
	if got.TokenEndpoint != srv.URL || got.ClientID != "device-client" {
		t.Errorf("endpoint/client preservation failed: %+v", got)
	}
	if got.RefreshExpiry.IsZero() {
		t.Error("RefreshExpiry should be set when refresh_expires_in is returned")
	}
}

func TestRefreshIDToken_PreservesRefreshTokenWhenNotRotated(t *testing.T) {
	srv := stubTokenEndpoint(t, func(_ *testing.T, w http.ResponseWriter, _ url.Values) {
		// Provider returns no refresh_token field in the response.
		writeJSON(w, http.StatusOK, map[string]any{
			"id_token": jwtWithExp(t, time.Now().Add(time.Hour)),
		})
	})
	defer srv.Close()

	in := &auth.CachedToken{
		RefreshToken:  "keep-this",
		RefreshExpiry: time.Now().Add(2 * time.Hour),
		TokenEndpoint: srv.URL,
		ClientID:      "c",
	}
	got, err := auth.RefreshIDToken(context.Background(), in)
	if err != nil {
		t.Fatalf("RefreshIDToken: %v", err)
	}
	if got.RefreshToken != "keep-this" {
		t.Errorf("RefreshToken should be preserved when provider omits rotation, got %q", got.RefreshToken)
	}
	if !got.RefreshExpiry.Equal(in.RefreshExpiry) {
		t.Error("RefreshExpiry should be preserved when refresh token not rotated")
	}
}

func TestRefreshIDToken_RejectedReturnsErrRefreshFailed(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := stubTokenEndpoint(t, func(_ *testing.T, w http.ResponseWriter, _ url.Values) {
				writeJSON(w, status, map[string]string{"error": "invalid_grant"})
			})
			defer srv.Close()

			in := &auth.CachedToken{RefreshToken: "r", TokenEndpoint: srv.URL, ClientID: "c"}
			_, err := auth.RefreshIDToken(context.Background(), in)
			if !errors.Is(err, auth.ErrRefreshFailed) {
				t.Errorf("status %d: err = %v, want ErrRefreshFailed", status, err)
			}
		})
	}
}

func TestRefreshIDToken_TransportErrorReturnedAsIs(t *testing.T) {
	srv := stubTokenEndpoint(t, func(_ *testing.T, w http.ResponseWriter, _ url.Values) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer srv.Close()

	in := &auth.CachedToken{RefreshToken: "r", TokenEndpoint: srv.URL, ClientID: "c"}
	_, err := auth.RefreshIDToken(context.Background(), in)
	if err == nil || errors.Is(err, auth.ErrRefreshFailed) {
		t.Errorf("expected non-ErrRefreshFailed error for 500, got: %v", err)
	}
}

func TestRefreshIDToken_NotRefreshable(t *testing.T) {
	_, err := auth.RefreshIDToken(context.Background(), &auth.CachedToken{IDToken: "x"})
	if err == nil {
		t.Fatal("expected error for unrefreshable token")
	}
}

func TestLoadAndRefresh_NoToken(t *testing.T) {
	tok, err := auth.LoadAndRefresh(context.Background(), filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || tok != nil {
		t.Errorf("expected (nil, nil) for missing file, got (%v, %v)", tok, err)
	}
}

func TestLoadAndRefresh_ValidTokenReturnedAsIs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	saved := &auth.CachedToken{
		IDToken: "valid",
		Expiry:  time.Now().Add(time.Hour),
	}
	if err := auth.SaveToken(path, saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := auth.LoadAndRefresh(context.Background(), path)
	if err != nil {
		t.Fatalf("LoadAndRefresh: %v", err)
	}
	if got == nil || got.IDToken != "valid" {
		t.Errorf("expected valid token, got %+v", got)
	}
}

func TestLoadAndRefresh_ExpiredWithoutRefreshTokenReturnsNil(t *testing.T) {
	// Mirrors a credentials file from an older CLI version: no refresh fields.
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	saved := &auth.CachedToken{
		IDToken: "stale",
		Expiry:  time.Now().Add(-time.Hour),
	}
	if err := auth.SaveToken(path, saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := auth.LoadAndRefresh(context.Background(), path)
	if err != nil || got != nil {
		t.Errorf("expected (nil, nil) for expired-without-refresh, got (%v, %v)", got, err)
	}
}

func TestLoadAndRefresh_ExpiredWithRefreshSucceedsAndPersists(t *testing.T) {
	newID := jwtWithExp(t, time.Now().Add(30*time.Minute))
	srv := stubTokenEndpoint(t, func(_ *testing.T, w http.ResponseWriter, _ url.Values) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id_token":      newID,
			"refresh_token": "rotated",
		})
	})
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	saved := &auth.CachedToken{
		IDToken:       "expired",
		Expiry:        time.Now().Add(-time.Hour),
		RefreshToken:  "old",
		TokenEndpoint: srv.URL,
		ClientID:      "device",
	}
	if err := auth.SaveToken(path, saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := auth.LoadAndRefresh(context.Background(), path)
	if err != nil {
		t.Fatalf("LoadAndRefresh: %v", err)
	}
	if got == nil || got.IDToken != newID {
		t.Errorf("expected refreshed token, got %+v", got)
	}

	// Verify the refreshed token was persisted to disk.
	reloaded, _ := auth.LoadTokenRaw(path)
	if reloaded == nil || reloaded.IDToken != newID || reloaded.RefreshToken != "rotated" {
		t.Errorf("refreshed token not persisted, got %+v", reloaded)
	}
}

func TestLoadAndRefresh_RefreshRejectedReturnsNil(t *testing.T) {
	srv := stubTokenEndpoint(t, func(_ *testing.T, w http.ResponseWriter, _ url.Values) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	})
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	saved := &auth.CachedToken{
		IDToken:       "expired",
		Expiry:        time.Now().Add(-time.Hour),
		RefreshToken:  "revoked",
		TokenEndpoint: srv.URL,
		ClientID:      "device",
	}
	if err := auth.SaveToken(path, saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := auth.LoadAndRefresh(context.Background(), path)
	if err != nil || got != nil {
		t.Errorf("expected (nil, nil) when refresh rejected, got (%v, %v)", got, err)
	}
}

func TestSaveLoadRoundTripWithRefreshFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	want := &auth.CachedToken{
		IDToken:       "id",
		Expiry:        time.Now().Add(time.Hour).Round(time.Second),
		RefreshToken:  "ref",
		RefreshExpiry: time.Now().Add(2 * time.Hour).Round(time.Second),
		TokenEndpoint: "https://kc.example/realms/r/protocol/openid-connect/token",
		ClientID:      "device-client",
	}
	if err := auth.SaveToken(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := auth.LoadTokenRaw(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.IDToken != want.IDToken ||
		got.RefreshToken != want.RefreshToken ||
		got.TokenEndpoint != want.TokenEndpoint ||
		got.ClientID != want.ClientID ||
		!got.Expiry.Equal(want.Expiry) ||
		!got.RefreshExpiry.Equal(want.RefreshExpiry) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}
