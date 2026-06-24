package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes limits the size of HTTP responses to prevent OOM from
// malicious or misconfigured servers.
const maxResponseBytes = 1024 * 1024 // 1MB

// httpClient is used for all auth-related HTTP requests. It sets a timeout
// to prevent indefinite hangs when servers are unreachable.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// ErrAuthDisabled is returned when the server has no OIDC configured.
var ErrAuthDisabled = errors.New("server does not require authentication")

// DeviceFlowPending holds the state between StartDeviceFlow and PollForToken.
type DeviceFlowPending struct {
	VerificationURI         string
	VerificationURIComplete string
	UserCode                string
	DeviceCode              string
	TokenEndpoint           string
	ClientID                string
	Interval                time.Duration
	ExpiresAt               time.Time
}

// DeviceFlowResult holds the result of a completed device flow.
type DeviceFlowResult struct {
	IDToken       string
	Email         string
	Expiry        time.Time
	RefreshToken  string
	RefreshExpiry time.Time
	TokenEndpoint string
	ClientID      string
}

type authConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	IssuerURL      string `json:"issuer_url"`
	ClientID       string `json:"client_id"`
	DeviceClientID string `json:"device_client_id"`
}

type oidcDiscovery struct {
	DeviceAuthEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint      string `json:"token_endpoint"`
}

type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	IDToken          string `json:"id_token"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Error            string `json:"error"`
}

// StartDeviceFlow fetches auth config from the skillsctl server, discovers
// OIDC endpoints, requests a device code, and returns the pending state
// for display to the user. Call PollForToken after displaying the URL.
func StartDeviceFlow(ctx context.Context, serverURL string) (*DeviceFlowPending, error) {
	authCfg, err := fetchAuthConfig(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("cannot reach server at %s: %w", serverURL, err)
	}
	if !authCfg.Enabled {
		return nil, ErrAuthDisabled
	}

	if err := validateIssuerURL(authCfg.IssuerURL); err != nil {
		return nil, err
	}

	discovery, err := fetchOIDCDiscovery(ctx, authCfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("cannot reach OIDC provider at %s: %w", authCfg.IssuerURL, err)
	}
	if discovery.DeviceAuthEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider does not support device flow")
	}
	if err := validateEndpointOrigin(discovery.DeviceAuthEndpoint, authCfg.IssuerURL); err != nil {
		return nil, fmt.Errorf("device auth endpoint: %w", err)
	}
	if err := validateEndpointOrigin(discovery.TokenEndpoint, authCfg.IssuerURL); err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}

	// Use the device flow client ID if available, otherwise fall back to the
	// regular client ID (for servers that don't provision a separate device client).
	deviceClientID := authCfg.DeviceClientID
	if deviceClientID == "" {
		deviceClientID = authCfg.ClientID
	}

	deviceAuth, err := requestDeviceCode(ctx, discovery.DeviceAuthEndpoint, deviceClientID)
	if err != nil {
		return nil, fmt.Errorf("device authorization: %w", err)
	}

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	expiresIn := time.Duration(deviceAuth.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 5 * time.Minute
	}

	return &DeviceFlowPending{
		VerificationURI:         deviceAuth.VerificationURI,
		VerificationURIComplete: deviceAuth.VerificationURIComplete,
		UserCode:                deviceAuth.UserCode,
		DeviceCode:              deviceAuth.DeviceCode,
		TokenEndpoint:           discovery.TokenEndpoint,
		ClientID:                deviceClientID,
		Interval:                interval,
		ExpiresAt:               time.Now().Add(expiresIn),
	}, nil
}

// PollForToken polls the token endpoint until the user completes authentication.
// Pass the pending state from StartDeviceFlow. The pollInterval parameter
// overrides the server-suggested interval (use 0 for the server default).
// In tests, pass a short duration to avoid sleeping.
func PollForToken(ctx context.Context, pending *DeviceFlowPending, pollInterval time.Duration) (*DeviceFlowResult, error) {
	interval := pending.Interval
	if pollInterval > 0 {
		interval = pollInterval
	}

	for {
		if time.Now().After(pending.ExpiresAt) {
			return nil, fmt.Errorf("authentication timed out. Try again")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		tok, err := pollToken(ctx, pending.TokenEndpoint, pending.ClientID, pending.DeviceCode)
		if err != nil {
			return nil, err
		}
		if tok.Error == "authorization_pending" {
			continue
		}
		if tok.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		if tok.Error == "expired_token" {
			return nil, fmt.Errorf("authentication timed out. Try again")
		}
		if tok.Error == "access_denied" {
			return nil, fmt.Errorf("authentication denied")
		}
		if tok.Error != "" {
			return nil, fmt.Errorf("authentication error: %s", tok.Error)
		}

		email, exp := DecodeJWTClaims(tok.IDToken)
		var refreshExpiry time.Time
		if tok.RefreshExpiresIn > 0 {
			refreshExpiry = time.Now().Add(time.Duration(tok.RefreshExpiresIn) * time.Second)
		}
		return &DeviceFlowResult{
			IDToken:       tok.IDToken,
			Email:         email,
			Expiry:        exp,
			RefreshToken:  tok.RefreshToken,
			RefreshExpiry: refreshExpiry,
			TokenEndpoint: pending.TokenEndpoint,
			ClientID:      pending.ClientID,
		}, nil
	}
}

// decodeJSON reads a limited response body and decodes JSON into v.
// Returns an error if the status code is not 200 (for GET) or 200/400 (for token endpoints).
func decodeJSON(resp *http.Response, v any) error {
	defer func() { _ = resp.Body.Close() }()
	body := io.LimitReader(resp.Body, maxResponseBytes)
	return json.NewDecoder(body).Decode(v)
}

func fetchAuthConfig(ctx context.Context, serverURL string) (*authConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/auth/config", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from %s/auth/config", resp.StatusCode, serverURL)
	}
	var cfg authConfigResponse
	if err := decodeJSON(resp, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func fetchOIDCDiscovery(ctx context.Context, issuerURL string) (*oidcDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", issuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from OIDC discovery", resp.StatusCode)
	}
	var disc oidcDiscovery
	if err := decodeJSON(resp, &disc); err != nil {
		return nil, err
	}
	return &disc, nil
}

func requestDeviceCode(ctx context.Context, endpoint, clientID string) (*deviceAuthResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"openid email profile"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from device authorization endpoint", resp.StatusCode)
	}
	var dar deviceAuthResponse
	if err := decodeJSON(resp, &dar); err != nil {
		return nil, err
	}
	return &dar, nil
}

func pollToken(ctx context.Context, endpoint, clientID, deviceCode string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":   {clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Token endpoint returns 400 for error responses (authorization_pending, etc.)
	// and 200 for success. Both contain JSON we need to decode.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from token endpoint", resp.StatusCode)
	}
	var tok tokenResponse
	if err := decodeJSON(resp, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// validateIssuerURL checks the issuer URL for SSRF risks. HTTP scheme is only
// allowed for localhost. HTTPS is required for all other hosts, and known
// internal IP ranges are blocked.
func validateIssuerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid issuer URL %q: %w", rawURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid issuer URL scheme %q: must be http or https", u.Scheme)
	}

	hostname := u.Hostname()

	// HTTP is only allowed for localhost addresses.
	if u.Scheme == "http" {
		if hostname != "localhost" && hostname != "127.0.0.1" && hostname != "::1" {
			return fmt.Errorf("http scheme is only allowed for localhost, got %q", hostname)
		}
		return nil
	}

	// For HTTPS, block internal/reserved IP ranges.
	ip := net.ParseIP(hostname)
	if ip == nil {
		return nil // hostname, not IP - allow
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("issuer URL must not point to an internal address: %s", hostname)
	}
	return nil
}

// validateEndpointOrigin checks that an OIDC endpoint URL shares the same
// scheme and host as the issuer URL to prevent token theft via endpoint hijack.
func validateEndpointOrigin(endpoint, issuerURL string) error {
	eURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL %q: %w", endpoint, err)
	}
	iURL, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("invalid issuer URL %q: %w", issuerURL, err)
	}
	if eURL.Scheme != iURL.Scheme || eURL.Host != iURL.Host {
		return fmt.Errorf("endpoint %q does not match issuer origin %s://%s", endpoint, iURL.Scheme, iURL.Host)
	}
	return nil
}

// DecodeJWTClaims decodes the JWT payload without verification.
// Returns email and expiry. This is for display only - the server verifies the token.
func DecodeJWTClaims(idToken string) (email string, expiry time.Time) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", time.Time{}
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", time.Time{}
	}
	var claims struct {
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(data, &claims); err != nil {
		return "", time.Time{}
	}
	return claims.Email, time.Unix(claims.Exp, 0)
}
