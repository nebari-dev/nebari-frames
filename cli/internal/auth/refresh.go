package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// refreshSkew is the buffer applied when deciding whether to refresh proactively.
// If the ID token's remaining lifetime is shorter than this, refresh first.
const refreshSkew = 60 * time.Second

// ErrRefreshFailed is returned when the refresh_token grant is rejected by
// the token endpoint (refresh token expired, revoked, or otherwise invalid).
// Callers should treat this as "needs interactive re-login".
var ErrRefreshFailed = errors.New("refresh token grant failed")

// RefreshIDToken exchanges the cached refresh token for a new ID token using
// the OAuth 2.0 refresh_token grant (RFC 6749 §6). On success, returns a new
// CachedToken with refreshed fields - if the provider returns a rotated
// refresh token (Keycloak does by default), the new value is used. Endpoint,
// ClientID, and a never-rotated refresh expiry are preserved from the input.
//
// Returns ErrRefreshFailed if the token endpoint rejects the grant.
func RefreshIDToken(ctx context.Context, tok *CachedToken) (*CachedToken, error) {
	if !tok.CanRefresh() {
		return nil, fmt.Errorf("token cannot be refreshed: missing refresh token, endpoint, or client id")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {tok.ClientID},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tok.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// 400/401 with an OAuth error JSON means the refresh token is no longer
	// valid (expired session, revoked, etc.). Anything else is a transport
	// problem the caller may want to surface differently.
	if resp.StatusCode != http.StatusOK {
		var body tokenResponse
		_ = decodeJSON(resp, &body)
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrRefreshFailed
		}
		return nil, fmt.Errorf("unexpected status %d from token endpoint", resp.StatusCode)
	}

	var body tokenResponse
	if err := decodeJSON(resp, &body); err != nil {
		return nil, err
	}
	if body.IDToken == "" {
		return nil, fmt.Errorf("token endpoint returned no id_token")
	}

	_, exp := DecodeJWTClaims(body.IDToken)

	// Keycloak rotates the refresh token by default; if the provider didn't
	// return one, reuse the existing refresh token rather than dropping it.
	newRefresh := body.RefreshToken
	newRefreshExpiry := tok.RefreshExpiry
	if newRefresh == "" {
		newRefresh = tok.RefreshToken
	} else if body.RefreshExpiresIn > 0 {
		newRefreshExpiry = time.Now().Add(time.Duration(body.RefreshExpiresIn) * time.Second)
	}

	return &CachedToken{
		IDToken:       body.IDToken,
		Expiry:        exp,
		RefreshToken:  newRefresh,
		RefreshExpiry: newRefreshExpiry,
		TokenEndpoint: tok.TokenEndpoint,
		ClientID:      tok.ClientID,
	}, nil
}

// LoadAndRefresh loads the cached token and renews it via refresh_token grant
// if the ID token is expired (or close to expiring) and a usable refresh
// token is available. The renewed token is persisted back to disk.
//
// Returns (nil, nil) if no token is cached, the token has expired and cannot
// be refreshed, or refresh was attempted and rejected. Callers should treat
// nil as "not authenticated" and prompt for a fresh device-flow login.
//
// Network or transport errors during refresh are returned to the caller so
// they can distinguish "refresh token bad" (silent fall-through to nil) from
// "couldn't reach the token endpoint" (probably worth surfacing).
func LoadAndRefresh(ctx context.Context, path string) (*CachedToken, error) {
	tok, _ := LoadTokenRaw(path)
	if tok == nil {
		return nil, nil
	}

	if time.Until(tok.Expiry) > refreshSkew {
		return tok, nil
	}

	if !tok.CanRefresh() {
		// Expired ID token and no way to renew: behave like the legacy
		// LoadToken and report no usable credentials.
		return nil, nil
	}

	refreshed, err := RefreshIDToken(ctx, tok)
	if err != nil {
		if errors.Is(err, ErrRefreshFailed) {
			return nil, nil
		}
		return nil, err
	}

	if err := SaveToken(path, refreshed); err != nil {
		return nil, fmt.Errorf("save refreshed credentials: %w", err)
	}
	return refreshed, nil
}
