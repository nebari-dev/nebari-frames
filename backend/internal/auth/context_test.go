package auth

import "testing"

func TestDevClaims(t *testing.T) {
	c := DevClaims()
	if c == nil {
		t.Fatal("DevClaims() returned nil")
	}
	if c.Subject != "dev-user" || c.Email != "dev@localhost" {
		t.Fatalf("unexpected dev claims: %+v", c)
	}
	// Must be a fresh copy each call so callers cannot mutate shared state.
	c.Subject = "mutated"
	if DevClaims().Subject != "dev-user" {
		t.Fatal("DevClaims() must return an independent copy")
	}
}
