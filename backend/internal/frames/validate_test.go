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
			name: "empty terminology definition",
			doc: &frames.Doc{
				Name:        "good-name",
				Description: "valid description",
				Version:     "1.0.0",
			},
			// Slots.Terminology is set after the slice literal (see below tests[4]).
			wantErrorPaths: []string{"slots.terminology[0].definition"},
		},
		{
			name: "duplicate term",
			doc: func() *frames.Doc {
				d := &frames.Doc{
					Name:        "good-name",
					Description: "valid description",
					Version:     "1.0.0",
				}
				d.Slots.Terminology = []frames.Term{
					{Term: "foo", Definition: "first"},
					{Term: "foo", Definition: "second"},
				}
				return d
			}(),
			wantErrorPaths: []string{"slots.terminology[1].term"},
		},
		{
			name: "empty rule",
			doc: func() *frames.Doc {
				d := &frames.Doc{
					Name:        "good-name",
					Description: "valid description",
					Version:     "1.0.0",
				}
				d.Slots.Rules = []string{"valid rule", ""}
				return d
			}(),
			wantErrorPaths: []string{"slots.rules[1]"},
		},
		{
			name: "extends missing slash in ref",
			doc: func() *frames.Doc {
				d := &frames.Doc{
					Name:        "good-name",
					Description: "valid description",
					Version:     "1.0.0",
				}
				d.Extends = []frames.ExtendRef{{Ref: "noslash", Version: "1.0.0"}}
				return d
			}(),
			wantErrorPaths: []string{"extends[0].ref"},
		},
		{
			name: "extends unpinned version",
			doc: func() *frames.Doc {
				d := &frames.Doc{
					Name:        "good-name",
					Description: "valid description",
					Version:     "1.0.0",
				}
				d.Extends = []frames.ExtendRef{{Ref: "org/frame", Version: ""}}
				return d
			}(),
			wantErrorPaths: []string{"extends[0].version"},
		},
		{
			name: "multiple errors collected at once",
			doc: func() *frames.Doc {
				d := &frames.Doc{
					Name:        "Bad Name",
					Description: "",
					Version:     "",
				}
				d.Slots.Terminology = []frames.Term{
					{Term: "x", Definition: ""},
					{Term: "x", Definition: "dupe"},
				}
				return d
			}(),
			wantErrorPaths: []string{
				"name",
				"description",
				"version",
				"slots.terminology[0].definition",
				"slots.terminology[1].term",
			},
		},
	}

	// Set the empty-definition case's terminology (the entry above used a comment placeholder).
	tests[4].doc.Slots.Terminology = []frames.Term{{Term: "x", Definition: ""}}

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
	_, err := frames.Parse([]byte("name: x\nbogus: y\nslots: {}\n"))
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}
