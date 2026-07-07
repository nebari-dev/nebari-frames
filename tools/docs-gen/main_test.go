package main

import "testing"

// TestCLILinkHandler pins the mapping from cobra/doc's "<command>.md" link
// targets to the root-absolute, extensionless, trailing-slash routes Starlight
// actually serves. Regressing this reintroduces the broken "SEE ALSO" links.
func TestCLILinkHandler(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "root command",
			in:   "frames.md",
			want: "/reference/cli/frames/",
		},
		{
			name: "one-level subcommand",
			in:   "frames_config.md",
			want: "/reference/cli/frames_config/",
		},
		{
			name: "two-level subcommand",
			in:   "frames_auth_login.md",
			want: "/reference/cli/frames_auth_login/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cliLinkHandler(tt.in); got != tt.want {
				t.Errorf("cliLinkHandler(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
