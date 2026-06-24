package cmd

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

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
