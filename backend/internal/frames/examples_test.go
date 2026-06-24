package frames_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

func TestExampleFrames_Validate(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "examples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			doc, err := frames.Parse(content)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := frames.Validate(doc); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
	if count == 0 {
		t.Fatal("no example frames found")
	}
}
