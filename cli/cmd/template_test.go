package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"gopkg.in/yaml.v3"
)

func TestTemplateListCommand(t *testing.T) {
	tests := []struct {
		name       string
		templates  []*framesv1.FrameTemplateSummary
		wantLines  []string
		wantAbsent []string
	}{
		{
			name: "shows built-ins and org templates with their source",
			templates: []*framesv1.FrameTemplateSummary{
				{Id: "builtin:blank", Title: "Blank", Description: "Empty.", Builtin: true},
				{Id: "01JORG", Title: "Our House Style", Description: "Ours.", Builtin: false},
			},
			wantLines: []string{"builtin:blank", "Blank", "built-in", "01JORG", "Our House Style", "organization"},
		},
		{
			name:       "says so when there are none",
			templates:  nil,
			wantLines:  []string{"No templates"},
			wantAbsent: []string{"ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := testutil.NewStubServer(t, &testutil.StubService{
				ListTemplatesFn: func(_ context.Context, _ *connect.Request[framesv1.ListFrameTemplatesRequest]) (*connect.Response[framesv1.ListFrameTemplatesResponse], error) {
					return connect.NewResponse(&framesv1.ListFrameTemplatesResponse{Templates: tt.templates}), nil
				},
			})
			out := runCmd(t, url, "template", "list")
			for _, want := range tt.wantLines {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, out)
				}
			}
		})
	}
}

// returns serves a fixed template from GetFrameTemplate.
func returns(t *framesv1.FrameTemplate) func(context.Context, *connect.Request[framesv1.GetFrameTemplateRequest]) (*connect.Response[framesv1.GetFrameTemplateResponse], error) {
	return func(context.Context, *connect.Request[framesv1.GetFrameTemplateRequest]) (*connect.Response[framesv1.GetFrameTemplateResponse], error) {
		return connect.NewResponse(&framesv1.GetFrameTemplateResponse{Template: t}), nil
	}
}

func TestTemplateInitWritesAScaffold(t *testing.T) {
	tmpl := &framesv1.FrameTemplate{
		Id: "builtin:domain-vocabulary", Title: "Domain Vocabulary",
		Description: "Terms.", Builtin: true,
		Prefill: []byte("slots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n"),
		FieldRules: map[string]*framesv1.FieldRule{
			"terminology": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED, Note: "One entry per term of art."},
			"rules":       {Level: framesv1.Requirement_REQUIREMENT_RECOMMENDED, Note: "Usage constraints."},
		},
	}
	url := testutil.NewStubServer(t, &testutil.StubService{GetTemplateFn: returns(tmpl)})
	dir := t.TempDir()

	out := runCmd(t, url, "template", "init", "--template", "builtin:domain-vocabulary", "--dir", dir)
	if !strings.Contains(out, "frame.yaml") {
		t.Errorf("output does not name the file it wrote:\n%s", out)
	}

	b, err := os.ReadFile(filepath.Join(dir, "frame.yaml"))
	if err != nil {
		t.Fatalf("reading the scaffold: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		"Domain Vocabulary",          // the header names the template
		"One entry per term of art.", // the required rule's note
		"Usage constraints.",         // the recommended rule's note
		"required",                   // the level is stated
		"term: Frame",                // the prefill content is present
	} {
		if !strings.Contains(got, want) {
			t.Errorf("scaffold missing %q:\n%s", want, got)
		}
	}
	// It has to be publishable as-is once identity is filled in, so it must
	// carry the keys the schema requires.
	for _, want := range []string{"name:", "description:", "version:"} {
		if !strings.Contains(got, want) {
			t.Errorf("scaffold missing the %q key:\n%s", want, got)
		}
	}
}

// TestTemplateInitScaffoldIsOneWellFormedDocument decodes the written scaffold
// instead of substring-matching it. scaffoldBody trades a guarantee enforced
// by construction (build a Doc, marshal it) for one that is merely trusted:
// that the server's prefill bytes stay shaped as promised (no identity keys,
// no document separator). If a prefill ever emitted a "---" document-start
// marker, a YAML decoder reading the assembled scaffold would return only the
// first document - the three identity lines this command writes - and every
// slot and extend the template seeded would silently vanish. A substring
// check would not notice; only decoding does.
func TestTemplateInitScaffoldIsOneWellFormedDocument(t *testing.T) {
	tmpl := &framesv1.FrameTemplate{
		Id: "builtin:domain-vocabulary", Title: "Domain Vocabulary",
		Description: "Terms.", Builtin: true,
		Prefill: []byte("extends:\n  - ref: acme/base\n    version: \"1.0.0\"\n" +
			"slots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n"),
		FieldRules: map[string]*framesv1.FieldRule{
			"terminology": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED, Note: "One entry per term of art."},
		},
	}
	url := testutil.NewStubServer(t, &testutil.StubService{GetTemplateFn: returns(tmpl)})
	dir := t.TempDir()
	runCmd(t, url, "template", "init", "--template", "builtin:domain-vocabulary", "--dir", dir)

	b, err := os.ReadFile(filepath.Join(dir, "frame.yaml"))
	if err != nil {
		t.Fatalf("reading the scaffold: %v", err)
	}

	// Exactly one document: decoding a second time off the same stream must
	// report EOF. This is the assertion that actually catches a document
	// separator sneaking into the prefill; everything below it is just
	// making sure the one document that exists has the right shape.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decoding the scaffold: %v", err)
	}
	if err := dec.Decode(new(map[string]any)); err != io.EOF {
		t.Fatalf("scaffold decoded as more than one YAML document (second Decode err = %v); "+
			"the prefill must not split it in two", err)
	}

	for _, key := range []string{"name", "description", "version", "slots", "extends"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("decoded scaffold missing top-level key %q: %#v", key, doc)
		}
	}

	// The prefill's structural content, not its raw text, has to survive.
	slots, ok := doc["slots"].(map[string]any)
	if !ok {
		t.Fatalf("slots is not a map: %#v", doc["slots"])
	}
	terminology, ok := slots["terminology"].([]any)
	if !ok || len(terminology) != 1 {
		t.Fatalf("slots.terminology = %#v, want exactly one entry", slots["terminology"])
	}
	entry, ok := terminology[0].(map[string]any)
	if !ok || entry["term"] != "Frame" || entry["definition"] != "A scoped context artifact." {
		t.Errorf("slots.terminology[0] = %#v, want the prefill's term and definition", terminology[0])
	}

	// No duplicate top-level key. A map decode would collapse a repeated key
	// silently (the second assignment just overwrites the first in Go), so
	// this needs the raw node tree, not the decoded map.
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		t.Fatalf("parsing the scaffold as a node tree: %v", err)
	}
	mapping := node.Content[0]
	seen := make(map[string]bool, len(mapping.Content)/2)
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if seen[key] {
			t.Errorf("scaffold has a duplicate top-level key %q", key)
		}
		seen[key] = true
	}
}

func TestTemplateInitRefusesToClobber(t *testing.T) {
	tmpl := &framesv1.FrameTemplate{Id: "builtin:blank", Title: "Blank", Prefill: []byte("slots: {}\n")}
	url := testutil.NewStubServer(t, &testutil.StubService{GetTemplateFn: returns(tmpl)})
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.yaml")
	if err := os.WriteFile(path, []byte("name: mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runCmdErr(t, url, "template", "init", "--template", "builtin:blank", "--dir", dir)
	if err == nil {
		t.Fatal("init overwrote an existing frame.yaml")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("err = %v, want it to mention --force", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "name: mine\n" {
		t.Errorf("the existing file was modified: %q", b)
	}

	// The control: --force does overwrite.
	if _, err := runCmdErr(t, url, "template", "init", "--template", "builtin:blank", "--dir", dir, "--force"); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	b, _ = os.ReadFile(path)
	if string(b) == "name: mine\n" {
		t.Error("--force did not overwrite")
	}
}

func TestTemplateInitRequiresATemplate(t *testing.T) {
	url := testutil.NewStubServer(t, &testutil.StubService{})
	_, err := runCmdErr(t, url, "template", "init", "--dir", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--template") {
		t.Fatalf("err = %v, want it to require --template", err)
	}
}

// The header is instructions an author follows, and it persists into the
// published content, so a command it names that only works once is a mistake
// that outlives the scaffold: --template asserts a new Frame, and the server
// refuses it as AlreadyExists on every publish after the first.
func TestTemplateInitScaffoldScopesTheTemplateFlagToTheFirstPublish(t *testing.T) {
	tmpl := &framesv1.FrameTemplate{
		Id: "builtin:domain-vocabulary", Title: "Domain Vocabulary",
		Description: "Terms.", Builtin: true,
		Prefill: []byte("slots: {}\n"),
	}
	url := testutil.NewStubServer(t, &testutil.StubService{GetTemplateFn: returns(tmpl)})
	dir := t.TempDir()
	runCmd(t, url, "template", "init", "--template", "builtin:domain-vocabulary", "--dir", dir)

	b, err := os.ReadFile(filepath.Join(dir, "frame.yaml"))
	if err != nil {
		t.Fatalf("reading the scaffold: %v", err)
	}
	got := string(b)

	// Every publish command the header offers, in the order it offers them.
	var cmds []string
	for _, line := range strings.Split(got, "\n") {
		if trimmed := strings.TrimLeft(line, "# "); strings.HasPrefix(trimmed, "frames publish") {
			cmds = append(cmds, trimmed)
		}
	}
	if len(cmds) != 2 {
		t.Fatalf("want two publish commands (the first version and every later one), got %v:\n%s", cmds, got)
	}
	if !strings.Contains(cmds[0], "--template builtin:domain-vocabulary") {
		t.Errorf("the first-version command does not pass the template: %q", cmds[0])
	}
	if strings.Contains(cmds[1], "--template") {
		t.Errorf("the later-version command still passes --template: %q", cmds[1])
	}
	// And it has to say which is which, or two bare commands are just confusing.
	if !strings.Contains(got, "first version") {
		t.Errorf("the header does not say the flag belongs to the first version:\n%s", got)
	}
}
