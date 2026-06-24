package cmd

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/nebari-dev/nebari-frames/cli/internal/api"
)

func addShowCmd(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "show <org/name[@version]>",
		Short: "Show a Frame's metadata and contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, name, version, err := api.ParseRef(args[0])
			if err != nil {
				return err
			}
			resp, err := getClientCtx(cmd.Context()).Get(cmd.Context(), org, name, version)
			if err != nil {
				return notFoundAware(err)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s/%s@%s\n", org, resp.Frame.Name, resp.Version.Version)
			fmt.Fprintf(out, "Description: %s\n", resp.Frame.Description)
			fmt.Fprintf(out, "Owner: %s\n", resp.Frame.OwnerSub)
			if len(resp.Extends) > 0 {
				fmt.Fprintln(out, "Extends:")
				for _, p := range resp.Extends {
					fmt.Fprintf(out, "  - %s@%s\n", p.Ref, p.Version)
				}
			}
			fmt.Fprintf(out, "\n%s\n", string(resp.Version.Content))
			return nil
		},
	}
	root.AddCommand(cmd)
}

// notFoundAware rewrites a NotFound into a message that does not imply existence,
// honoring the backend's 403-vs-404 contract.
func notFoundAware(err error) error {
	if connect.CodeOf(err) == connect.CodeNotFound {
		return fmt.Errorf("frame not found (or you do not have access)")
	}
	return err
}
