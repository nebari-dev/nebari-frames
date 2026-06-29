package mcp

import (
	"context"
	"log"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// compile-time assertion that the real service satisfies the adapter's interface.
var _ FrameSource = (*frames.Service)(nil)

// FrameSource is the subset of frames.Service the MCP adapter needs. Taking an
// interface keeps the adapter testable with a stub. *frames.Service satisfies it.
type FrameSource interface {
	ListReadable(ctx context.Context) ([]frames.ReadableFrame, error)
	ResolveDoc(ctx context.Context, orgSlug, name, version string) (*frames.Doc, error)
}

type resourceServer struct {
	src FrameSource
	cfg Config
}

// claimsFor resolves the caller identity for a request. In dev mode it returns
// fixed dev claims; otherwise it reads the claims stashed by the bearer verifier.
func (rs *resourceServer) claimsFor(req *http.Request) *auth.Claims {
	if rs.cfg.DevMode {
		return auth.DevClaims()
	}
	if c, ok := claimsFromTokenInfo(mcpauth.TokenInfoFromContext(req.Context())); ok {
		return c
	}
	return nil
}

// getServer builds a per-request MCP server whose resources are the caller's
// RBAC-readable frames. The list is rebuilt every request (not cached) so a
// revoked grant takes effect immediately. Invoked by the Streamable HTTP
// handler per request. Returns nil (-> 400) only when the caller cannot be
// identified; a transient listing failure is logged and serves no resources.
func (rs *resourceServer) getServer(req *http.Request) *gomcp.Server {
	claims := rs.claimsFor(req)
	if claims == nil {
		return nil
	}
	ctx := auth.WithClaims(req.Context(), claims)
	readable, err := rs.src.ListReadable(ctx)
	if err != nil {
		// The SDK turns a nil server into HTTP 400 (client error), which would
		// misrepresent a backend fault. Log it and serve an empty resource set.
		log.Printf("mcp: getServer: ListReadable failed, serving no resources: %v", err)
		readable = nil
	}
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "nebari-frames", Version: "v1"}, nil)
	read := rs.readHandler(claims)
	for _, f := range readable {
		srv.AddResource(&gomcp.Resource{
			URI:         formatFrameURI(f.OrgSlug, f.Name, f.Version),
			Name:        f.Name + " (" + f.OrgDisplay + ")",
			Description: f.Description,
			MIMEType:    "text/markdown",
		}, read)
	}
	// Template lets clients read explicitly-versioned URIs not in the latest list.
	srv.AddResourceTemplate(&gomcp.ResourceTemplate{
		URITemplate: frameURIScheme + "{org}/{name}",
		Name:        "Nebari Frame",
		MIMEType:    "text/markdown",
	}, read)
	return srv
}

// readHandler resolves and composes a single frame. It closes over the caller's
// claims (the per-request server is short-lived) and re-attaches them to the
// handler context so frames.Service RBAC sees the right identity. Denied,
// missing, and malformed-URI reads all return an identical ResourceNotFoundError.
func (rs *resourceServer) readHandler(claims *auth.Claims) gomcp.ResourceHandler {
	return func(ctx context.Context, req *gomcp.ReadResourceRequest) (*gomcp.ReadResourceResult, error) {
		uri := req.Params.URI
		org, name, version, err := parseFrameURI(uri)
		if err != nil {
			return nil, gomcp.ResourceNotFoundError(uri)
		}
		ctx = auth.WithClaims(ctx, claims)
		doc, err := rs.src.ResolveDoc(ctx, org, name, version)
		if err != nil {
			return nil, gomcp.ResourceNotFoundError(uri)
		}
		md := composeMarkdown(doc, time.Now())
		return &gomcp.ReadResourceResult{
			Contents: []*gomcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: md}},
		}, nil
	}
}
