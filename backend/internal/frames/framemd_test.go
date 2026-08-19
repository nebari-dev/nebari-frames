package frames_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// TestExampleFrames_MarkdownRoundTrip is the golden corpus: every example is
// rendered to .frame.md, compared against the checked-in file, and parsed back.
// The parsed Doc must equal the original, which is what makes the markdown form
// safe to use as an editing surface for the canonical YAML.
func TestExampleFrames_MarkdownRoundTrip(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "examples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			doc, err := frames.Parse(content)
			if err != nil {
				t.Fatalf("parse yaml: %v", err)
			}

			md, err := frames.MarshalMarkdown(doc)
			if err != nil {
				t.Fatalf("marshal markdown: %v", err)
			}

			goldenPath := filepath.Join(dir, strings.TrimSuffix(e.Name(), ".yaml")+".frame.md")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(goldenPath, md, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (re-run with UPDATE_GOLDEN=1 to create): %v", err)
			}
			if string(golden) != string(md) {
				t.Errorf("markdown differs from golden.\n--- golden ---\n%s\n--- got ---\n%s", golden, md)
			}

			back, err := frames.UnmarshalMarkdown(md)
			if err != nil {
				t.Fatalf("unmarshal markdown: %v", err)
			}
			normalizeProse(doc)
			normalizeProse(back)
			if !reflect.DeepEqual(doc, back) {
				t.Errorf("round trip lost data.\noriginal:      %+v\nround-tripped: %+v", doc, back)
			}
			if err := frames.Validate(back); err != nil {
				t.Errorf("round-tripped doc no longer validates: %v", err)
			}
		})
	}
	if count == 0 {
		t.Fatal("no example frames found")
	}
}

func TestMarshalMarkdown_Frontmatter(t *testing.T) {
	doc := &frames.Doc{
		Name: "brand-voice", Description: "Voice guardrails.", Version: "1.0.0",
		Visibility: "internal", Scope: "company", Maintainer: "marketing",
		Extends: []frames.ExtendRef{
			{Ref: "openteams/company-core", Version: "1.2.0"},
			{Ref: "industry/healthcare", Version: "2024.4"},
		},
		Excludes: []string{"openteams/legacy"},
	}
	md, err := frames.MarshalMarkdown(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(md)
	for _, want := range []string{
		"type: frame [0.2]\n",
		"visibility: internal\n",
		"scope: company\n",
		"maintainer: marketing\n",
		"    - openteams/company-core@1.2.0\n",
		"    - industry/healthcare@2024.4\n",
		"x-nebari-excludes:\n",
		"# brand-voice\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

// A doc stored before `visibility` existed must still export as a conformant
// document, since the spec requires the field.
func TestMarshalMarkdown_DefaultsVisibility(t *testing.T) {
	md, err := frames.MarshalMarkdown(&frames.Doc{Name: "legacy", Description: "d", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(md), "visibility: internal\n") {
		t.Errorf("expected defaulted visibility, got:\n%s", md)
	}
}

func TestUnmarshalMarkdown_InheritsForms(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []frames.ExtendRef
	}{
		{"scalar", "inherits: org/a@1.0.0", []frames.ExtendRef{{Ref: "org/a", Version: "1.0.0"}}},
		{"list", "inherits:\n  - org/a@1.0.0\n  - org/b@2.0.0", []frames.ExtendRef{
			{Ref: "org/a", Version: "1.0.0"}, {Ref: "org/b", Version: "2.0.0"},
		}},
		// A bare spec-style ref is not a structural error: it parses, then
		// Validate reports it as unqualified/unpinned on the extends field.
		{"unpinned", "inherits: editorial-style-guide", []frames.ExtendRef{{Ref: "editorial-style-guide"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "---\ntype: frame [0.2]\nname: c\ndescription: d\nvisibility: internal\n" + tc.in + "\n---\n"
			doc, err := frames.UnmarshalMarkdown([]byte(src))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(tc.want, doc.Extends) {
				t.Errorf("want %+v, got %+v", tc.want, doc.Extends)
			}
		})
	}
}

// The spec's own examples/complete/frame.md must import: its sections all map
// to slots, and its bare `inherits` must surface as a fixable field error from
// Validate rather than blocking the conversion.
func TestUnmarshalMarkdown_SpecCompleteExample(t *testing.T) {
	src := `---
type: frame
name: engineering-documentation-style
description: Writing guidance for engineering documentation.
visibility: internal
version: 0.1.0
scope: department
maintainer: engineering enablement
inherits: editorial-style-guide
---

# Engineering Documentation Style

## Goals

- Make technical guidance easy to scan and act on.

## Terminology

- **abbreviation**: A shortened form defined before repeated use.

## Style

- Lead with the task outcome before implementation detail.
`
	doc, err := frames.UnmarshalMarkdown([]byte(src))
	if err != nil {
		t.Fatalf("spec example must convert, got: %v", err)
	}
	if doc.Scope != "department" || doc.Maintainer != "engineering enablement" {
		t.Errorf("metadata lost: %+v", doc)
	}
	if len(doc.Slots.Terminology) != 1 || doc.Slots.Terminology[0].Term != "abbreviation" {
		t.Errorf("terminology not parsed: %+v", doc.Slots.Terminology)
	}
	if !strings.Contains(doc.Slots.Goals, "easy to scan") {
		t.Errorf("goals not parsed: %q", doc.Slots.Goals)
	}

	verr := frames.Validate(doc)
	if verr == nil {
		t.Fatal("expected validation errors for the unpinned bare inherits ref")
	}
	var ve *frames.ValidationError
	if !errors.As(verr, &ve) {
		t.Fatalf("expected *ValidationError, got %T", verr)
	}
	wantPaths := map[string]bool{"extends[0].ref": false, "extends[0].version": false}
	for _, fe := range ve.Errors {
		if _, ok := wantPaths[fe.Path]; ok {
			wantPaths[fe.Path] = true
		}
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Errorf("expected a fixable field error on %s, got %v", path, ve.Errors)
		}
	}
}

func TestUnmarshalMarkdown_StructuralErrors(t *testing.T) {
	const head = "---\ntype: frame [0.2]\nname: c\ndescription: d\nvisibility: internal\n---\n\n"
	tests := []struct {
		name    string
		src     string
		wantSub string
	}{
		{"no frontmatter", "# Just markdown\n", "must begin with a YAML frontmatter block"},
		{"unterminated frontmatter", "---\ntype: frame [0.2]\nname: c\n", "has no closing ---"},
		{"unknown frontmatter key", "---\ntype: frame [0.2]\nname: c\nowner: bob\n---\n", "unknown frontmatter key \"owner\""},
		{"missing type", "---\nname: c\ndescription: d\nvisibility: internal\n---\n", "missing required frontmatter field \"type\""},
		{"bad type", "---\ntype: skill\nname: c\ndescription: d\nvisibility: internal\n---\n", "type must be"},
		{"unknown section", head + "## Ways of Working\n\n- a\n", "did you mean \"## Norms\"?"},
		{"unknown section no hint", head + "## Escalation Path\n\n- a\n", "recognized sections are: Terminology"},
		{"wrong case section", head + "## goals\n\ntext\n", "did you mean \"## Goals\"?"},
		{"duplicate section", head + "## Goals\n\na\n\n## Goals\n\nb\n", "duplicate section"},
		{"content before section", head + "Loose prose.\n\n## Goals\n\na\n", "content before the first section heading"},
		{"malformed terminology", head + "## Terminology\n\n- customer is an org\n", "malformed terminology entry"},
		{"stray content in list", head + "## Rules\n\nnot a bullet\n", "unexpected content in \"## Rules\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := frames.UnmarshalMarkdown([]byte(tc.src))
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("error %q does not name a line number", err.Error())
			}
		})
	}
}

// Multi-line list items are the one shape naive bullet rendering breaks: the
// continuation lines must be indented on the way out and dedented on the way in.
func TestMarkdown_MultiLineListItem(t *testing.T) {
	doc := &frames.Doc{
		Name: "c", Description: "d", Version: "1.0.0", Visibility: "internal",
		Slots: frames.Slots{Rules: []string{
			"First line of the rule.\nSecond line.\n\nA new paragraph.",
			"A single-line rule.",
		}},
	}
	md, err := frames.MarshalMarkdown(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(md), "- First line of the rule.\n  Second line.\n\n  A new paragraph.\n") {
		t.Errorf("continuation lines not indented:\n%s", md)
	}
	back, err := frames.UnmarshalMarkdown(md)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(doc.Slots.Rules, back.Slots.Rules) {
		t.Errorf("multi-line rule lost.\nwant %q\ngot  %q", doc.Slots.Rules, back.Slots.Rules)
	}
}

// Slot bodies may not contain their own "## " headings, since those delimit
// sections. Authors must use "###" or deeper; this locks in the diagnostic.
func TestUnmarshalMarkdown_H2InsideProse(t *testing.T) {
	src := "---\ntype: frame [0.2]\nname: c\ndescription: d\nvisibility: internal\n---\n\n## Goals\n\nIntro.\n\n## Sub Goal\n\nMore.\n"
	_, err := frames.UnmarshalMarkdown([]byte(src))
	if err == nil {
		t.Fatal("expected an error for an unrecognized ## inside prose")
	}
	if !strings.Contains(err.Error(), "unknown section \"## Sub Goal\"") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnmarshalMarkdown_H3InsideProseIsKept(t *testing.T) {
	src := "---\ntype: frame [0.2]\nname: c\ndescription: d\nvisibility: internal\n---\n\n## Goals\n\n### Near term\n\nShip it.\n"
	doc, err := frames.UnmarshalMarkdown([]byte(src))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Slots.Goals != "### Near term\n\nShip it." {
		t.Errorf("prose body not preserved: %q", doc.Slots.Goals)
	}
}

// normalizeProse strips trailing newlines from prose slots. A YAML block scalar
// ("goals: |") always ends with one, whereas a markdown section is delimited by
// the next heading rather than by trailing whitespace. That whitespace carries
// no meaning in either form, so it is the one difference the round trip does not
// preserve, and both sides are normalized before comparison.
func normalizeProse(d *frames.Doc) {
	for _, sd := range frames.SlotTable {
		if sd.Kind == frames.SlotProse {
			d.Slots.SetProse(sd.Key, strings.TrimRight(d.Slots.Prose(sd.Key), "\n"))
		}
	}
}
