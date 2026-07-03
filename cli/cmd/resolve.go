package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nebari-dev/nebari-frames/cli/internal/api"
)

func addResolveCmd(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "resolve <org/name[@version]>",
		Short: "Print the inheritance-resolved form of a Frame",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, name, version, err := api.ParseRef(args[0])
			if err != nil {
				return err
			}
			content, err := getClientCtx(cmd.Context()).Resolve(cmd.Context(), org, name, version)
			if err != nil {
				return authAware(notFoundAware(err))
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(content))
			return nil
		},
	}
	root.AddCommand(cmd)
}
