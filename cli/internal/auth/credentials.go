package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CachedToken holds a cached OIDC token and its expiry. The RefreshToken,
// RefreshExpiry, TokenEndpoint, and ClientID fields are optional and support
// silent refresh of the ID token. They are omitted on save when empty so
// older credentials files remain forward-compatible.
type CachedToken struct {
	IDToken       string    `json:"id_token"`
	Expiry        time.Time `json:"expiry"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	RefreshExpiry time.Time `json:"refresh_expiry,omitempty"`
	TokenEndpoint string    `json:"token_endpoint,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
}

// CanRefresh reports whether this token has the fields needed to attempt a
// refresh_token grant: a refresh token, a token endpoint, a client id, and
// the refresh token has not itself expired (if an expiry is recorded).
func (t *CachedToken) CanRefresh() bool {
	if t == nil || t.RefreshToken == "" || t.TokenEndpoint == "" || t.ClientID == "" {
		return false
	}
	if !t.RefreshExpiry.IsZero() && time.Now().After(t.RefreshExpiry) {
		return false
	}
	return true
}

// DefaultCredentialsPath returns ~/.config/frames/credentials.json.
func DefaultCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "frames", "credentials.json")
}

// FarFuture returns a time far in the future, useful for test tokens.
func FarFuture() time.Time {
	return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
}

// SaveToken writes the token to the given path with 0600 permissions.
// Creates parent directories with 0700 permissions if needed.
func SaveToken(path string, tok *CachedToken) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Persisting the refresh token to disk is the whole purpose of this
	// file - the alternative is forcing an interactive device-flow re-login
	// every time the ID token expires. The file is written with 0600 in a
	// 0700 parent directory.
	data, err := json.MarshalIndent(tok, "", "  ") //nolint:gosec // G117: refresh token persistence is intentional, see comment above
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads a cached token from the given path.
// Returns (nil, nil) if the file is missing, malformed, or the token is expired.
// Use LoadTokenRaw if you need to distinguish expired from missing.
func LoadToken(path string) (*CachedToken, error) {
	tok, err := LoadTokenRaw(path)
	if err != nil || tok == nil {
		return nil, nil
	}
	if time.Now().After(tok.Expiry) {
		return nil, nil
	}
	return tok, nil
}

// LoadTokenRaw reads a cached token regardless of expiry.
// Returns (nil, nil) if the file is missing or malformed.
// Returns the token even if expired (caller checks expiry).
func LoadTokenRaw(path string) (*CachedToken, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is from user config, not untrusted input
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}
	var tok CachedToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, nil
	}
	if tok.IDToken == "" {
		return nil, nil
	}
	return &tok, nil
}

// DeleteToken removes the credentials file. No error if missing.
func DeleteToken(path string) {
	_ = os.Remove(path)
}
