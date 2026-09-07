package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func addTemplateCmd(root *cobra.Command) {
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Browse Frame templates and scaffold a Frame from one",
	}
	addTemplateListCmd(templateCmd)
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
