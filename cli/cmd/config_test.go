package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigSetGet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir) // config writes under $HOME/.config/frames
	viper.Reset()
	t.Cleanup(viper.Reset)

	c := NewRootCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetArgs([]string{"config", "set", "api_url", "http://example:9000"})
	if err := c.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".config", "frames", "config.yaml")); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}
