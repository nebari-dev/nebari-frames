package api_test

import (
	"testing"

	"github.com/nebari-dev/nebari-frames/cli/internal/api"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name, in, org, frame, ver string
		wantErr                   bool
	}{
		{"org and name", "openteams/brand-voice", "openteams", "brand-voice", "", false},
		{"with version", "openteams/brand-voice@1.2.0", "openteams", "brand-voice", "1.2.0", false},
		{"no slash", "brand-voice", "", "", "", true},
		{"empty org", "/brand-voice", "", "", "", true},
		{"trailing slash", "openteams/", "", "", "", true},
		{"extra slash", "a/b/c", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, frame, ver, err := api.ParseRef(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error for %q", tt.in)
				}
				return
			}
			if err != nil || org != tt.org || frame != tt.frame || ver != tt.ver {
				t.Fatalf("got (%q,%q,%q,%v), want (%q,%q,%q,nil)", org, frame, ver, err, tt.org, tt.frame, tt.ver)
			}
		})
	}
}
