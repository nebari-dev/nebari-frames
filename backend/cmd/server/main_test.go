package main

import (
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/orgs"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
)

func TestSelectAuthMode(t *testing.T) {
	tests := []struct {
		name        string
		devModeEnv  string
		issuerURL   string
		clientID    string
		wantDev     bool
		wantErr     bool
		errContains []string
	}{
		{name: "explicit dev mode", devModeEnv: "true", wantDev: true},
		{name: "dev mode requires exact true", devModeEnv: "1", issuerURL: "https://idp", clientID: "web", wantDev: false},
		{name: "auth fully configured", issuerURL: "https://idp", clientID: "web", wantDev: false},
		{name: "missing issuer", clientID: "web", wantErr: true, errContains: []string{"OIDC_ISSUER_URL"}},
		{name: "missing client id", issuerURL: "https://idp", wantErr: true, errContains: []string{"OIDC_CLIENT_ID"}},
		{name: "missing both", wantErr: true, errContains: []string{"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "FRAMES_DEV_MODE"}},
		{name: "non-true dev value with missing OIDC errors", devModeEnv: "1", wantErr: true, errContains: []string{"OIDC_ISSUER_URL", "OIDC_CLIENT_ID"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dev, err := selectAuthMode(tc.devModeEnv, tc.issuerURL, tc.clientID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				for _, sub := range tc.errContains {
					if !strings.Contains(err.Error(), sub) {
						t.Fatalf("error %q missing %q", err.Error(), sub)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dev != tc.wantDev {
				t.Fatalf("devMode = %v, want %v", dev, tc.wantDev)
			}
		})
	}
}

func TestSelectDefaultMembership(t *testing.T) {
	tests := []struct {
		name     string
		roleEnv  string
		orgSlug  string
		want     orgs.DefaultMembership
		wantErr  bool
		errNames string // substring the error must mention
	}{
		{
			name:    "unset denies, which is the fail-closed default",
			roleEnv: "",
			orgSlug: "acme",
			want:    orgs.DefaultMembership{},
		},
		{
			name:    "unset with no org is also fine",
			roleEnv: "",
			orgSlug: "",
			want:    orgs.DefaultMembership{},
		},
		{
			name:    "viewer resolves against the seeded org",
			roleEnv: "viewer",
			orgSlug: "acme",
			want:    orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"},
		},
		{
			name:    "surrounding whitespace is tolerated",
			roleEnv: "  viewer\n",
			orgSlug: "acme",
			want:    orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"},
		},
		{
			name:    "admin is accepted, however unwise",
			roleEnv: "admin",
			orgSlug: "acme",
			want:    orgs.DefaultMembership{Role: rbac.RoleAdmin, OrgSlug: "acme"},
		},
		{
			name:     "an unknown role fails fast",
			roleEnv:  "superuser",
			orgSlug:  "acme",
			wantErr:  true,
			errNames: "FRAMES_DEFAULT_ROLE",
		},
		{
			name:     "a role with no seeded org fails fast rather than silently denying",
			roleEnv:  "viewer",
			orgSlug:  "",
			wantErr:  true,
			errNames: "SEED_ORG_SLUG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectDefaultMembership(tt.roleEnv, tt.orgSlug)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got config %+v", got)
				}
				if !strings.Contains(err.Error(), tt.errNames) {
					t.Errorf("error %q should name %q", err, tt.errNames)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
