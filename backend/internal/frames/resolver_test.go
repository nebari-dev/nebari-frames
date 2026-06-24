package frames_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// fakeFetcher resolves refs from an in-memory map of ref@version -> doc.
type fakeFetcher map[string]*frames.Doc

func (f fakeFetcher) FetchParent(_ context.Context, ref, version string) (*frames.Doc, []frames.ExtendRef, []string, error) {
	d, ok := f[ref+"@"+version]
	if !ok {
		return nil, nil, nil, errors.New("not found")
	}
	return d, d.Extends, d.Excludes, nil
}

func TestResolve_MergeOrderAndOverride(t *testing.T) {
	parent := &frames.Doc{Name: "base", Version: "1.0.0"}
	parent.Slots.Rules = []string{"rule-a", "shared"}
	parent.Slots.Terminology = []frames.Term{{Term: "x", Definition: "from-parent"}}
	parent.Slots.Goals = "parent goals"

	child := &frames.Doc{Name: "child", Version: "1.0.0", Extends: []frames.ExtendRef{{Ref: "org/base", Version: "1.0.0"}}}
	child.Slots.Rules = []string{"shared", "rule-b"}
	child.Slots.Terminology = []frames.Term{{Term: "x", Definition: "from-child"}}

	f := fakeFetcher{"org/base@1.0.0": parent}
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	t.Run("rules dedupe keeping last occurrence", func(t *testing.T) {
		// rules: concat parent then child, dedupe keeping last occurrence -> [rule-a, shared, rule-b]
		want := []string{"rule-a", "shared", "rule-b"}
		if len(got.Slots.Rules) != len(want) {
			t.Fatalf("rules = %v, want %v", got.Slots.Rules, want)
		}
		for i := range want {
			if got.Slots.Rules[i] != want[i] {
				t.Fatalf("rules[%d] = %q, want %q (full: %v, want %v)", i, got.Slots.Rules[i], want[i], got.Slots.Rules, want)
			}
		}
	})

	t.Run("terminology child overrides parent", func(t *testing.T) {
		if got.Slots.Terminology[0].Definition != "from-child" {
			t.Fatalf("terminology override failed: %v", got.Slots.Terminology)
		}
	})

	t.Run("prose flows through from parent when child has none", func(t *testing.T) {
		if got.Slots.Goals != "parent goals" {
			t.Fatalf("goals = %q, want parent goals", got.Slots.Goals)
		}
	})
}

func TestResolve_CycleDetected(t *testing.T) {
	a := &frames.Doc{Name: "a", Version: "1", Extends: []frames.ExtendRef{{Ref: "org/b", Version: "1"}}}
	b := &frames.Doc{Name: "b", Version: "1", Extends: []frames.ExtendRef{{Ref: "org/a", Version: "1"}}}
	f := fakeFetcher{"org/a@1": a, "org/b@1": b}
	_, err := frames.Resolve(context.Background(), f, a, a.Extends, a.Excludes)
	var ce *frames.CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("want *CycleError, got %v", err)
	}
}

func TestResolve_Excludes(t *testing.T) {
	parent := &frames.Doc{Name: "base", Version: "1"}
	parent.Slots.Rules = []string{"excluded-rule"}
	child := &frames.Doc{
		Name: "child", Version: "1",
		Extends:  []frames.ExtendRef{{Ref: "org/base", Version: "1"}},
		Excludes: []string{"org/base"},
	}
	f := fakeFetcher{"org/base@1": parent}
	got, err := frames.Resolve(context.Background(), f, child, child.Extends, child.Excludes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got.Slots.Rules) != 0 {
		t.Fatalf("excluded parent rules leaked: %v", got.Slots.Rules)
	}
}
