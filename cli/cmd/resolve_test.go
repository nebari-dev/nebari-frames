package cmd

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

func TestResolve(t *testing.T) {
	url := testutil.NewStubServer(t, &testutil.StubService{
		ResolveFn: func(_ context.Context, r *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error) {
			return connect.NewResponse(&framesv1.ResolveFrameResponse{ResolvedContent: []byte("name: brand-voice\nslots:\n  rules:\n    - merged\n")}), nil
		},
	})
	out := runCmd(t, url, "resolve", "openteams/brand-voice@1.0.0")
	if !strings.Contains(out, "merged") {
		t.Fatalf("resolve output: %q", out)
	}
}
