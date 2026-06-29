package server

import (
	"encoding/json"
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
	webui "github.com/nebari-dev/nebari-frames/web"
)

// Server wraps the combined HTTP mux that serves /healthz and the FrameService.
type Server struct{ handler http.Handler }

// Mounter mounts additional routes (e.g. the MCP endpoint) onto a mux. Taking
// this interface keeps the server package free of any dependency on the mcp
// package; *mcp.Component satisfies it.
type Mounter interface {
	Mount(*http.ServeMux)
}

// New creates a Server mounting /healthz, /readyz, /auth/config (unauthenticated),
// and the FrameService handler at its generated path. The auth interceptor is wired
// in for the FrameService. When devMode is true, requests pass through with stub
// claims and /readyz always returns 200. Pass a non-nil mcpMounter to also mount
// the MCP endpoint routes.
func New(repo store.Repository, validator auth.TokenValidator, authCfg auth.Config, devMode bool, mcpMounter Mounter) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", handleReadyz(readinessFunc(validator, devMode)))
	mux.HandleFunc("/auth/config", handleAuthConfig(authCfg))
	interceptor := auth.NewInterceptor(validator, devMode)
	path, handler := framesv1connect.NewFrameServiceHandler(
		frames.NewService(repo),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)
	if mcpMounter != nil {
		mcpMounter.Mount(mux)
	}
	mux.Handle("/", webui.NewHandler(webui.Assets(), webui.Config{IssuerURL: authCfg.IssuerURL}))
	return &Server{handler: mux}
}

// readinessFunc resolves how /readyz answers. Dev mode is always ready; in auth
// mode readiness tracks the validator if it reports it. The fallback (a non-dev
// validator that does not report readiness) defaults to not-ready, staying fail
// closed: /readyz only reports ready once a readiness-aware validator confirms
// it. This branch is unreachable in production, where the validator is always a
// *auth.LazyValidator.
func readinessFunc(v auth.TokenValidator, devMode bool) func() bool {
	if devMode {
		return func() bool { return true }
	}
	if rv, ok := v.(auth.ReadinessValidator); ok {
		return rv.Ready
	}
	return func() bool { return false }
}

func handleReadyz(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if ready() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	}
}

// Handler returns the underlying http.Handler for use with http.Server.
func (s *Server) Handler() http.Handler { return s.handler }

type authConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	IssuerURL      string `json:"issuer_url,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
	DeviceClientID string `json:"device_client_id,omitempty"`
}

func handleAuthConfig(cfg auth.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		resp := authConfigResponse{Enabled: cfg.IssuerURL != "" && cfg.ClientID != ""}
		if resp.Enabled {
			resp.IssuerURL = cfg.IssuerURL
			resp.ClientID = cfg.ClientID
			resp.DeviceClientID = cfg.DeviceClientID
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
