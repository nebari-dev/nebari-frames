package mcp

import "testing"

func TestParseFrameURI(t *testing.T) {
	tests := []struct {
		name                  string
		uri                   string
		wantOrg, wantN, wantV string
		wantErr               bool
	}{
		{"versioned", "nebari-frame://openteams/brand-voice@1.2.0", "openteams", "brand-voice", "1.2.0", false},
		{"no version", "nebari-frame://openteams/brand-voice", "openteams", "brand-voice", "", false},
		{"wrong scheme", "https://openteams/brand-voice@1.0.0", "", "", "", true},
		{"missing name", "nebari-frame://openteams", "", "", "", true},
		{"empty", "", "", "", "", true},
		{"empty org", "nebari-frame:///brand-voice@1.0.0", "", "", "", true},
		{"path traversal", "nebari-frame://openteams/../secret@1.0.0", "", "", "", true},
		{"extra path segment", "nebari-frame://openteams/a/b@1.0.0", "", "", "", true},
		{"at with empty version", "nebari-frame://openteams/brand-voice@", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, n, v, err := parseFrameURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if org != tt.wantOrg || n != tt.wantN || v != tt.wantV {
				t.Errorf("got (%q,%q,%q) want (%q,%q,%q)", org, n, v, tt.wantOrg, tt.wantN, tt.wantV)
			}
		})
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	uri := formatFrameURI("openteams", "brand-voice", "1.2.0")
	if uri != "nebari-frame://openteams/brand-voice@1.2.0" {
		t.Fatalf("unexpected format: %q", uri)
	}
	org, n, v, err := parseFrameURI(uri)
	if err != nil || org != "openteams" || n != "brand-voice" || v != "1.2.0" {
		t.Fatalf("round trip failed: %q %q %q %v", org, n, v, err)
	}
}
