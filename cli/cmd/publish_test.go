package cmd

import (
	"bytes"
	"context"
	"errors"
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

// recordTemplateID captures the template_id the CLI actually sent, which is the
// whole point of the publish tests: the flag has to reach the wire.
func recordTemplateID(into *string) func(context.Context, *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
	return func(_ context.Context, r *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
		*into = r.Msg.TemplateId
		return connect.NewResponse(&framesv1.PublishFrameResponse{
			Frame:   &framesv1.Frame{Name: "vocab"},
			Version: &framesv1.FrameVersion{Version: "1.0.0"},
		}), nil
	}
}

func TestPublishPassesTheTemplateID(t *testing.T) {
	var got string
	url := testutil.NewStubServer(t, &testutil.StubService{PublishFn: recordTemplateID(&got)})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frame.yaml"),
		[]byte("name: vocab\ndescription: Our terms\nversion: 1.0.0\nslots: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmdErr(t, url, "publish", "--dir", dir, "--template", "builtin:domain-vocabulary"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got != "builtin:domain-vocabulary" {
		t.Errorf("server saw template_id %q, want the flag's value", got)
	}
}

func TestPublishWithoutTemplateSendsNothing(t *testing.T) {
	var got string
	url := testutil.NewStubServer(t, &testutil.StubService{PublishFn: recordTemplateID(&got)})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frame.yaml"),
		[]byte("name: vocab\ndescription: Our terms\nversion: 1.0.0\nslots: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmdErr(t, url, "publish", "--dir", dir); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got != "" {
		t.Errorf("server saw template_id %q, want empty", got)
	}
}

// A second publish of a scaffolded Frame is the likeliest way to meet this
// error: the scaffold's header names --template, the flag asserts a new Frame,
// and the bare "already_exists" the server returns says nothing about the flag
// that caused it.
func TestPublishAlreadyExistsPointsAtTheTemplateFlag(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    string
		wantAbsent string
	}{
		{
			name:    "with --template, the flag is named as the cause",
			args:    []string{"--template", "builtin:domain-vocabulary"},
			wantErr: "--template",
		},
		{
			name: "without it, no flag is blamed",
			args: nil,
			// Suggesting a flag the author never passed would send them
			// looking for a command they did not run.
			wantErr:    "already",
			wantAbsent: "--template",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := testutil.NewStubServer(t, &testutil.StubService{
				PublishFn: func(context.Context, *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
					return nil, connect.NewError(connect.CodeAlreadyExists,
						errors.New(`a frame named "vocab" already exists; update it instead`))
				},
			})
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "frame.yaml"),
				[]byte("name: vocab\ndescription: Our terms\nversion: 2.0.0\nslots: {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"publish", "--dir", dir}, tt.args...)
			_, err := runCmdErr(t, url, args...)
			if err == nil {
				t.Fatal("publish succeeded against a server that refused it")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
			if tt.wantAbsent != "" && strings.Contains(err.Error(), tt.wantAbsent) {
				t.Errorf("error %q should not mention %q", err, tt.wantAbsent)
			}
		})
	}
}
