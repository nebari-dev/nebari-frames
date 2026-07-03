package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
		slog.Error("mcp: getServer: ListReadable failed, serving no resources", "error", err)
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

	// Tools, for MCP clients that invoke tools in-conversation (e.g. Claude.ai),
	// where passive resources are not reachable. Same RBAC + per-request claims
	// as the resources; thin wrappers over ListReadable/ResolveDoc.
	gomcp.AddTool(srv, &gomcp.Tool{
		Name:        "list_frames",
		Description: "List the Frames the current user can read in their organization (name, version, description). Call this to discover available Frames.",
	}, rs.listFramesTool(claims))
	gomcp.AddTool(srv, &gomcp.Tool{
		Name:        "get_frame",
		Description: "Get the full composed Markdown of a Frame by name (optionally a specific version). Use this to load an organization Frame as context before writing.",
	}, rs.getFrameTool(claims))

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

// listFramesInput takes no arguments (lists the caller's readable frames).
type listFramesInput struct{}

// listFramesTool lists the caller's RBAC-readable Frames. It closes over the
// per-request caller claims (like readHandler) and reuses ListReadable.
func (rs *resourceServer) listFramesTool(claims *auth.Claims) gomcp.ToolHandlerFor[listFramesInput, any] {
	return func(ctx context.Context, _ *gomcp.CallToolRequest, _ listFramesInput) (*gomcp.CallToolResult, any, error) {
		ctx = auth.WithClaims(ctx, claims)
		readable, err := rs.src.ListReadable(ctx)
		if err != nil {
			return errorResult("could not list frames"), nil, nil
		}
		if len(readable) == 0 {
			return textResult("No readable Frames in your organization."), nil, nil
		}
		var b strings.Builder
		for _, f := range readable {
			fmt.Fprintf(&b, "- %s@%s: %s\n", f.Name, f.Version, f.Description)
		}
		return textResult(b.String()), nil, nil
	}
}

// getFrameInput selects a Frame by name (and optional version).
type getFrameInput struct {
	Name    string `json:"name" jsonschema:"the Frame name, e.g. nebari-platform"`
	Version string `json:"version,omitempty" jsonschema:"optional version; defaults to the latest"`
}

// getFrameTool returns a Frame's composed Markdown. It finds the named frame
// among the caller's readable frames (RBAC) to resolve its org, then composes
// it. Unknown or unreadable names return an error result (no existence leak).
func (rs *resourceServer) getFrameTool(claims *auth.Claims) gomcp.ToolHandlerFor[getFrameInput, any] {
	return func(ctx context.Context, _ *gomcp.CallToolRequest, in getFrameInput) (*gomcp.CallToolResult, any, error) {
		ctx = auth.WithClaims(ctx, claims)
		readable, err := rs.src.ListReadable(ctx)
		if err != nil {
			return errorResult("could not load frame"), nil, nil
		}
		var match *frames.ReadableFrame
		for i := range readable {
			if readable[i].Name == in.Name {
				match = &readable[i]
				break
			}
		}
		if match == nil {
			return errorResult("frame not found: " + in.Name), nil, nil
		}
		version := in.Version
		if version == "" {
			version = match.Version
		}
		doc, err := rs.src.ResolveDoc(ctx, match.OrgSlug, in.Name, version)
		if err != nil {
			return errorResult("frame not found: " + in.Name), nil, nil
		}
		return textResult(composeMarkdown(doc, time.Now())), nil, nil
	}
}

// textResult / errorResult build single-text-content tool results.
func textResult(s string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: s}}}
}

func errorResult(s string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{IsError: true, Content: []gomcp.Content{&gomcp.TextContent{Text: s}}}
}
