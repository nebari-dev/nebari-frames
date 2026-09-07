package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
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
