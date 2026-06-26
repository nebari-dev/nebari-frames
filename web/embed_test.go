package webui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	webui "github.com/nebari-dev/nebari-frames/web"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>app</title>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
		"favicon.ico":   {Data: []byte("icondata")},
	}
}

func TestHandler_Serving(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantCode    int
		wantBodyHas string
	}{
		{name: "root serves index", path: "/", wantCode: 200, wantBodyHas: "<title>app</title>"},
		{name: "known asset served", path: "/assets/app.js", wantCode: 200, wantBodyHas: "console.log"},
		{name: "top-level file served", path: "/favicon.ico", wantCode: 200, wantBodyHas: "icondata"},
		{name: "client route falls back to index", path: "/frames/acme/voice", wantCode: 200, wantBodyHas: "<title>app</title>"},
		{name: "missing asset is 404", path: "/assets/missing.js", wantCode: 404, wantBodyHas: ""},
	}
	h := webui.NewHandler(testFS(), webui.Config{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.wantCode {
				t.Fatalf("%s = %d, want %d", tc.path, rr.Code, tc.wantCode)
			}
			if tc.wantBodyHas != "" && !strings.Contains(rr.Body.String(), tc.wantBodyHas) {
				t.Fatalf("body for %s missing %q: %s", tc.path, tc.wantBodyHas, rr.Body.String())
			}
		})
	}
}

func TestHandler_SecurityHeaders(t *testing.T) {
	tests := []struct {
		name           string
		cfg            webui.Config
		wantConnectSrc string
	}{
		{name: "issuer set adds origin to connect-src", cfg: webui.Config{IssuerURL: "https://oidc.example.com/realms/x"}, wantConnectSrc: "connect-src 'self' https://oidc.example.com;"},
		{name: "dev mode connect-src self only", cfg: webui.Config{}, wantConnectSrc: "connect-src 'self';"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := webui.NewHandler(testFS(), tc.cfg)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			csp := rr.Header().Get("Content-Security-Policy")
			if !strings.Contains(csp, tc.wantConnectSrc) {
				t.Fatalf("CSP %q missing %q", csp, tc.wantConnectSrc)
			}
			if !strings.HasPrefix(csp, "default-src 'self';") {
				t.Fatalf("CSP missing default-src: %q", csp)
			}
			if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := rr.Header().Get("Referrer-Policy"); got != "same-origin" {
				t.Fatalf("Referrer-Policy = %q, want same-origin", got)
			}
		})
	}
}

func TestAssets_HasIndex(t *testing.T) {
	if _, err := webui.Assets().Open("index.html"); err != nil {
		t.Fatalf("embedded assets missing index.html: %v", err)
	}
}
