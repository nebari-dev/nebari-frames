package frames_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// fakeFetcher resolves refs from an in-memory map of ref@version -> doc.
// A configured error for a ref@version key takes precedence over the doc map.
type fakeFetcher struct {
	docs map[string]*frames.Doc
	errs map[string]error
}

func newFakeFetcher(docs map[string]*frames.Doc) fakeFetcher {
	return fakeFetcher{docs: docs}
}

func (f fakeFetcher) FetchParent(_ context.Context, ref, version string) (*frames.Doc, []frames.ExtendRef, []string, error) {
	key := ref + "@" + version
	if err, ok := f.errs[key]; ok {
		return nil, nil, nil, err
	}
	d, ok := f.docs[key]
	if !ok {
		return nil, nil, nil, errors.New("not found")
	}
	return d, d.Extends, d.Excludes, nil
}

func TestResolve_BodiesConcatenateInMergeOrder(t *testing.T) {
	grandparent := &frames.Doc{Name: "root", Version: "1.0.0", Body: "root guidance"}
	parent := &frames.Doc{
		Name: "base", Version: "1.0.0", Body: "base guidance",
		Extends: []frames.ExtendRef{{Ref: "org/root", Version: "1.0.0"}},
	}
	child := &frames.Doc{
		Name: "child", Version: "1.0.0", Body: "child guidance",
		Extends: []frames.ExtendRef{{Ref: "org/base", Version: "1.0.0"}},
	}

	f := newFakeFetcher(map[string]*frames.Doc{
		"org/root@1.0.0": grandparent,
		"org/base@1.0.0": parent,
	})
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := "root guidance\n\nbase guidance\n\nchild guidance"
	if got.Body != want {
		t.Errorf("body = %q, want %q", got.Body, want)
	}
}

func TestResolve_EmptyBodiesAreSkipped(t *testing.T) {
	parent := &frames.Doc{Name: "base", Version: "1.0.0", Body: "parent guidance"}
	child := &frames.Doc{
		Name: "child", Version: "1.0.0", Body: "",
		Extends: []frames.ExtendRef{{Ref: "org/base", Version: "1.0.0"}},
	}
	f := newFakeFetcher(map[string]*frames.Doc{"org/base@1.0.0": parent})
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Body != "parent guidance" {
		t.Errorf("body = %q, want parent body only with no separator", got.Body)
	}
}

// A parent reachable through more than one path (a diamond) must contribute
// its body exactly once.
func TestResolve_DiamondParentMergedOnce(t *testing.T) {
	shared := &frames.Doc{Name: "shared", Version: "1", Body: "shared guidance"}
	left := &frames.Doc{
		Name: "left", Version: "1", Body: "left guidance",
		Extends: []frames.ExtendRef{{Ref: "org/shared", Version: "1"}},
	}
	right := &frames.Doc{
		Name: "right", Version: "1", Body: "right guidance",
		Extends: []frames.ExtendRef{{Ref: "org/shared", Version: "1"}},
	}
	child := &frames.Doc{
		Name: "child", Version: "1", Body: "child guidance",
		Extends: []frames.ExtendRef{
			{Ref: "org/left", Version: "1"},
			{Ref: "org/right", Version: "1"},
		},
	}
	f := newFakeFetcher(map[string]*frames.Doc{
		"org/shared@1": shared, "org/left@1": left, "org/right@1": right,
	})
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := "shared guidance\n\nleft guidance\n\nright guidance\n\nchild guidance"
	if got.Body != want {
		t.Errorf("body = %q, want %q", got.Body, want)
	}
}

// Spec metadata describes the child itself and is never inherited.
func TestResolve_MetadataCarriedFromChild(t *testing.T) {
	parent := &frames.Doc{
		Name: "base", Version: "9.9.9", Visibility: "public",
		Scope: "company", Maintainer: "platform", Body: "parent guidance",
	}
	child := &frames.Doc{
		Name: "child", Description: "child desc", Version: "1.0.0",
		Visibility: "private", Scope: "project", Maintainer: "data science",
		Extends: []frames.ExtendRef{{Ref: "org/base", Version: "9.9.9"}},
	}
	f := newFakeFetcher(map[string]*frames.Doc{"org/base@9.9.9": parent})
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Name != "child" || got.Description != "child desc" || got.Version != "1.0.0" ||
		got.Visibility != "private" || got.Scope != "project" || got.Maintainer != "data science" {
		t.Errorf("child metadata not carried through: %+v", got)
	}
}

func TestResolve_CycleDetected(t *testing.T) {
	a := &frames.Doc{Name: "a", Version: "1", Extends: []frames.ExtendRef{{Ref: "org/b", Version: "1"}}}
	b := &frames.Doc{Name: "b", Version: "1", Extends: []frames.ExtendRef{{Ref: "org/a", Version: "1"}}}
	f := newFakeFetcher(map[string]*frames.Doc{"org/a@1": a, "org/b@1": b})
	_, err := frames.Resolve(context.Background(), f, a, a.Extends, a.Excludes)
	var ce *frames.CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("want *CycleError, got %v", err)
	}
}

func TestResolve_Excludes(t *testing.T) {
	parent := &frames.Doc{Name: "base", Version: "1", Body: "excluded guidance"}
	child := &frames.Doc{
		Name: "child", Version: "1", Body: "child guidance",
		Extends:  []frames.ExtendRef{{Ref: "org/base", Version: "1"}},
		Excludes: []string{"org/base"},
	}
	f := newFakeFetcher(map[string]*frames.Doc{"org/base@1": parent})
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Body != "child guidance" {
		t.Fatalf("excluded parent body leaked: %q", got.Body)
	}
}

func TestResolve_UnreadableParentPropagates(t *testing.T) {
	child := &frames.Doc{
		Name: "child", Version: "1",
		Extends: []frames.ExtendRef{{Ref: "org/secret", Version: "1"}},
	}
	f := fakeFetcher{
		errs: map[string]error{
			"org/secret@1": fmt.Errorf("forbidden: %w", frames.ErrParentUnreadable),
		},
	}
	_, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err == nil {
		t.Fatal("want error for unreadable parent, got nil (parent silently skipped)")
	}
	if !errors.Is(err, frames.ErrParentUnreadable) {
		t.Fatalf("want err wrapping ErrParentUnreadable, got %v", err)
	}
}
