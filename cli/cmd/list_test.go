package cmd

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

func TestList(t *testing.T) {
	tests := []struct {
		name    string
		frames  []*framesv1.FrameSummary
		wantSub string
	}{
		{"renders rows", []*framesv1.FrameSummary{{Name: "brand-voice", LatestVersion: "1.0.0", OwnerSub: "u1", Description: "voice"}}, "brand-voice"},
		{"empty", nil, "No frames found."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := testutil.NewStubServer(t, &testutil.StubService{
				ListFn: func(_ context.Context, _ *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error) {
					return connect.NewResponse(&framesv1.ListFramesResponse{Frames: tt.frames}), nil
				},
			})
			out := runCmd(t, url, "list")
			if !strings.Contains(out, tt.wantSub) {
				t.Fatalf("output %q missing %q", out, tt.wantSub)
			}
		})
	}
}
