package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

// TestAuthAware verifies that commands surfacing CodeUnauthenticated produce
// the re-login prompt rather than a raw connect error.
func TestAuthAware(t *testing.T) {
	unauthErr := connect.NewError(connect.CodeUnauthenticated, nil)
	tests := []struct {
		name    string
		stub    *testutil.StubService
		args    []string
		wantMsg string
	}{
		{
			name: "show unauthenticated",
			stub: &testutil.StubService{
				GetFn: func(_ context.Context, _ *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error) {
					return nil, unauthErr
				},
			},
			args:    []string{"show", "openteams/brand-voice"},
			wantMsg: "run 'frames auth login'",
		},
		{
			name: "extends unauthenticated",
			stub: &testutil.StubService{
				GetFn: func(_ context.Context, _ *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error) {
					return nil, unauthErr
				},
			},
			args:    []string{"extends", "openteams/brand-voice"},
			wantMsg: "run 'frames auth login'",
		},
		{
			name: "resolve unauthenticated",
			stub: &testutil.StubService{
				ResolveFn: func(_ context.Context, _ *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error) {
					return nil, unauthErr
				},
			},
			args:    []string{"resolve", "openteams/brand-voice"},
			wantMsg: "run 'frames auth login'",
		},
		{
			name: "list unauthenticated",
			stub: &testutil.StubService{
				ListFn: func(_ context.Context, _ *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error) {
					return nil, unauthErr
				},
			},
			args:    []string{"list"},
			wantMsg: "run 'frames auth login'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := testutil.NewStubServer(t, tt.stub)
			c := NewRootCmd()
			c.SetArgs(append([]string{
				"--api-url", url,
				"--credentials-path", filepath.Join(t.TempDir(), "c.json"),
			}, tt.args...))
			err := c.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestShowAndExtends(t *testing.T) {
	getFn := func(_ context.Context, r *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error) {
		if r.Msg.Name == "missing" {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		}
		return connect.NewResponse(&framesv1.GetFrameResponse{
			Frame:   &framesv1.Frame{Name: "brand-voice", Description: "voice", OwnerSub: "u1"},
			Version: &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("name: brand-voice\n")},
			Extends: []*framesv1.ParentRef{{Ref: "openteams/base", Version: "1.0.0"}},
		}), nil
	}
	url := testutil.NewStubServer(t, &testutil.StubService{GetFn: getFn})

	if out := runCmd(t, url, "show", "openteams/brand-voice"); !strings.Contains(out, "brand-voice") || !strings.Contains(out, "openteams/base@1.0.0") {
		t.Fatalf("show output: %q", out)
	}
	if out := runCmd(t, url, "extends", "openteams/brand-voice"); !strings.Contains(out, "openteams/base@1.0.0") {
		t.Fatalf("extends output: %q", out)
	}
	// NotFound must not imply existence.
	c := NewRootCmd()
	c.SetArgs([]string{"--api-url", url, "--credentials-path", t.TempDir() + "/c.json", "show", "openteams/missing"})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not found (or you do not have access)") {
		t.Fatalf("want no-leak not-found error, got %v", err)
	}
}
