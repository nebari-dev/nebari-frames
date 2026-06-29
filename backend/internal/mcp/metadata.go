package mcp

import (
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// Config holds the deployment-specific values the MCP endpoint needs.
type Config struct {
	PublicURL string // scheme+host of the public deployment, e.g. https://frames.example.com
	IssuerURL string // OIDC issuer (Keycloak realm) - the authorization server
	Audience  string // required token audience; defaults to canonicalResourceURL()
	DevMode   bool   // when true, /mcp is served without bearer auth using DevClaims
}

func (c Config) trimmedBase() string { return strings.TrimRight(c.PublicURL, "/") }

// canonicalResourceURL is the RFC 8707 resource identifier for this MCP server.
func (c Config) canonicalResourceURL() string { return c.trimmedBase() + "/mcp" }

// metadataURL is the absolute URL of the protected-resource-metadata document.
func (c Config) metadataURL() string {
	return c.trimmedBase() + "/.well-known/oauth-protected-resource"
}

// audienceOrDefault returns the configured audience, defaulting to the canonical
// resource URL when unset.
func (c Config) audienceOrDefault() string {
	if c.Audience != "" {
		return c.Audience
	}
	return c.canonicalResourceURL()
}

// ResolvedAudience returns the effective audience value that token validators
// must enforce. It is the configured Audience when set, or the canonical
// resource URL otherwise.
func (c Config) ResolvedAudience() string { return c.audienceOrDefault() }

// buildMetadata builds the RFC 9728 protected-resource metadata document.
func buildMetadata(c Config) *oauthex.ProtectedResourceMetadata {
	return &oauthex.ProtectedResourceMetadata{
		Resource:               c.canonicalResourceURL(),
		AuthorizationServers:   []string{c.IssuerURL},
		ScopesSupported:        []string{"openid", "email", "profile"},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Nebari Frames",
	}
}

// metadataHandler serves /.well-known/oauth-protected-resource (public, CORS).
func metadataHandler(c Config) http.Handler {
	return mcpauth.ProtectedResourceMetadataHandler(buildMetadata(c))
}
