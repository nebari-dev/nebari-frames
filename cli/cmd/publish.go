package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
)

func addPublishCmd(root *cobra.Command) {
	var dir, changelog, templateID string
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
			frame, version, err := getClientCtx(cmd.Context()).Publish(cmd.Context(), content, changelog, templateID)
			if err != nil {
				switch code := connect.CodeOf(err); {
				case code == connect.CodeInvalidArgument:
					return fmt.Errorf("frame.yaml is invalid: %w", err)
				case code == connect.CodeAlreadyExists && templateID != "":
					// --template asserts a new Frame, which is exactly what a
					// second publish is not. The scaffold's header names the
					// flag, so an author following it lands here with nothing
					// in the server's message pointing at the flag.
					return fmt.Errorf(
						"%w (--template applies to a Frame's first version only; publish later versions without it)", err)
				}
				return authAware(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s\n", frame.Name, version.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory containing frame.yaml")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Release notes for this version")
	cmd.Flags().StringVar(&templateID, "template", "",
		"Template this Frame is being created from; asserts the Frame is new and checks the template's required sections")
	root.AddCommand(cmd)
}
