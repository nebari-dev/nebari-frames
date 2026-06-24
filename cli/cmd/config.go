package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addConfigCmd(root *cobra.Command) {
	configCmd := &cobra.Command{Use: "config", Short: "Manage CLI configuration"}

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), viper.GetString(args[0]))
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.Set(args[0], args[1])
			path := viper.ConfigFileUsed()
			if path == "" {
				home, _ := os.UserHomeDir()
				dir := filepath.Join(home, ".config", "frames")
				if err := os.MkdirAll(dir, 0700); err != nil {
					return err
				}
				path = filepath.Join(dir, "config.yaml")
			}
			if err := viper.WriteConfigAs(path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", args[0], args[1])
			return nil
		},
	}

	configCmd.AddCommand(getCmd, setCmd)
	root.AddCommand(configCmd)
}
