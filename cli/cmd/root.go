package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/nebari-dev/nebari-frames/cli/internal/api"
	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	apiURL          string
	credentialsPath string
	version         = "dev"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "frames",
		Short: "Publish, browse, and resolve Nebari Frames",
		Long: `frames is the CLI for the Nebari Frames registry. Use it to publish
Frames authored in your editor, browse Frames your organization has shared,
and print the inheritance-resolved form of a Frame.`,
	}
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Registry URL; overrides api_url config and FRAMES_API_URL")
	rootCmd.PersistentFlags().StringVar(&credentialsPath, "credentials-path", "", "Credentials file path (for testing)")

	cobra.OnInitialize(func() {
		home, _ := os.UserHomeDir()
		viper.SetDefault("api_url", "http://localhost:8080")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(filepath.Join(home, ".config", "frames"))
		viper.SetEnvPrefix("FRAMES")
		viper.AutomaticEnv()
		_ = viper.ReadInConfig()
	})

	addConfigCmd(rootCmd)
	addAuthCmd(rootCmd)
	addPublishCmd(rootCmd)
	addListCmd(rootCmd)
	addShowCmd(rootCmd)
	addExtendsCmd(rootCmd)
	addResolveCmd(rootCmd)
	return rootCmd
}

func getAPIURL() string {
	if apiURL != "" {
		return apiURL
	}
	return viper.GetString("api_url")
}

func resolveCredentialsPath() string {
	if credentialsPath != "" {
		return credentialsPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "frames", "credentials.json")
}

func getClientCtx(ctx context.Context) *api.Client {
	token := ""
	if tok, _ := auth.LoadAndRefresh(ctx, resolveCredentialsPath()); tok != nil {
		token = tok.IDToken
	}
	return api.NewClient(getAPIURL(), api.WithToken(token))
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
