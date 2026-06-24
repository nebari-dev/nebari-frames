package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
)

func addAuthCmd(root *cobra.Command) {
	authCmd := &cobra.Command{Use: "auth", Short: "Authenticate with the registry"}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Log in via OIDC device flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			pending, err := auth.StartDeviceFlow(ctx, getAPIURL())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if pending.VerificationURIComplete != "" {
				fmt.Fprintf(out, "Open %s to log in.\n", pending.VerificationURIComplete)
			} else {
				fmt.Fprintf(out, "Open %s and enter code %s.\n", pending.VerificationURI, pending.UserCode)
			}
			res, err := auth.PollForToken(ctx, pending, 0)
			if err != nil {
				return err
			}
			tok := &auth.CachedToken{
				IDToken: res.IDToken, Expiry: res.Expiry, RefreshToken: res.RefreshToken,
				RefreshExpiry: res.RefreshExpiry, TokenEndpoint: res.TokenEndpoint, ClientID: res.ClientID,
			}
			if err := auth.SaveToken(resolveCredentialsPath(), tok); err != nil {
				return err
			}
			fmt.Fprintf(out, "Logged in as %s.\n", res.Email)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			me, err := getClientCtx(cmd.Context()).Me(cmd.Context())
			if err != nil {
				return err
			}
			org := ""
			if me.Org != nil {
				org = me.Org.Slug
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (org: %s, role: %s)\n", me.Subject, org, me.Role)
			return nil
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			auth.DeleteToken(resolveCredentialsPath())
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}

	authCmd.AddCommand(loginCmd, statusCmd, logoutCmd)
	root.AddCommand(authCmd)
}
