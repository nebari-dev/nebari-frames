// Package mcp implements the remote Model Context Protocol endpoint that serves
// Frames as MCP resources. It is a thin protocol adapter over frames.Service.
package mcp

import (
	"errors"
	"strings"
)

const frameURIScheme = "nebari-frame://"

// formatFrameURI builds the canonical resource URI for a frame version.
func formatFrameURI(orgSlug, name, version string) string {
	return frameURIScheme + orgSlug + "/" + name + "@" + version
}

// parseFrameURI parses nebari-frame://<org>/<name>[@<version>]. A missing
// version is allowed (empty return). Any structural deviation is an error so
// malformed or injection-style URIs are rejected rather than misrouted.
func parseFrameURI(uri string) (orgSlug, name, version string, err error) {
	rest, ok := strings.CutPrefix(uri, frameURIScheme)
	if !ok {
		return "", "", "", errors.New("mcp: uri must use nebari-frame:// scheme")
	}
	orgSlug, tail, ok := strings.Cut(rest, "/")
	if !ok || orgSlug == "" {
		return "", "", "", errors.New("mcp: uri missing org/name")
	}
	// The name segment (with optional @version) must not contain further path
	// separators; reject traversal and multi-segment paths.
	if strings.Contains(tail, "/") {
		return "", "", "", errors.New("mcp: uri name must be a single segment")
	}
	name, version, hasAt := strings.Cut(tail, "@")
	if name == "" || name == "." || name == ".." {
		return "", "", "", errors.New("mcp: uri missing or invalid name")
	}
	if hasAt && version == "" {
		return "", "", "", errors.New("mcp: uri has empty version after @")
	}
	return orgSlug, name, version, nil
}
