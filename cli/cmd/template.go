package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/spf13/cobra"
)

func addTemplateCmd(root *cobra.Command) {
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Browse Frame templates and scaffold a Frame from one",
	}
	addTemplateListCmd(templateCmd)
	addTemplateInitCmd(templateCmd)
	root.AddCommand(templateCmd)
}

func addTemplateListCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Frame templates you can start from",
		RunE: func(cmd *cobra.Command, _ []string) error {
			templates, _, err := getClientCtx(cmd.Context()).ListTemplates(cmd.Context())
			if err != nil {
				return authAware(err)
			}
			if len(templates) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No templates found.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTITLE\tSOURCE\tDESCRIPTION")
			for _, t := range templates {
				// "built-in" vs "organization" rather than a bare boolean: the
				// distinction an author cares about is who controls it.
				source := "organization"
				if t.Builtin {
					source = "built-in"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Id, t.Title, source, t.Description)
			}
			return w.Flush()
		},
	}
	parent.AddCommand(cmd)
}

// scaffoldComment renders the guidance block at the top of a scaffold. It names
// the template and lists what it wants, so an author filling the file in does
// not have to go back to the web app to find out.
//
// These comments persist into the published content, because the Connect RPC
// stores the author's exact bytes. That is accepted: they document why the file
// is shaped the way it is, and the alternative - printing guidance to the
// terminal and handing over a bare file - loses it the moment the terminal
// scrolls.
func scaffoldComment(t *framesv1.FrameTemplate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Scaffolded from the %q template.\n", t.Title)
	if t.Description != "" {
		fmt.Fprintf(&b, "# %s\n", t.Description)
	}
	b.WriteString("#\n# Fill in name, description, and version, then publish with:\n")
	b.WriteString("#   frames publish --dir . --template " + t.Id + "\n")

	// Required first: those are what will actually refuse the publish. Slot
	// keys are sorted rather than walked in schema order, because that order
	// lives in the backend's frame schema package, which a CLI binary cannot
	// import (see scaffoldBody's comment below); alphabetical is still
	// deterministic, which is what a test - and an author rereading the file
	// on a second run - actually needs.
	for _, want := range []framesv1.Requirement{
		framesv1.Requirement_REQUIREMENT_REQUIRED,
		framesv1.Requirement_REQUIREMENT_RECOMMENDED,
	} {
		keys := make([]string, 0, len(t.FieldRules))
		for key, rule := range t.FieldRules {
			if rule.Level == want {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		var lines []string
		for _, key := range keys {
			rule := t.FieldRules[key]
			line := fmt.Sprintf("#   %s (%s)", key, strings.ToLower(strings.TrimPrefix(want.String(), "REQUIREMENT_")))
			if rule.Note != "" {
				line += ": " + rule.Note
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			b.WriteString("#\n")
			b.WriteString(strings.Join(lines, "\n") + "\n")
		}
	}
	return b.String()
}

// scaffoldBody assembles frame.yaml's full contents: the guidance comment,
// empty identity keys for the author to fill in, and the template's prefill
// verbatim.
//
// It does not parse or re-marshal the prefill. The server only ever sends the
// "extends"/"slots" subset (GetFrameTemplate always goes through
// MarshalPrefill, which structurally omits every identity field), so splicing
// it in after the identity keys produces exactly the document a decode-then-
// encode round trip would, without needing a decoder here at all.
func scaffoldBody(t *framesv1.FrameTemplate) []byte {
	var b strings.Builder
	b.WriteString(scaffoldComment(t))
	// name and description have no sensible default, so they are left for the
	// author to type; version gets a real starting value, since "leave it
	// blank" is not a meaningful answer for a field that is otherwise always a
	// semver string.
	b.WriteString("name: \"\"\n")
	b.WriteString("description: \"\"\n")
	b.WriteString("version: \"1.0.0\"\n")
	b.Write(t.Prefill)
	return []byte(b.String())
}

func addTemplateInitCmd(parent *cobra.Command) {
	var templateID, dir string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a frame.yaml scaffold from a template, ready to fill in and publish",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if templateID == "" {
				return fmt.Errorf("--template is required; run `frames template list` to see the options")
			}
			if dir == "" {
				dir = "."
			}
			tmpl, err := getClientCtx(cmd.Context()).GetTemplate(cmd.Context(), templateID)
			if err != nil {
				return authAware(err)
			}
			path := filepath.Join(dir, "frame.yaml")
			if _, statErr := os.Stat(path); statErr == nil && !force {
				// Overwriting someone's in-progress Frame is not recoverable, so
				// it takes an explicit flag.
				return fmt.Errorf("%s already exists; pass --force to overwrite it", path)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, scaffoldBody(tmpl), 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s from %q. Fill in name and description, then publish.\n", path, tmpl.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&templateID, "template", "", "Template id from frames template list")
	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to write frame.yaml into")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing frame.yaml")
	parent.AddCommand(cmd)
}
