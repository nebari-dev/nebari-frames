// Package testutil provides an in-process stub FrameService for CLI tests.
package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

// StubService implements framesv1connect.FrameServiceHandler via optional func
// fields. Unset methods return CodeUnimplemented.
type StubService struct {
	PublishFn func(context.Context, *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error)
	ListFn    func(context.Context, *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error)
	GetFn     func(context.Context, *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error)
	ResolveFn func(context.Context, *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error)
	MeFn      func(context.Context, *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error)
}

var _ framesv1connect.FrameServiceHandler = (*StubService)(nil)

func unimpl() error { return connect.NewError(connect.CodeUnimplemented, nil) }

func (s *StubService) PublishFrame(ctx context.Context, r *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
	if s.PublishFn != nil {
		return s.PublishFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) ListFrames(ctx context.Context, r *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error) {
	if s.ListFn != nil {
		return s.ListFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) GetFrame(ctx context.Context, r *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error) {
	if s.GetFn != nil {
		return s.GetFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) ResolveFrame(ctx context.Context, r *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error) {
	if s.ResolveFn != nil {
		return s.ResolveFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) GetMe(ctx context.Context, r *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error) {
	if s.MeFn != nil {
		return s.MeFn(ctx, r)
	}
	return nil, unimpl()
}

// NewStubServer mounts h on an httptest.Server and returns the base URL.
func NewStubServer(t *testing.T, h framesv1connect.FrameServiceHandler) string {
	t.Helper()
	path, handler := framesv1connect.NewFrameServiceHandler(h)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}
