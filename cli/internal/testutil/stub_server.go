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
	PublishFn        func(context.Context, *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error)
	ListFn           func(context.Context, *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error)
	GetFn            func(context.Context, *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error)
	ResolveFn        func(context.Context, *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error)
	MeFn             func(context.Context, *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error)
	ListVersionsFn   func(context.Context, *connect.Request[framesv1.ListFrameVersionsRequest]) (*connect.Response[framesv1.ListFrameVersionsResponse], error)
	ListTemplatesFn  func(context.Context, *connect.Request[framesv1.ListFrameTemplatesRequest]) (*connect.Response[framesv1.ListFrameTemplatesResponse], error)
	GetTemplateFn    func(context.Context, *connect.Request[framesv1.GetFrameTemplateRequest]) (*connect.Response[framesv1.GetFrameTemplateResponse], error)
	CreateTemplateFn func(context.Context, *connect.Request[framesv1.CreateFrameTemplateRequest]) (*connect.Response[framesv1.CreateFrameTemplateResponse], error)
	UpdateTemplateFn func(context.Context, *connect.Request[framesv1.UpdateFrameTemplateRequest]) (*connect.Response[framesv1.UpdateFrameTemplateResponse], error)
	DeleteTemplateFn func(context.Context, *connect.Request[framesv1.DeleteFrameTemplateRequest]) (*connect.Response[framesv1.DeleteFrameTemplateResponse], error)
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

func (s *StubService) ListFrameVersions(ctx context.Context, r *connect.Request[framesv1.ListFrameVersionsRequest]) (*connect.Response[framesv1.ListFrameVersionsResponse], error) {
	if s.ListVersionsFn != nil {
		return s.ListVersionsFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) DeleteFrame(_ context.Context, r *connect.Request[framesv1.DeleteFrameRequest]) (*connect.Response[framesv1.DeleteFrameResponse], error) {
	return nil, unimpl()
}

// ConvertFrame is web-only; the CLI keeps publishing frame.yaml directly.
func (s *StubService) ConvertFrame(_ context.Context, r *connect.Request[framesv1.ConvertFrameRequest]) (*connect.Response[framesv1.ConvertFrameResponse], error) {
	return nil, unimpl()
}

func (s *StubService) ListOrgMembers(_ context.Context, r *connect.Request[framesv1.ListOrgMembersRequest]) (*connect.Response[framesv1.ListOrgMembersResponse], error) {
	return nil, unimpl()
}

func (s *StubService) AddOrgMember(_ context.Context, r *connect.Request[framesv1.AddOrgMemberRequest]) (*connect.Response[framesv1.AddOrgMemberResponse], error) {
	return nil, unimpl()
}

func (s *StubService) SetMemberRole(_ context.Context, r *connect.Request[framesv1.SetMemberRoleRequest]) (*connect.Response[framesv1.SetMemberRoleResponse], error) {
	return nil, unimpl()
}

func (s *StubService) RemoveOrgMember(_ context.Context, r *connect.Request[framesv1.RemoveOrgMemberRequest]) (*connect.Response[framesv1.RemoveOrgMemberResponse], error) {
	return nil, unimpl()
}

func (s *StubService) ListFrameTemplates(ctx context.Context, r *connect.Request[framesv1.ListFrameTemplatesRequest]) (*connect.Response[framesv1.ListFrameTemplatesResponse], error) {
	if s.ListTemplatesFn != nil {
		return s.ListTemplatesFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) GetFrameTemplate(ctx context.Context, r *connect.Request[framesv1.GetFrameTemplateRequest]) (*connect.Response[framesv1.GetFrameTemplateResponse], error) {
	if s.GetTemplateFn != nil {
		return s.GetTemplateFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) CreateFrameTemplate(ctx context.Context, r *connect.Request[framesv1.CreateFrameTemplateRequest]) (*connect.Response[framesv1.CreateFrameTemplateResponse], error) {
	if s.CreateTemplateFn != nil {
		return s.CreateTemplateFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) UpdateFrameTemplate(ctx context.Context, r *connect.Request[framesv1.UpdateFrameTemplateRequest]) (*connect.Response[framesv1.UpdateFrameTemplateResponse], error) {
	if s.UpdateTemplateFn != nil {
		return s.UpdateTemplateFn(ctx, r)
	}
	return nil, unimpl()
}

func (s *StubService) DeleteFrameTemplate(ctx context.Context, r *connect.Request[framesv1.DeleteFrameTemplateRequest]) (*connect.Response[framesv1.DeleteFrameTemplateResponse], error) {
	if s.DeleteTemplateFn != nil {
		return s.DeleteTemplateFn(ctx, r)
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
