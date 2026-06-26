package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// composeMarkdown renders a resolved Frame Doc into the deterministic markdown
// format defined in the MCP design doc (section 3.4). Empty slots are omitted
// entirely. resolvedAt is passed in (not read from a clock) so the function is
// pure and testable.
func composeMarkdown(doc *frames.Doc, resolvedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Frame: %s\n\n", doc.Name)
	if doc.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", doc.Description)
	}
	if len(doc.Extends) > 0 {
		parts := make([]string, len(doc.Extends))
		for i, e := range doc.Extends {
			parts[i] = e.Ref + "@" + e.Version
		}
		fmt.Fprintf(&b, "> Inherits from: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(&b, "> Resolved at: %s\n\n", resolvedAt.UTC().Format(time.RFC3339))

	s := doc.Slots
	if len(s.Terminology) > 0 {
		b.WriteString("## Terminology\n\n")
		for _, t := range s.Terminology {
			fmt.Fprintf(&b, "- **%s**: %s\n", t.Term, t.Definition)
		}
		b.WriteString("\n")
	}
	writeList(&b, "Rules", s.Rules)
	writeList(&b, "Skills", s.Skills)
	writeList(&b, "Prompts", s.Prompts)
	writeProse(&b, "Tool Specifications", s.ToolSpecs)
	writeProse(&b, "Goals", s.Goals)
	writeProse(&b, "Style", s.Style)
	writeProse(&b, "Norms", s.Norms)
	writeProse(&b, "Architecture", s.Architecture)
	writeProse(&b, "Business Process", s.BusinessProcess)

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeList(b *strings.Builder, header string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", header)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteString("\n")
}

func writeProse(b *strings.Builder, header, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	fmt.Fprintf(b, "## %s\n\n%s\n\n", header, body)
}
