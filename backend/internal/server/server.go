package server

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

// Server wraps the combined HTTP mux that serves /healthz and the FrameService.
type Server struct{ handler http.Handler }

// New creates a Server mounting /healthz and the FrameService handler at its
// generated path. The auth interceptor is wired in; passing nil for validator
// enables dev mode (requests pass through with stub claims).
func New(repo store.Repository, validator auth.TokenValidator) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	interceptor := auth.NewInterceptor(validator)
	path, handler := framesv1connect.NewFrameServiceHandler(
		frames.NewService(repo),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)
	return &Server{handler: mux}
}

// Handler returns the underlying http.Handler for use with http.Server.
func (s *Server) Handler() http.Handler { return s.handler }
