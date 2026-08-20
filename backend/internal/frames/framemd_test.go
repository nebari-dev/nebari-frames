package frames_test

import (
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
			normalizeBody(doc)
			normalizeBody(back)
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
		Body:     "Lead with customer impact.",
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
		"Lead with customer impact.\n",
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

// The body is emitted verbatim: no synthesized title heading, no section
// structure imposed on the author's markdown.
func TestMarshalMarkdown_BodyPassthrough(t *testing.T) {
	doc := &frames.Doc{
		Name: "c", Description: "d", Version: "1.0.0", Visibility: "internal",
		Body: "# My Own Title\n\nSome prose.\n\n## Any Heading At All\n\n- a bullet",
	}
	md, err := frames.MarshalMarkdown(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wantTail := "---\n\n" + doc.Body + "\n"
	if !strings.HasSuffix(string(md), wantTail) {
		t.Errorf("body not passed through verbatim:\n%s", md)
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

// Any body shape at all must import: the spec defines no sections, so headings,
// loose prose, bullets, and content before a heading are all just body.
func TestUnmarshalMarkdown_FreeFormBody(t *testing.T) {
	src := `---
type: frame
name: code-review-norms
description: How this team reviews pull requests.
visibility: shared
version: 0.1.0
scope: department
maintainer: engineering enablement
---

# Code Review Norms

Block on correctness, security, and data loss.

## Whatever Heading

- **abbreviation**: A shortened form defined before repeated use.

Approve when the change is safe to merge, not when it is perfect.
`
	doc, err := frames.UnmarshalMarkdown([]byte(src))
	if err != nil {
		t.Fatalf("free-form body must convert, got: %v", err)
	}
	if doc.Scope != "department" || doc.Maintainer != "engineering enablement" {
		t.Errorf("metadata lost: %+v", doc)
	}
	for _, want := range []string{
		"# Code Review Norms",
		"Block on correctness, security, and data loss.",
		"## Whatever Heading",
		"- **abbreviation**: A shortened form defined before repeated use.",
		"Approve when the change is safe to merge, not when it is perfect.",
	} {
		if !strings.Contains(doc.Body, want) {
			t.Errorf("body missing %q:\n%s", want, doc.Body)
		}
	}
	if err := frames.Validate(doc); err != nil {
		t.Errorf("doc should validate: %v", err)
	}
}

// The template flag has no Frame Spec equivalent, so it travels in the
// x- namespace and must survive a round trip.
func TestMarkdown_TemplateFlagRoundTrip(t *testing.T) {
	doc := &frames.Doc{
		Name: "starter", Description: "d", Version: "1.0.0",
		Visibility: "internal", Template: true, Body: "Guidance.",
	}
	md, err := frames.MarshalMarkdown(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(md), "x-nebari-template: true\n") {
		t.Errorf("template flag not exported:\n%s", md)
	}
	back, err := frames.UnmarshalMarkdown(md)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.Template {
		t.Error("template flag lost on import")
	}
}

func TestUnmarshalMarkdown_EmptyBody(t *testing.T) {
	src := "---\ntype: frame [0.2]\nname: c\ndescription: d\nvisibility: internal\n---\n"
	doc, err := frames.UnmarshalMarkdown([]byte(src))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Body != "" {
		t.Errorf("expected empty body, got %q", doc.Body)
	}
}

func TestUnmarshalMarkdown_StructuralErrors(t *testing.T) {
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

// normalizeBody strips trailing newlines from the body. A YAML block scalar
// ("body: |") always ends with one, whereas the markdown form trims it. That
// whitespace carries no meaning in either form, so it is the one difference
// the round trip does not preserve, and both sides are normalized before
// comparison.
func normalizeBody(d *frames.Doc) {
	d.Body = strings.TrimRight(d.Body, "\n")
}
