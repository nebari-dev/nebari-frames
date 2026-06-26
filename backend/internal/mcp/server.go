package mcp

import (
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
)

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
	mux.Handle("/.well-known/oauth-protected-resource", metadataHandler(c.cfg))

	rs := &resourceServer{src: c.src, cfg: c.cfg}
	mcpHandler := gomcp.NewStreamableHTTPHandler(rs.getServer, nil)

	var handler http.Handler = mcpHandler
	if !c.cfg.DevMode {
		verifier := newTokenVerifier(c.verifier)
		middleware := mcpauth.RequireBearerToken(verifier, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: c.cfg.metadataURL(),
		})
		handler = middleware(mcpHandler)
	}

	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)
}
