package main

import (
	"strings"
	"testing"
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
