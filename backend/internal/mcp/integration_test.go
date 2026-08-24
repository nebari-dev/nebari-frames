package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	mcppkg "github.com/nebari-dev/nebari-frames/backend/internal/mcp"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// stubVerifier satisfies auth.TokenValidator for non-dev wiring tests without a
// live OIDC provider. Tokenless requests are rejected by the bearer middleware
// before Validate is ever reached, so it only needs to be non-nil.
type stubVerifier struct{}

func (stubVerifier) Validate(context.Context, string) (*auth.Claims, error) {
	return nil, errors.New("stub: no validation")
}

// acceptingVerifier admits any token as the dev user, with a live expiry. The
// expiry matters: the bearer middleware treats a zero Expiration as expired and
// answers 401, which would make a body-size test pass without ever reaching the
// handler being tested.
type acceptingVerifier struct{}

func (acceptingVerifier) Validate(context.Context, string) (*auth.Claims, error) {
	c := auth.DevClaims()
	c.Expiry = time.Now().Add(time.Hour)
	return c, nil
}

// newTestServer builds an httptest server mounting only the MCP component. In
// non-dev mode it supplies a stub verifier (Mount requires one); the metadata
// and 401-challenge tests never present a token, and the dev-mode test sets
// DevMode:true so the verifier is not consulted.
func newTestServer(t *testing.T, cfg mcppkg.Config) (*httptest.Server, *store.Memory) {
	t.Helper()
	mem := store.NewMemory()
	seedOrgAndReadableFrame(t, mem)
	svc := frames.NewService(mem)
	var verifier auth.TokenValidator
	if !cfg.DevMode {
		verifier = stubVerifier{}
	}
	comp := mcppkg.NewComponent(cfg, svc, verifier)
	mux := http.NewServeMux()
	comp.Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, mem
}

// seedOrgAndReadableFrame seeds an org and a frame visible to any org member.
// The frame has an org-level read grant so the dev-user (via DevClaims) can see it.
func seedOrgAndReadableFrame(t *testing.T, mem *store.Memory) {
	t.Helper()
	ctx := context.Background()

	if err := mem.CreateOrg(ctx, &framesv1.Org{
		Id: "o1", Slug: "openteams", DisplayName: "OpenTeams",
	}); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	if err := mem.UpsertMembership(ctx, &framesv1.Membership{
		OrgId: "o1", UserSub: "dev-user", Role: "viewer",
	}); err != nil {
		t.Fatalf("UpsertMembership: %v", err)
	}

	if err := mem.CreateFrameVersion(ctx, store.CreateFrameVersionInput{
		Frame: &framesv1.Frame{
			Id: "f-alpha", OrgId: "o1", Name: "alpha", Description: "Alpha frame",
			OwnerSub: "someone", LatestVersion: "1.0.0",
			CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now(),
		},
		Version: &framesv1.FrameVersion{
			Version:     "1.0.0",
			Content:     []byte("name: alpha\ndescription: Alpha frame\nversion: 1.0.0\nbody: |\n  r1\n"),
			PublishedAt: timestamppb.Now(),
		},
		Grants:     []store.Grant{{SubjectType: "org", SubjectID: "o1", Permission: "read"}},
		IsNewFrame: true,
	}); err != nil {
		t.Fatalf("CreateFrameVersion: %v", err)
	}
}

func TestUnauthenticatedMCPReturns401Challenge(t *testing.T) {
	cfg := mcppkg.Config{
		PublicURL: "https://frames.example.com",
		IssuerURL: "https://kc/realms/x",
		DevMode:   false,
	}
	srv, _ := newTestServer(t, cfg)

	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	wa := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wa, "resource_metadata") {
		t.Errorf("WWW-Authenticate missing resource_metadata: %q", wa)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	cfg := mcppkg.Config{
		PublicURL: "https://frames.example.com",
		IssuerURL: "https://kc/realms/x",
	}
	srv, _ := newTestServer(t, cfg)

	resp, err := http.Get(srv.URL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("GET metadata: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var md struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if md.Resource != "https://frames.example.com/mcp" {
		t.Errorf("resource = %q, want https://frames.example.com/mcp", md.Resource)
	}
	if len(md.AuthorizationServers) == 0 || md.AuthorizationServers[0] != "https://kc/realms/x" {
		t.Errorf("authorization_servers = %v, want [https://kc/realms/x]", md.AuthorizationServers)
	}
	if len(md.ScopesSupported) == 0 {
		t.Errorf("scopes_supported is empty")
	}
}

func TestDevModeListAndRead(t *testing.T) {
	cfg := mcppkg.Config{
		PublicURL: "https://frames.example.com",
		DevMode:   true,
	}
	srv, _ := newTestServer(t, cfg)

	ctx := context.Background()

	resources := listResourcesViaSDK(t, ctx, srv.URL+"/mcp")
	if len(resources) == 0 {
		t.Fatal("expected at least one readable frame resource in dev mode")
	}

	body := readResourceViaSDK(t, ctx, srv.URL+"/mcp", resources[0].URI)
	if !strings.Contains(body, "# Frame:") {
		t.Errorf("read body is not composed markdown, got: %q", body)
	}
}

// listResourcesViaSDK connects a go-sdk MCP client and returns all listed resources.
func listResourcesViaSDK(t *testing.T, ctx context.Context, endpoint string) []*gomcp.Resource {
	t.Helper()
	session := connectSDK(t, ctx, endpoint)
	defer func() { _ = session.Close() }()

	res, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	return res.Resources
}

// readResourceViaSDK connects a go-sdk MCP client and returns the text of the first content item.
func readResourceViaSDK(t *testing.T, ctx context.Context, endpoint, uri string) string {
	t.Helper()
	session := connectSDK(t, ctx, endpoint)
	defer func() { _ = session.Close() }()

	rr, err := session.ReadResource(ctx, &gomcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource(%q): %v", uri, err)
	}
	if len(rr.Contents) == 0 {
		t.Fatalf("ReadResource returned no contents for %q", uri)
	}
	return rr.Contents[0].Text
}

// connectSDK builds and connects a go-sdk MCP client to the given endpoint.
func connectSDK(t *testing.T, ctx context.Context, endpoint string) *gomcp.ClientSession {
	t.Helper()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "test", Version: "v0"}, nil)
	transport := &gomcp.StreamableClientTransport{
		Endpoint:             endpoint,
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect to %s: %v", endpoint, err)
	}
	return session
}

// newWriteTestSession wires the real frames.Service to an in-process MCP client
// session in dev mode, with the dev user holding role in org o1. Nothing is
// stubbed on the permission path, so these tests fail if the MCP layer ever
// gains a shortcut around RBAC.
func newWriteTestSession(t *testing.T, role string) (*gomcp.ClientSession, *store.Memory) {
	t.Helper()
	ctx := context.Background()
	mem := store.NewMemory()
	seedOrgAndReadableFrame(t, mem)
	if err := mem.UpsertMembership(ctx, &framesv1.Membership{
		OrgId: "o1", UserSub: "dev-user", Role: role,
	}); err != nil {
		t.Fatalf("UpsertMembership: %v", err)
	}

	svc := frames.NewService(mem)
	comp := mcppkg.NewComponent(mcppkg.Config{DevMode: true, PublicURL: "https://frames.example.com"}, svc, nil)
	mux := http.NewServeMux()
	comp.Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := gomcp.NewClient(&gomcp.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, &gomcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, mem
}

// callTool invokes a tool and returns its text plus whether it reported an error.
func callTool(t *testing.T, cs *gomcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &gomcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*gomcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func TestMCPWritesEnforceRBAC(t *testing.T) {
	newFrame := map[string]any{
		"name":        "brand-voice",
		"description": "How we write",
		"version":     "1.0.0",
		"body":        "## Rules\n\n- Cite benchmarks.",
	}

	t.Run("a viewer cannot create a frame", func(t *testing.T) {
		cs, mem := newWriteTestSession(t, "viewer")
		text, isErr := callTool(t, cs, "create_frame", newFrame)
		if !isErr {
			t.Fatalf("viewer created a frame: %q", text)
		}
		if !strings.Contains(text, "permission denied") {
			t.Errorf("text = %q, want a permission denial", text)
		}
		if _, err := mem.GetFrameBySlugName(context.Background(), "openteams", "brand-voice"); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("a denied create still wrote a frame (err=%v)", err)
		}
	})

	t.Run("a publisher can create a frame and read it back", func(t *testing.T) {
		cs, mem := newWriteTestSession(t, "publisher")
		text, isErr := callTool(t, cs, "create_frame", newFrame)
		if isErr {
			t.Fatalf("publisher denied: %q", text)
		}
		if !strings.Contains(text, "brand-voice@1.0.0") {
			t.Errorf("text = %q, want the published name and version", text)
		}
		f, err := mem.GetFrameBySlugName(context.Background(), "openteams", "brand-voice")
		if err != nil {
			t.Fatalf("frame not persisted: %v", err)
		}
		if f.OwnerSub != "dev-user" {
			t.Errorf("owner = %q, want dev-user", f.OwnerSub)
		}
		// The new Frame is immediately readable through the read tool.
		got, isErr := callTool(t, cs, "get_frame", map[string]any{"name": "brand-voice"})
		if isErr || !strings.Contains(got, "Cite benchmarks.") {
			t.Errorf("get_frame after create = %q (isErr=%v)", got, isErr)
		}
	})

	t.Run("creating over an existing name fails", func(t *testing.T) {
		cs, _ := newWriteTestSession(t, "publisher")
		if _, isErr := callTool(t, cs, "create_frame", newFrame); isErr {
			t.Fatal("first create should succeed")
		}
		// A different version, so this can only be the create-intent check and
		// not the version-uniqueness check.
		second := map[string]any{
			"name": "brand-voice", "description": "How we write", "version": "2.0.0",
			"body": "## Rules\n\n- Cite benchmarks.",
		}
		text, isErr := callTool(t, cs, "create_frame", second)
		if !isErr {
			t.Fatalf("second create succeeded: %q", text)
		}
		if !strings.Contains(text, "already exists") {
			t.Errorf("text = %q, want an already-exists error", text)
		}
	})

	t.Run("the owner can update their frame", func(t *testing.T) {
		cs, mem := newWriteTestSession(t, "publisher")
		if _, isErr := callTool(t, cs, "create_frame", newFrame); isErr {
			t.Fatal("create should succeed")
		}
		updated := map[string]any{
			"name":         "brand-voice",
			"description":  "How we write, revised",
			"version":      "1.1.0",
			"base_version": "1.0.0",
			"body":         "## Rules\n\n- Cite benchmarks.\n- Avoid jargon.",
			"changelog":    "added a rule",
		}
		text, isErr := callTool(t, cs, "update_frame", updated)
		if isErr {
			t.Fatalf("owner denied update: %q", text)
		}
		f, err := mem.GetFrameBySlugName(context.Background(), "openteams", "brand-voice")
		if err != nil {
			t.Fatalf("get frame: %v", err)
		}
		if f.LatestVersion != "1.1.0" {
			t.Errorf("latest version = %q, want 1.1.0", f.LatestVersion)
		}
	})

	t.Run("updating a frame the caller cannot edit is denied", func(t *testing.T) {
		// alpha is seeded owned by "someone" with only an org-level read grant.
		cs, _ := newWriteTestSession(t, "publisher")
		text, isErr := callTool(t, cs, "update_frame", map[string]any{
			"name":         "alpha",
			"description":  "hijacked",
			"version":      "2.0.0",
			"base_version": "1.0.0",
			"body":         "## Rules\n\n- mine now",
		})
		if !isErr {
			t.Fatalf("edit permission was bypassed: %q", text)
		}
		if !strings.Contains(text, "permission denied") {
			t.Errorf("text = %q, want a permission denial", text)
		}
	})

	t.Run("updating an unknown frame reports not found", func(t *testing.T) {
		cs, _ := newWriteTestSession(t, "publisher")
		text, isErr := callTool(t, cs, "update_frame", map[string]any{
			"name": "ghost", "description": "x",
			"version": "1.0.0", "base_version": "1.0.0",
			"body": "- r",
		})
		if !isErr {
			t.Fatalf("update of an unknown frame succeeded: %q", text)
		}
		if !strings.Contains(text, "not found") {
			t.Errorf("text = %q, want a not-found error", text)
		}
	})

	t.Run("an invalid document is rejected by the canonical validator", func(t *testing.T) {
		cs, mem := newWriteTestSession(t, "publisher")
		text, isErr := callTool(t, cs, "create_frame", map[string]any{
			"name":        "Not A Valid Name",
			"description": "x",
			"version":     "1.0.0",
			"body":        "- r",
		})
		if !isErr {
			t.Fatalf("invalid name accepted: %q", text)
		}
		if !strings.Contains(text, "invalid frame") {
			t.Errorf("text = %q, want an invalid-frame error", text)
		}
		if _, err := mem.GetFrameBySlugName(context.Background(), "openteams", "Not A Valid Name"); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("an invalid create still wrote something (err=%v)", err)
		}
	})
}

// An update must not destroy what the caller did not mention. Absent fields keep
// their current values; supplied fields replace them; an explicitly empty list
// clears. Without this, an AI that updates only the body silently wipes the
// Frame's visibility, maintainer, and - worst - its inheritance edges.
func TestMCPUpdatePreservesOmittedFields(t *testing.T) {
	cs, mem := newWriteTestSession(t, "publisher")
	ctx := context.Background()

	// A parent to inherit from, then a child that pins it and carries metadata.
	if _, isErr := callTool(t, cs, "create_frame", map[string]any{
		"name": "base", "description": "Base", "version": "1.0.0",
		"body": "## Rules\n\n- from parent",
	}); isErr {
		t.Fatal("create base failed")
	}
	if _, isErr := callTool(t, cs, "create_frame", map[string]any{
		"name": "child", "description": "Child", "version": "1.0.0",
		"body":       "## Rules\n\n- from child\n\n## Goals\n\nship the thing",
		"visibility": "private",
		"scope":      "company",
		"maintainer": "platform team",
		"extends":    []any{map[string]any{"ref": "openteams/base", "version": "1.0.0"}},
	}); isErr {
		t.Fatal("create child failed")
	}

	// Update only the body. Everything else must survive.
	text, isErr := callTool(t, cs, "update_frame", map[string]any{
		"name": "child", "version": "1.1.0", "base_version": "1.0.0",
		"body": "## Rules\n\n- from child\n- and another\n\n## Goals\n\nship the thing",
	})
	if isErr {
		t.Fatalf("update failed: %q", text)
	}

	v, _, _, err := mem.GetFrameVersion(ctx, mustFrameID(t, mem, "child"), "1.1.0")
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	doc, err := frames.Parse(v.Content)
	if err != nil {
		t.Fatalf("parse stored content: %v", err)
	}

	if doc.Visibility != "private" {
		t.Errorf("visibility = %q, want private (omitted fields must be preserved)", doc.Visibility)
	}
	if doc.Scope != "company" {
		t.Errorf("scope = %q, want company", doc.Scope)
	}
	if doc.Maintainer != "platform team" {
		t.Errorf("maintainer = %q, want %q", doc.Maintainer, "platform team")
	}
	if len(doc.Extends) != 1 || doc.Extends[0].Ref != "openteams/base" || doc.Extends[0].Version != "1.0.0" {
		t.Errorf("extends = %+v, want the pinned parent preserved: inheritance must survive an update", doc.Extends)
	}
	if !strings.Contains(doc.Body, "ship the thing") {
		t.Errorf("body = %q, want the original prose preserved", doc.Body)
	}
	if doc.Description != "Child" {
		t.Errorf("description = %q, want Child", doc.Description)
	}
	// The parent's rule must NOT have been copied into the child.
	if !strings.Contains(doc.Body, "and another") {
		t.Errorf("body = %q, want the supplied rules", doc.Body)
	}
	if strings.Contains(doc.Body, "from parent") {
		t.Errorf("parent content was flattened into the child: %q", doc.Body)
	}

	t.Run("supplied fields replace, and an explicit empty list clears", func(t *testing.T) {
		text, isErr := callTool(t, cs, "update_frame", map[string]any{
			"name": "child", "version": "1.2.0", "base_version": "1.1.0",
			"maintainer": "data team",
			"extends":    []any{},
		})
		if isErr {
			t.Fatalf("update failed: %q", text)
		}
		v, _, _, err := mem.GetFrameVersion(ctx, mustFrameID(t, mem, "child"), "1.2.0")
		if err != nil {
			t.Fatalf("get version: %v", err)
		}
		doc, err := frames.Parse(v.Content)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if doc.Maintainer != "data team" {
			t.Errorf("maintainer = %q, want the supplied value", doc.Maintainer)
		}
		if len(doc.Extends) != 0 {
			t.Errorf("extends = %+v, want cleared by the explicit empty list", doc.Extends)
		}
		if doc.Visibility != "private" {
			t.Errorf("visibility = %q, still want private (untouched)", doc.Visibility)
		}
	})
}

func mustFrameID(t *testing.T, mem *store.Memory, name string) string {
	t.Helper()
	f, err := mem.GetFrameBySlugName(context.Background(), "openteams", name)
	if err != nil {
		t.Fatalf("frame %q: %v", name, err)
	}
	return f.Id
}

// The most likely instruction this tool will ever get is "add a rule to X".
// Doing that requires reading the Frame's current body, and if the only read
// available returns the inheritance-composed form, the model has no choice but
// to send the parent's content back as the child's own - which validates, looks
// identical when composed, and silently detaches the child from its parent's
// future revisions.
func TestMCPReadModifyWriteDoesNotFlattenInheritance(t *testing.T) {
	cs, mem := newWriteTestSession(t, "publisher")
	ctx := context.Background()

	if _, isErr := callTool(t, cs, "create_frame", map[string]any{
		"name": "company-base", "description": "Company", "version": "1.0.0",
		"body": "## Rules\n\n- Use inclusive language.\n\n## Goals\n\nGrow the platform.",
	}); isErr {
		t.Fatal("create parent failed")
	}
	if _, isErr := callTool(t, cs, "create_frame", map[string]any{
		"name": "team-api", "description": "API team", "version": "1.0.0",
		"body":    "## Rules\n\n- Version every endpoint.",
		"extends": []any{map[string]any{"ref": "openteams/company-base", "version": "1.0.0"}},
	}); isErr {
		t.Fatal("create child failed")
	}

	// A source read must exist, and must return only the child's own content.
	src, isErr := callTool(t, cs, "get_frame", map[string]any{"name": "team-api", "source": true})
	if isErr {
		t.Fatalf("get_frame source mode failed: %q", src)
	}
	if strings.Contains(src, "Use inclusive language.") {
		t.Errorf("source read leaked inherited content, so a model editing it would copy the parent in:\n%s", src)
	}
	if !strings.Contains(src, "Version every endpoint.") {
		t.Errorf("source read is missing the frame's own rule:\n%s", src)
	}
	if strings.Contains(src, "Grow the platform.") {
		t.Errorf("source read leaked the parent's prose:\n%s", src)
	}

	// The default read stays composed, which is what a consumer wants.
	composed, isErr := callTool(t, cs, "get_frame", map[string]any{"name": "team-api"})
	if isErr {
		t.Fatalf("get_frame failed: %q", composed)
	}
	if !strings.Contains(composed, "Use inclusive language.") {
		t.Errorf("default read should still compose inherited content:\n%s", composed)
	}

	// Editing from the source read leaves inheritance intact and un-flattened.
	if _, isErr := callTool(t, cs, "update_frame", map[string]any{
		"name": "team-api", "version": "1.1.0", "base_version": "1.0.0",
		"body": "## Rules\n\n- Version every endpoint.\n- Prefer cursor pagination.",
	}); isErr {
		t.Fatal("update failed")
	}
	v, _, _, err := mem.GetFrameVersion(ctx, mustFrameID(t, mem, "team-api"), "1.1.0")
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	doc, err := frames.Parse(v.Content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Contains(doc.Body, "Use inclusive language.") {
		t.Errorf("the parent's rule was copied into the child: %q", doc.Body)
	}
	if strings.Contains(doc.Body, "Grow the platform.") {
		t.Errorf("body = %q: the parent's prose must not be frozen into the child", doc.Body)
	}
	if len(doc.Extends) != 1 {
		t.Errorf("extends = %+v, want the parent still pinned", doc.Extends)
	}
}

// The SDK reads a request body in full before any tool handler runs, so the
// body limit is the only thing standing between an authenticated caller and the
// memory of a single-replica deployment.
//
// Both auth modes are covered on purpose: the bearer middleware is installed
// only when DevMode is false, and wrapping the wrong handler there silently
// drops the cap in exactly the deployments that need it.
func TestMCPRejectsOversizedRequestBody(t *testing.T) {
	tests := []struct {
		name     string
		devMode  bool
		verifier auth.TokenValidator
		token    string
	}{
		{name: "dev mode", devMode: true},
		{name: "with bearer auth", devMode: false, verifier: acceptingVerifier{}, token: "any-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := store.NewMemory()
			seedOrgAndReadableFrame(t, mem)
			comp := mcppkg.NewComponent(
				mcppkg.Config{DevMode: tt.devMode, PublicURL: "https://frames.example.com"},
				frames.NewService(mem), tt.verifier)
			mux := http.NewServeMux()
			comp.Mount(mux)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			post := func(size int) int {
				t.Helper()
				body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
					`{"protocolVersion":"2025-06-18","capabilities":{},` +
					`"clientInfo":{"name":"probe","version":"1"},"padding":"` +
					strings.Repeat("a", size) + `"}}`
				req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(body))
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				if tt.token != "" {
					req.Header.Set("Authorization", "Bearer "+tt.token)
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					// A connection reset is an acceptable refusal.
					return http.StatusRequestEntityTooLarge
				}
				defer func() { _ = resp.Body.Close() }()
				return resp.StatusCode
			}

			// Control: a small body must succeed, so a refusal below cannot be
			// mistaken for an unrelated rejection such as a 401.
			if got := post(100); got >= 400 {
				t.Fatalf("small body rejected with %d; the test is not reaching the handler", got)
			}
			if got := post(mcppkg.MaxRequestBytes + 1024); got < 400 {
				t.Errorf("oversized body accepted with %d: the body cap is not in force on this path", got)
			}
		})
	}
}
