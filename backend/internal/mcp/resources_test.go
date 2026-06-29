package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

type stubSource struct {
	readable []frames.ReadableFrame
	docs     map[string]*frames.Doc // key: org/name
	listErr  error
}

func (s stubSource) ListReadable(context.Context) ([]frames.ReadableFrame, error) {
	return s.readable, s.listErr
}
func (s stubSource) ResolveDoc(_ context.Context, org, name, _ string) (*frames.Doc, error) {
	d, ok := s.docs[org+"/"+name]
	if !ok {
		return nil, errors.New("not found") // mirror denied/missing -> error
	}
	return d, nil
}

func TestGetServer_DevModeBuildsServer(t *testing.T) {
	src := stubSource{
		readable: []frames.ReadableFrame{
			{OrgSlug: "openteams", OrgDisplay: "OpenTeams", Name: "alpha", Version: "1.0.0", Description: "A"},
		},
		docs: map[string]*frames.Doc{
			"openteams/alpha": {Name: "alpha", Description: "A", Version: "1.0.0", Slots: frames.Slots{Rules: []string{"r1"}}},
		},
	}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	req := httptest.NewRequest("POST", "/mcp", nil)
	srv := rs.getServer(req)
	if srv == nil {
		t.Fatal("getServer returned nil in dev mode")
	}
}

func TestReadHandler(t *testing.T) {
	src := stubSource{
		docs: map[string]*frames.Doc{
			"openteams/alpha": {Name: "alpha", Description: "A", Version: "1.0.0", Slots: frames.Slots{Rules: []string{"r1"}}},
		},
	}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.readHandler(auth.DevClaims())

	t.Run("reads composed markdown", func(t *testing.T) {
		req := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: formatFrameURI("openteams", "alpha", "1.0.0")}}
		res, err := h(context.Background(), req)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "# Frame: alpha") {
			t.Errorf("unexpected contents: %+v", res.Contents)
		}
		if res.Contents[0].MIMEType != "text/markdown" {
			t.Errorf("mime=%q", res.Contents[0].MIMEType)
		}
	})

	t.Run("unreadable and missing are identical not-found", func(t *testing.T) {
		missing := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: formatFrameURI("openteams", "ghost", "1.0.0")}}
		_, errMissing := h(context.Background(), missing)
		if errMissing == nil {
			t.Fatal("expected not-found for missing frame")
		}
		// A malformed URI must also be not-found (no distinct error surface).
		bad := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: "https://evil/x"}}
		_, errBad := h(context.Background(), bad)
		if errBad == nil {
			t.Fatal("expected not-found for malformed URI")
		}
	})
}

// A transient ListReadable failure must not produce a nil server: the SDK turns
// a nil server into HTTP 400 (client error), misrepresenting a backend fault.
// getServer should log and serve an empty resource set instead.
func TestGetServer_ListErrorServesEmptyNotNil(t *testing.T) {
	rs := &resourceServer{src: stubSource{listErr: errors.New("db down")}, cfg: Config{DevMode: true}}
	req := httptest.NewRequest("POST", "/mcp", nil)
	if srv := rs.getServer(req); srv == nil {
		t.Fatal("getServer returned nil on ListReadable error; want non-nil empty server (avoids HTTP 400)")
	}
}

// Constructing the bearer-protected endpoint without a validator is a wiring
// bug; Mount must fail fast at startup rather than panic at request time.
func TestMount_PanicsWhenNonDevAndNilVerifier(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic: non-dev mode with nil verifier")
		}
	}()
	c := NewComponent(Config{DevMode: false, PublicURL: "https://frames.example.com"}, stubSource{}, nil)
	c.Mount(http.NewServeMux())
}
