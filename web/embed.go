// Package webui embeds the compiled web SPA and serves it over HTTP with an
// index.html fallback for client-side routes.
package webui

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Assets returns the embedded SPA file tree rooted at dist/.
func Assets() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// dist is always embedded at build time; this cannot fail in practice.
		panic(err)
	}
	return sub
}

// Config carries runtime values the handler needs to build its CSP.
type Config struct {
	// IssuerURL is the OIDC issuer; its scheme://host origin is added to
	// connect-src so the browser can perform OIDC token exchange.
	IssuerURL string
}

// NewHandler returns an http.Handler that serves the SPA from fsys. Existing
// files are served directly; missing requests under /assets/ return 404, and
// any other missing path returns index.html (SPA deep-link fallback).
// fsys must return files whose Open result implements io.ReadSeeker; both
// embed.FS and fstest.MapFS satisfy this contract.
func NewHandler(fsys fs.FS, cfg Config) http.Handler {
	csp := buildCSP(cfg.IssuerURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, csp)

		upath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if upath == "" {
			serveIndex(w, fsys)
			return
		}

		f, err := fsys.Open(upath)
		if err == nil {
			defer func() { _ = f.Close() }()
			if info, statErr := f.Stat(); statErr == nil && !info.IsDir() {
				if rs, ok := f.(io.ReadSeeker); ok {
					if strings.HasPrefix(upath, "assets/") {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
					return
				}
			}
		}

		if strings.HasPrefix(upath, "assets/") {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, fsys)
	})
}

func serveIndex(w http.ResponseWriter, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "web app not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func setSecurityHeaders(w http.ResponseWriter, csp string) {
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
}

func buildCSP(issuerURL string) string {
	connectSrc := "'self'"
	if origin := originOf(issuerURL); origin != "" {
		connectSrc += " " + origin
	}
	return "default-src 'self'; connect-src " + connectSrc +
		"; img-src 'self' data:; style-src 'self' 'unsafe-inline'; base-uri 'self'; frame-ancestors 'none'"
}

// originOf returns the scheme://host of raw, or "" if raw is empty or unparseable.
func originOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
