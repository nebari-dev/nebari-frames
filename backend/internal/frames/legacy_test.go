package frames_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// The legacy `slots:` renderer exists twice - here and in
// web/src/lib/frame-yaml.ts - because the web app renders a stored legacy
// version without a round trip to the server. That mirror is not display-only:
// restoring a legacy version re-serializes the rendered body as the new
// canonical content, so a divergence between the two rewrites what is stored.
//
// testdata/legacy-slots is the one fixture both sides are pinned to, and it is
// asserted whole rather than by substring. Substring assertions are what let
// the two implementations drift on continuation-line indentation and on
// newline-only versus whitespace trimming while both suites stayed green.
//
// The web-side assertion lives in web/src/lib/frame-yaml.test.ts. Changing the
// rendering means regenerating expected.md and updating both.
func TestLegacySlots_SharedFixture(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "legacy-slots")

	in, err := os.ReadFile(filepath.Join(dir, "input.yaml"))
	if err != nil {
		t.Fatalf("read fixture input: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join(dir, "expected.md"))
	if err != nil {
		t.Fatalf("read fixture expectation: %v", err)
	}
	// expected.md carries a trailing newline so it is a well-formed text file;
	// the rendered body does not.
	want := strings.TrimSuffix(string(wantBytes), "\n")

	doc, err := frames.Parse(in)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if doc.Body != want {
		t.Errorf("rendered body does not match the shared fixture\n--- got ---\n%s\n--- want ---\n%s", doc.Body, want)
	}
}
