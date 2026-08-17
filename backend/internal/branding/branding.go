// Package branding resolves the branding fields of the runtime configuration
// document the SPA fetches from /config.json at startup: browser-tab title, header logo, favicon, and theme
// token overrides. Branding is delivered at runtime rather than baked into the
// image, so an operator can rebrand a deployment without rebuilding it.
//
// Fields resolve with this precedence (highest first, per field):
//
//  1. BRANDING_* environment variables
//  2. The JSON document at BRANDING_CONFIG_FILE (the Helm chart renders one
//     into a ConfigMap and mounts it)
//  3. Built-in Nebari defaults, applied by the SPA when a field is empty
//
// Nothing here is required: an unset environment yields the zero Config, the
// SPA falls back to its bundled defaults, and the app looks exactly as it does
// without branding. Invalid input is skipped rather than fatal - see Load.
package branding

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
)

// ThemeTokens maps a camelCase theme token name (e.g. "primaryForeground") to a
// CSS value. The SPA applies each as the kebab-case custom property
// (--primary-foreground) and drops values that fail its CSS-safety check.
type ThemeTokens map[string]string

// Theme carries per-mode token overrides. Keys are "light" and "dark"; any
// other key is passed through to the SPA, which ignores it.
type Theme map[string]ThemeTokens

// Config is the branding portion of the document served at /config.json. Empty fields are
// omitted so an unbranded deployment serves "{}".
type Config struct {
	Title       string `json:"title,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
	LogoURLDark string `json:"logoUrlDark,omitempty"`
	FaviconURL  string `json:"faviconUrl,omitempty"`
	Theme       Theme  `json:"theme,omitempty"`
}

// Environment variables Load reads. BRANDING_THEME holds a raw JSON object,
// e.g. {"light":{"primary":"#0066cc"},"dark":{}}.
const (
	EnvConfigFile  = "BRANDING_CONFIG_FILE"
	EnvTitle       = "BRANDING_TITLE"
	EnvLogoURL     = "BRANDING_LOGO_URL"
	EnvLogoURLDark = "BRANDING_LOGO_URL_DARK"
	EnvFaviconURL  = "BRANDING_FAVICON_URL"
	EnvTheme       = "BRANDING_THEME"
)

// Load resolves branding from the environment, reading BRANDING_CONFIG_FILE
// with readFile. Pass os.ReadFile (or nil, which means os.ReadFile) and
// os.Getenv in production; both are parameters so tests need no real files or
// process environment.
//
// Load is best-effort by design: branding must never keep the app from
// starting. An unreadable config file, a malformed base document, or invalid
// BRANDING_THEME JSON is reported in the returned error while every other field
// still resolves, so callers should log the error as a warning and use the
// returned Config regardless.
func Load(getenv func(string) string, readFile func(string) ([]byte, error)) (Config, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}

	var cfg Config
	var problems []error

	if path := getenv(EnvConfigFile); path != "" {
		data, err := readFile(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("read %s=%s: %w", EnvConfigFile, path, err))
		} else if err := json.Unmarshal(data, &cfg); err != nil {
			// Unmarshalling into cfg directly means a document that parses
			// partially before failing still contributes the fields it did set,
			// matching the per-field best-effort contract above.
			problems = append(problems, fmt.Errorf("parse %s=%s: %w", EnvConfigFile, path, err))
		}
	}

	// Individual env vars overlay the base document, field by field.
	for _, o := range []struct {
		env    string
		assign func(string)
	}{
		{EnvTitle, func(v string) { cfg.Title = v }},
		{EnvLogoURL, func(v string) { cfg.LogoURL = v }},
		{EnvLogoURLDark, func(v string) { cfg.LogoURLDark = v }},
		{EnvFaviconURL, func(v string) { cfg.FaviconURL = v }},
	} {
		if v := getenv(o.env); v != "" {
			o.assign(v)
		}
	}

	if raw := getenv(EnvTheme); raw != "" {
		var theme Theme
		if err := json.Unmarshal([]byte(raw), &theme); err != nil {
			problems = append(problems, fmt.Errorf("parse %s: %w", EnvTheme, err))
		} else {
			cfg.Theme = theme
		}
	}

	return cfg, errors.Join(problems...)
}

// ImageOrigins returns the distinct scheme://host origins of the configured
// logo and favicon URLs that point at another host. The server adds them to the
// SPA's Content-Security-Policy img-src, without which a branded logo hosted on
// a CDN would be blocked by the default img-src 'self'. Root-relative paths and
// data: URIs are already covered by the base policy and yield no origin.
func (c Config) ImageOrigins() []string {
	var origins []string
	seen := map[string]bool{}
	for _, raw := range []string{c.LogoURL, c.LogoURLDark, c.FaviconURL} {
		origin := originOf(raw)
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		origins = append(origins, origin)
	}
	return origins
}

// originOf returns the scheme://host of an absolute http(s) URL, or "" for
// anything else (empty values, relative paths, data: URIs, unparseable input).
func originOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
