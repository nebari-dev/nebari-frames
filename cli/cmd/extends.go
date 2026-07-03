package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nebari-dev/nebari-frames/cli/internal/api"
)

func addExtendsCmd(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "extends <org/name[@version]>",
		Short: "Print a Frame's inheritance references",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, name, version, err := api.ParseRef(args[0])
			if err != nil {
				return err
			}
			resp, err := getClientCtx(cmd.Context()).Get(cmd.Context(), org, name, version)
			if err != nil {
				return authAware(notFoundAware(err))
			}
			out := cmd.OutOrStdout()
			if len(resp.Extends) == 0 {
				_, _ = fmt.Fprintln(out, "(no parents)")
				return nil
			}
			for _, p := range resp.Extends {
				_, _ = fmt.Fprintf(out, "%s@%s\n", p.Ref, p.Version)
			}
			return nil
		},
	}
	root.AddCommand(cmd)
}
