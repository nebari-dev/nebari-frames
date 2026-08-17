package branding_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/branding"
)

// envFunc turns a map into a getenv function; a missing key reads as "".
func envFunc(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestLoad_Empty(t *testing.T) {
	cfg, err := branding.Load(envFunc(nil), nil)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(cfg, branding.Config{}) {
		t.Fatalf("Load() = %+v, want zero Config", cfg)
	}
}

func TestLoad_EnvVars(t *testing.T) {
	cfg, err := branding.Load(envFunc(map[string]string{
		branding.EnvTitle:       "Acme Frames",
		branding.EnvLogoURL:     "https://cdn.acme.example/logo.svg",
		branding.EnvLogoURLDark: "https://cdn.acme.example/logo-dark.svg",
		branding.EnvFaviconURL:  "/custom-favicon.svg",
		branding.EnvTheme:       `{"light":{"primary":"#0066cc"},"dark":{"primary":"#3399ff"}}`,
	}), nil)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	want := branding.Config{
		Title:       "Acme Frames",
		LogoURL:     "https://cdn.acme.example/logo.svg",
		LogoURLDark: "https://cdn.acme.example/logo-dark.svg",
		FaviconURL:  "/custom-favicon.svg",
		Theme: branding.Theme{
			"light": {"primary": "#0066cc"},
			"dark":  {"primary": "#3399ff"},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_EnvVarsOverlayConfigFile(t *testing.T) {
	path := writeConfig(t, `{
	  "title": "From File",
	  "logoUrl": "/file-logo.svg",
	  "faviconUrl": "/file-favicon.svg",
	  "theme": {"light": {"primary": "#111111"}}
	}`)

	cfg, err := branding.Load(envFunc(map[string]string{
		branding.EnvConfigFile: path,
		branding.EnvTitle:      "From Env",
	}), os.ReadFile)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Title != "From Env" {
		t.Errorf("Title = %q, want the env var to win", cfg.Title)
	}
	// Fields the env does not set keep the file's values.
	if cfg.LogoURL != "/file-logo.svg" || cfg.FaviconURL != "/file-favicon.svg" {
		t.Errorf("logo/favicon = %q/%q, want the file values", cfg.LogoURL, cfg.FaviconURL)
	}
	if got := cfg.Theme["light"]["primary"]; got != "#111111" {
		t.Errorf("theme.light.primary = %q, want the file value", got)
	}
}

func TestLoad_InvalidInputIsSkippedNotFatal(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "unreadable config file",
			env: map[string]string{
				branding.EnvConfigFile: filepath.Join(t.TempDir(), "missing.json"),
				branding.EnvTitle:      "Acme Frames",
			},
		},
		{
			name: "malformed config file",
			env: map[string]string{
				branding.EnvConfigFile: writeConfig(t, "not json"),
				branding.EnvTitle:      "Acme Frames",
			},
		},
		{
			name: "malformed theme JSON",
			env: map[string]string{
				branding.EnvTitle: "Acme Frames",
				branding.EnvTheme: `{"light": `,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := branding.Load(envFunc(tc.env), os.ReadFile)
			if err == nil {
				t.Fatal("Load() error = nil, want the skipped input reported")
			}
			// The rest of the document must still resolve so one bad value
			// cannot cost a deployment its whole branding.
			if cfg.Title != "Acme Frames" {
				t.Errorf("Title = %q, want the valid field to survive", cfg.Title)
			}
			if cfg.Theme["light"] != nil {
				t.Errorf("Theme = %+v, want the invalid theme dropped", cfg.Theme)
			}
		})
	}
}

func TestLoad_ReportsEveryProblem(t *testing.T) {
	cfg, err := branding.Load(envFunc(map[string]string{
		branding.EnvConfigFile: filepath.Join(t.TempDir(), "missing.json"),
		branding.EnvTheme:      "{",
	}), os.ReadFile)
	if err == nil {
		t.Fatal("Load() error = nil, want both problems reported")
	}
	// errors.Join wraps each problem; both must be recoverable from the result.
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		t.Fatalf("Load() error = %v, want a joined error", err)
	}
	if got := len(joined.Unwrap()); got != 2 {
		t.Fatalf("joined errors = %d, want 2 (%v)", got, err)
	}
	if !reflect.DeepEqual(cfg, branding.Config{}) {
		t.Fatalf("Load() = %+v, want zero Config", cfg)
	}
}

func TestLoad_NilReadFileDefaultsToOsReadFile(t *testing.T) {
	path := writeConfig(t, `{"title": "From File"}`)
	cfg, err := branding.Load(envFunc(map[string]string{branding.EnvConfigFile: path}), nil)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Title != "From File" {
		t.Fatalf("Title = %q, want %q", cfg.Title, "From File")
	}
}

func TestImageOrigins(t *testing.T) {
	tests := []struct {
		name string
		cfg  branding.Config
		want []string
	}{
		{
			name: "unbranded",
			cfg:  branding.Config{},
			want: nil,
		},
		{
			name: "root-relative paths need no origin",
			cfg:  branding.Config{LogoURL: "/logo.svg", FaviconURL: "/favicon.svg"},
			want: nil,
		},
		{
			name: "data URI needs no origin",
			cfg:  branding.Config{LogoURL: "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="},
			want: nil,
		},
		{
			name: "same host is listed once",
			cfg: branding.Config{
				LogoURL:     "https://cdn.acme.example/logo.svg",
				LogoURLDark: "https://cdn.acme.example/logo-dark.svg",
				FaviconURL:  "https://cdn.acme.example/favicon.svg",
			},
			want: []string{"https://cdn.acme.example"},
		},
		{
			name: "distinct hosts and schemes",
			cfg: branding.Config{
				LogoURL:    "https://cdn.acme.example/logo.svg",
				FaviconURL: "http://assets.acme.test:8080/favicon.svg",
			},
			want: []string{"https://cdn.acme.example", "http://assets.acme.test:8080"},
		},
		{
			name: "non-http scheme is dropped",
			cfg:  branding.Config{LogoURL: "javascript:alert(1)", FaviconURL: "ftp://acme.example/f.ico"},
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.ImageOrigins(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ImageOrigins() = %v, want %v", got, tc.want)
			}
		})
	}
}

// writeConfig writes body to a temp file and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
