package auth_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/cli/internal/auth"
)

func TestSaveAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	tok := &auth.CachedToken{
		IDToken: "eyJ.test.token",
		Expiry:  time.Now().Add(time.Hour),
	}

	if err := auth.SaveToken(path, tok); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := auth.LoadToken(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.IDToken != tok.IDToken {
		t.Errorf("expected token %q, got %q", tok.IDToken, loaded.IDToken)
	}

	// Check file permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestLoadToken_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	tok, err := auth.LoadToken(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if tok != nil {
		t.Error("expected nil token for missing file")
	}
}

func TestLoadToken_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tok, err := auth.LoadToken(path)
	if err != nil {
		t.Fatalf("expected no error for malformed file, got: %v", err)
	}
	if tok != nil {
		t.Error("expected nil token for malformed file")
	}
}

func TestLoadToken_Expired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	tok := &auth.CachedToken{
		IDToken: "eyJ.expired.token",
		Expiry:  time.Now().Add(-time.Hour),
	}
	if err := auth.SaveToken(path, tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	loaded, err := auth.LoadToken(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for expired token")
	}
}

func TestLoadTokenRaw_Expired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	tok := &auth.CachedToken{
		IDToken: "eyJ.expired.token",
		Expiry:  time.Now().Add(-time.Hour),
	}
	if err := auth.SaveToken(path, tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	loaded, err := auth.LoadTokenRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil token from LoadTokenRaw even when expired")
	}
	if loaded.IDToken != tok.IDToken {
		t.Errorf("expected token %q, got %q", tok.IDToken, loaded.IDToken)
	}
}

func TestDeleteToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	auth.DeleteToken(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDeleteToken_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	// Should not panic or error
	auth.DeleteToken(path)
}
