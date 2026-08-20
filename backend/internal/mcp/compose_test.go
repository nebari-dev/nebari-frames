package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

func TestComposeMarkdown(t *testing.T) {
	ts := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	full := &frames.Doc{
		Name:        "brand-voice",
		Description: "How we speak.",
		Version:     "1.2.0",
		Extends:     []frames.ExtendRef{{Ref: "openteams/base", Version: "1.0.0"}},
		Body:        "Be concise.\n\n## House Style\n\nSound human.",
	}

	tests := []struct {
		name        string
		doc         *frames.Doc
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "renders body verbatim with inheritance header",
			doc:  full,
			mustContain: []string{
				"# Frame: brand-voice",
				"How we speak.",
				"> Inherits from: openteams/base@1.0.0",
				"> Resolved at: 2026-06-26T12:00:00Z",
				"Be concise.",
				"## House Style",
				"Sound human.",
			},
		},
		{
			name:        "no extends omits inherits header",
			doc:         &frames.Doc{Name: "base", Description: "Root.", Version: "1.0.0"},
			mustContain: []string{"# Frame: base", "Root.", "> Resolved at: 2026-06-26T12:00:00Z"},
			mustNotHave: []string{"> Inherits from:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := composeMarkdown(tt.doc, ts)
			for _, s := range tt.mustContain {
				if !strings.Contains(out, s) {
					t.Errorf("output missing %q\n---\n%s", s, out)
				}
			}
			for _, s := range tt.mustNotHave {
				if strings.Contains(out, s) {
					t.Errorf("output unexpectedly contains %q\n---\n%s", s, out)
				}
			}
		})
	}
}
