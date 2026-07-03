package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
)

func addPublishCmd(root *cobra.Command) {
	var dir, changelog string
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a Frame from a directory containing frame.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				return fmt.Errorf("--dir is required")
			}
			path := filepath.Join(dir, "frame.yaml")
			content, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no frame.yaml found in %s", dir)
				}
				return err
			}
			frame, version, err := getClientCtx(cmd.Context()).Publish(cmd.Context(), content, changelog)
			if err != nil {
				if connect.CodeOf(err) == connect.CodeInvalidArgument {
					return fmt.Errorf("frame.yaml is invalid: %w", err)
				}
				return authAware(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s\n", frame.Name, version.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory containing frame.yaml")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Release notes for this version")
	root.AddCommand(cmd)
}
