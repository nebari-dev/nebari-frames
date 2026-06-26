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

// New creates a Server mounting /healthz, /auth/config (unauthenticated), and
// the FrameService handler at its generated path. The auth interceptor is wired
// in for the FrameService; passing nil for validator enables dev mode (requests
// pass through with stub claims).
func New(repo store.Repository, validator auth.TokenValidator, authCfg auth.Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/auth/config", handleAuthConfig(authCfg))
	interceptor := auth.NewInterceptor(validator)
	path, handler := framesv1connect.NewFrameServiceHandler(
		frames.NewService(repo),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)
	mux.Handle("/", webui.NewHandler(webui.Assets(), webui.Config{IssuerURL: authCfg.IssuerURL}))
	return &Server{handler: mux}
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
