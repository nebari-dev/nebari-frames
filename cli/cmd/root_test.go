package cmd

import (
	"bytes"
	"testing"
)

func TestRootCmd_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"help lists use", []string{"--help"}, "frames is the CLI"},
		{"version flag", []string{"--version"}, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewRootCmd()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetArgs(tt.args)
			if err := c.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !bytes.Contains(out.Bytes(), []byte(tt.want)) {
				t.Fatalf("output %q missing %q", out.String(), tt.want)
			}
		})
	}
}
