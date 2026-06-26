package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	mcppkg "github.com/nebari-dev/nebari-frames/backend/internal/mcp"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// newTestServer builds an httptest server mounting only the MCP component.
// Pass nil verifier - tests 1 and 2 use non-dev cfg but never present a token;
// test 3 uses DevMode:true so the verifier is not consulted.
func newTestServer(t *testing.T, cfg mcppkg.Config) (*httptest.Server, *store.Memory) {
	t.Helper()
	mem := store.NewMemory()
	seedOrgAndReadableFrame(t, mem)
	svc := frames.NewService(mem)
	comp := mcppkg.NewComponent(cfg, svc, nil)
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
			Content:     []byte("name: alpha\ndescription: Alpha frame\nversion: 1.0.0\nslots:\n  rules:\n    - r1\n"),
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer session.Close()

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
	defer session.Close()

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
