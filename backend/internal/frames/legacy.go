package frames

import (
	"fmt"
	"strings"
)

// This file exists only to read documents published before the schema was
// reduced to a single free-form body (Frame Spec v0.2 defines no body
// structure). Stored versions are immutable, so the old ten-slot YAML shape
// must stay readable forever; Parse folds it into Doc.Body via renderMarkdown.
// Nothing ever writes this shape again.

// legacyTerm is a vocabulary entry from the retired terminology slot.
type legacyTerm struct {
	Term       string `yaml:"term"`
	Definition string `yaml:"definition"`
}

// legacySlots is the retired ten-slot content schema.
type legacySlots struct {
	Terminology     []legacyTerm `yaml:"terminology,omitempty"`
	Rules           []string     `yaml:"rules,omitempty"`
	Skills          []string     `yaml:"skills,omitempty"`
	Prompts         []string     `yaml:"prompts,omitempty"`
	ToolSpecs       string       `yaml:"tool_specs,omitempty"`
	Goals           string       `yaml:"goals,omitempty"`
	Style           string       `yaml:"style,omitempty"`
	Norms           string       `yaml:"norms,omitempty"`
	Architecture    string       `yaml:"architecture,omitempty"`
	BusinessProcess string       `yaml:"business_process,omitempty"`
}

// renderMarkdown renders the legacy slots as the markdown sections the old
// .frame.md codec emitted, in the old schema order, so a legacy document reads
// the same as its historical export.
func (s *legacySlots) renderMarkdown() string {
	var b strings.Builder

	if len(s.Terminology) > 0 {
		b.WriteString("## Terminology\n\n")
		for _, t := range s.Terminology {
			writeLegacyBullet(&b, fmt.Sprintf("**%s**: %s", t.Term, t.Definition))
		}
		b.WriteString("\n")
	}
	writeLegacyList := func(heading string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", heading)
		for _, it := range items {
			writeLegacyBullet(&b, it)
		}
		b.WriteString("\n")
	}
	writeLegacyList("Rules", s.Rules)
	writeLegacyList("Skills", s.Skills)
	writeLegacyList("Prompts", s.Prompts)

	writeProse := func(heading, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", heading, strings.Trim(body, "\n"))
	}
	writeProse("Tool Specifications", s.ToolSpecs)
	writeProse("Goals", s.Goals)
	writeProse("Style", s.Style)
	writeProse("Norms", s.Norms)
	writeProse("Architecture", s.Architecture)
	writeProse("Business Process", s.BusinessProcess)

	return strings.TrimRight(b.String(), "\n")
}

// writeLegacyBullet renders one list item as a markdown bullet, indenting
// continuation lines two spaces so a multi-line item stays part of the item.
func writeLegacyBullet(b *strings.Builder, item string) {
	lines := strings.Split(strings.Trim(item, "\n"), "\n")
	fmt.Fprintf(b, "- %s\n", lines[0])
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(b, "  %s\n", l)
	}
}
