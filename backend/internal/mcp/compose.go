package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// composeMarkdown renders a resolved Frame Doc into the deterministic markdown
// format defined in the MCP design doc (section 3.4). resolvedAt is passed in
// (not read from a clock) so the function is pure and testable.
//
// The body passes through verbatim. The framing differs from the .frame.md
// authoring form on purpose: this output describes an already-resolved frame
// for an AI client, so it carries the "# Frame:" title and the provenance
// blockquotes that an authoring document must not contain.
func composeMarkdown(doc *frames.Doc, resolvedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Frame: %s\n\n", doc.Name)
	if doc.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", doc.Description)
	}
	// The version is what update_frame's base_version must be set to, so it has
	// to be visible to a client that intends to edit this frame.
	if doc.Version != "" {
		fmt.Fprintf(&b, "> Version: %s\n", doc.Version)
	}
	if len(doc.Extends) > 0 {
		parts := make([]string, len(doc.Extends))
		for i, e := range doc.Extends {
			parts[i] = e.Ref + "@" + e.Version
		}
		fmt.Fprintf(&b, "> Inherits from: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(&b, "> Resolved at: %s\n\n", resolvedAt.UTC().Format(time.RFC3339))

	if body := strings.Trim(doc.Body, "\n"); body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
