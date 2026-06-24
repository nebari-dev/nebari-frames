package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

func TestPublish(t *testing.T) {
	url := testutil.NewStubServer(t, &testutil.StubService{
		PublishFn: func(_ context.Context, r *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
			if len(r.Msg.Content) == 0 {
				return nil, connect.NewError(connect.CodeInvalidArgument, nil)
			}
			return connect.NewResponse(&framesv1.PublishFrameResponse{
				Frame:   &framesv1.Frame{Name: "brand-voice"},
				Version: &framesv1.FrameVersion{Version: "1.0.0"},
			}), nil
		},
	})

	tests := []struct {
		name      string
		writeFile bool
		wantErr   string
		wantOut   string
	}{
		{"publishes frame.yaml", true, "", "Published brand-voice@1.0.0"},
		{"missing frame.yaml", false, "no frame.yaml found", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.writeFile {
				if err := os.WriteFile(filepath.Join(dir, "frame.yaml"), []byte("name: brand-voice\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			c := NewRootCmd()
			var buf bytes.Buffer
			c.SetOut(&buf)
			c.SetErr(&buf)
			c.SetArgs([]string{"--api-url", url, "--credentials-path", filepath.Join(t.TempDir(), "c.json"), "publish", "--dir", dir})
			err := c.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(buf.String(), tt.wantOut) {
				t.Fatalf("output %q missing %q", buf.String(), tt.wantOut)
			}
		})
	}
}
