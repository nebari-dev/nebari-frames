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
//
// Section headings and ordering come from frames.SlotTable, the same table the
// .frame.md codec uses, so the two markdown renderings cannot drift apart. The
// framing differs on purpose: this output describes an already-resolved frame
// for an AI client, so it carries the "# Frame:" title and the provenance
// blockquotes that an authoring document must not contain.
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
	for _, d := range frames.SlotTable {
		switch d.Kind {
		case frames.SlotTerms:
			if len(s.Terminology) == 0 {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n", d.Heading)
			for _, t := range s.Terminology {
				frames.WriteBullet(&b, fmt.Sprintf("**%s**: %s", t.Term, t.Definition))
			}
			b.WriteString("\n")
		case frames.SlotList:
			items := s.List(d.Key)
			if len(items) == 0 {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n", d.Heading)
			for _, it := range items {
				frames.WriteBullet(&b, it)
			}
			b.WriteString("\n")
		case frames.SlotProse:
			body := s.Prose(d.Key)
			if strings.TrimSpace(body) == "" {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", d.Heading, strings.Trim(body, "\n"))
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
