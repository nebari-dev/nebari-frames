package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func addListCmd(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Frames you can read",
		RunE: func(cmd *cobra.Command, _ []string) error {
			frames, _, err := getClientCtx(cmd.Context()).List(cmd.Context())
			if err != nil {
				return err
			}
			if len(frames) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No frames found.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tOWNER\tDESCRIPTION")
			for _, f := range frames {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Name, f.LatestVersion, f.OwnerSub, f.Description)
			}
			return w.Flush()
		},
	}
	root.AddCommand(cmd)
}
