package mcp

import (
	"slices"
	"testing"
)

func TestBuildMetadata(t *testing.T) {
	c := Config{
		PublicURL: "https://frames.example.com",
		IssuerURL: "https://kc.example.com/realms/main",
		Audience:  "https://frames.example.com/mcp",
	}
	md := buildMetadata(c)
	if md.Resource != "https://frames.example.com/mcp" {
		t.Errorf("Resource=%q", md.Resource)
	}
	if !slices.Equal(md.AuthorizationServers, []string{"https://kc.example.com/realms/main"}) {
		t.Errorf("AuthorizationServers=%v", md.AuthorizationServers)
	}
	if len(md.ScopesSupported) == 0 {
		t.Error("ScopesSupported must be populated")
	}
	if !slices.Contains(md.BearerMethodsSupported, "header") {
		t.Errorf("BearerMethodsSupported=%v", md.BearerMethodsSupported)
	}
}

func TestCanonicalAndMetadataURL(t *testing.T) {
	c := Config{PublicURL: "https://frames.example.com/"} // trailing slash must be trimmed
	if got := c.canonicalResourceURL(); got != "https://frames.example.com/mcp" {
		t.Errorf("canonicalResourceURL=%q", got)
	}
	if got := c.metadataURL(); got != "https://frames.example.com/.well-known/oauth-protected-resource" {
		t.Errorf("metadataURL=%q", got)
	}
}
