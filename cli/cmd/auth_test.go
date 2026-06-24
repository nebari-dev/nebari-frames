package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

func TestAuthStatus(t *testing.T) {
	url := testutil.NewStubServer(t, &testutil.StubService{
		MeFn: func(_ context.Context, _ *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error) {
			return connect.NewResponse(&framesv1.GetMeResponse{Subject: "u1", Role: "publisher", Org: &framesv1.Org{Slug: "openteams"}}), nil
		},
	})
	out := runCmd(t, url, "auth", "status")
	if !bytes.Contains([]byte(out), []byte("u1")) || !bytes.Contains([]byte(out), []byte("openteams")) {
		t.Fatalf("status output: %q", out)
	}
}

func TestAuthLogout(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := auth.SaveToken(credPath, &auth.CachedToken{IDToken: "x", Expiry: auth.FarFuture()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	c := NewRootCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetArgs([]string{"--credentials-path", credPath, "auth", "logout"})
	if err := c.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tok, _ := auth.LoadTokenRaw(credPath); tok != nil {
		t.Fatal("credentials not cleared")
	}
}

// runCmd executes the root command with --api-url set to the stub and returns combined output.
func runCmd(t *testing.T, apiURL string, args ...string) string {
	t.Helper()
	c := NewRootCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	full := append([]string{"--api-url", apiURL, "--credentials-path", filepath.Join(t.TempDir(), "c.json")}, args...)
	c.SetArgs(full)
	if err := c.Execute(); err != nil {
		t.Fatalf("execute %v: %v\noutput: %s", args, err, buf.String())
	}
	return buf.String()
}
