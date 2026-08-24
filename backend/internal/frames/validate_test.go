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
// Both keys in one document is a legacy version somebody has since edited, and
// the precedence is silent - the discarded side produces no error and no
// warning - so it has to be pinned rather than left to be rediscovered.
func TestParse_BodyWinsOverLegacySlots(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantBody string
	}{
		{
			name:     "slots only fold into the body",
			content:  "name: c\ndescription: d\nversion: 1.0.0\nslots:\n  rules:\n    - from slots\n",
			wantBody: "## Rules\n\n- from slots",
		},
		{
			name: "an explicit body wins and the legacy block is dropped",
			content: "name: c\ndescription: d\nversion: 1.0.0\nbody: from body\n" +
				"slots:\n  rules:\n    - from slots\n",
			wantBody: "from body",
		},
		{
			name: "an explicitly empty body still falls back to slots",
			content: "name: c\ndescription: d\nversion: 1.0.0\nbody: \"\"\n" +
				"slots:\n  rules:\n    - from slots\n",
			wantBody: "## Rules\n\n- from slots",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := frames.Parse([]byte(tc.content))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if doc.Body != tc.wantBody {
				t.Errorf("body = %q, want %q", doc.Body, tc.wantBody)
			}
		})
	}
}

// The decode error reaches API clients unwrapped, so it must name the schema
// rather than the unexported Go type yaml.v3 happens to be decoding into.
func TestParse_UnknownKeyErrorNamesTheSchema(t *testing.T) {
	_, err := frames.Parse([]byte("name: c\ndescription: d\nversion: 1.0.0\nbogus: x\n"))
	if err == nil {
		t.Fatal("expected an error for an unknown key")
	}
	if strings.Contains(err.Error(), "docYAML") {
		t.Errorf("error leaks an internal type name, which means nothing to a client: %v", err)
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error does not name the offending key: %v", err)
	}
	if !strings.Contains(err.Error(), "maintainer") {
		t.Errorf("error does not list the recognized keys: %v", err)
	}
}

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
