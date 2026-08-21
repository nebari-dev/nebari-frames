package mcp

import (
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
)

// MaxRequestBytes caps a single MCP request body. It is deliberately larger
// than frames.MaxContentBytes so a create_frame at the content limit still fits
// with its JSON-RPC framing.
const MaxRequestBytes = 8 << 20 // 8 MiB

// maxBodyBytes limits how much of a request body the next handler can read.
func maxBodyBytes(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// Component bundles the MCP endpoint routes and can be mounted onto any
// http.ServeMux. Create it with NewComponent.
type Component struct {
	cfg      Config
	src      FrameSource
	verifier auth.TokenValidator
}

// NewComponent constructs the MCP Component. src is the frame data source
// (typically *frames.Service). verifier is the token validator scoped to the
// MCP audience; pass nil in dev mode and set cfg.DevMode true.
func NewComponent(cfg Config, src FrameSource, verifier auth.TokenValidator) *Component {
	return &Component{cfg: cfg, src: src, verifier: verifier}
}

// Mount registers the MCP routes on mux:
//
//   - GET /.well-known/oauth-protected-resource - public RFC 9728 metadata
//   - /mcp - Streamable HTTP MCP endpoint (bearer-protected in non-dev mode)
func (c *Component) Mount(mux *http.ServeMux) {
	// Fail fast at startup on a wiring bug: a bearer-protected endpoint with no
	// validator would otherwise panic at request time on the first token.
	if !c.cfg.DevMode && c.verifier == nil {
		panic("mcp: non-dev mode requires a non-nil TokenValidator")
	}
	mux.Handle("/.well-known/oauth-protected-resource", metadataHandler(c.cfg))

	rs := &resourceServer{src: c.src, cfg: c.cfg}
	mcpHandler := gomcp.NewStreamableHTTPHandler(rs.getServer, nil)

	// Bound what a request can make the server buffer. The SDK reads the body in
	// full before a tool handler - and therefore before RBAC - is ever reached,
	// so without this an authenticated caller who may not write at all can
	// exhaust the memory of a single-replica deployment.
	handler := maxBodyBytes(mcpHandler, MaxRequestBytes)
	if !c.cfg.DevMode {
		verifier := newTokenVerifier(c.verifier)
		middleware := mcpauth.RequireBearerToken(verifier, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: c.cfg.metadataURL(),
		})
		// Wraps handler, not mcpHandler: wrapping the latter would discard the
		// body cap in exactly the deployments that have authentication on.
		handler = middleware(handler)
	}

	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)
}
