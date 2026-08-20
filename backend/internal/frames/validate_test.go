package frames_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

func TestParseAndValidate_Valid(t *testing.T) {
	content := []byte(`
name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
body: |
  Lead with customer impact.

  Never claim performance numbers without a benchmark citation.
`)
	doc, err := frames.Parse(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := frames.Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(doc.Body, "Lead with customer impact.") {
		t.Errorf("body not parsed: %q", doc.Body)
	}
}

// Documents published under the retired ten-slot schema must stay readable:
// Parse folds a legacy `slots:` block into the free-form body, rendered as the
// markdown sections the old .frame.md codec emitted.
func TestParse_LegacySlotsFoldIntoBody(t *testing.T) {
	content := []byte(`
name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
slots:
  terminology:
    - term: customer
      definition: An enterprise organization.
  rules:
    - Never claim performance numbers without a benchmark citation.
  goals: |
    Lead with customer impact.
`)
	doc, err := frames.Parse(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := frames.Validate(doc); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, want := range []string{
		"## Terminology",
		"- **customer**: An enterprise organization.",
		"## Rules",
		"- Never claim performance numbers without a benchmark citation.",
		"## Goals",
		"Lead with customer impact.",
	} {
		if !strings.Contains(doc.Body, want) {
			t.Errorf("legacy body missing %q:\n%s", want, doc.Body)
		}
	}
	// The legacy shape is read-only: re-marshaling emits the new body form.
	out, err := frames.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "slots:") {
		t.Errorf("marshal must not emit the legacy slots key:\n%s", out)
	}
}

func TestValidate_CollectsFieldErrors(t *testing.T) {
	tests := []struct {
		name           string
		doc            *frames.Doc
		wantErrorPaths []string
	}{
		{
			name: "bad name",
			doc: &frames.Doc{
				Name:        "Bad Name",
				Description: "valid description",
				Version:     "1.0.0",
			},
			wantErrorPaths: []string{"name"},
		},
		{
			name: "empty description",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "",
				Version:     "1.0.0",
			},
			wantErrorPaths: []string{"description"},
		},
		{
			name: "over-280 description",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: strings.Repeat("a", 281),
				Version:     "1.0.0",
			},
			wantErrorPaths: []string{"description"},
		},
		{
			name: "empty version",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "valid description",
				Version:     "",
			},
			wantErrorPaths: []string{"version"},
		},
		{
			name: "bad visibility",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "valid description",
				Version:     "1.0.0",
				Visibility:  "everyone",
			},
			wantErrorPaths: []string{"visibility"},
		},
		{
			name: "extends missing slash in ref",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "valid description",
				Version:     "1.0.0",
				Extends:     []frames.ExtendRef{{Ref: "noslash", Version: "1.0.0"}},
			},
			wantErrorPaths: []string{"extends[0].ref"},
		},
		{
			name: "extends unpinned version",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "valid description",
				Version:     "1.0.0",
				Extends:     []frames.ExtendRef{{Ref: "org/frame", Version: ""}},
			},
			wantErrorPaths: []string{"extends[0].version"},
		},
		{
			name: "multiple errors collected at once",
			doc: &frames.Doc{
				Name:        "Bad Name",
				Description: "",
				Version:     "",
				Extends:     []frames.ExtendRef{{Ref: "noslash"}},
			},
			wantErrorPaths: []string{
				"name",
				"description",
				"version",
				"extends[0].ref",
				"extends[0].version",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := frames.Validate(tc.doc)
			var ve *frames.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *ValidationError, got %v", err)
			}
			paths := map[string]bool{}
			for _, fe := range ve.Errors {
				paths[fe.Path] = true
			}
			for _, want := range tc.wantErrorPaths {
				if !paths[want] {
					t.Errorf("missing expected field error %q; got errors: %v", want, ve.Errors)
				}
			}
		})
	}
}

func TestParse_RejectsUnknownKeys(t *testing.T) {
	_, err := frames.Parse([]byte("name: x\nbogus: y\nbody: text\n"))
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}
