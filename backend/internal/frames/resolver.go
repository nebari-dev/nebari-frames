package frames

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrParentUnreadable = errors.New("parent frame not readable")

// CycleError is returned when a cycle is detected in the inheritance graph.
type CycleError struct{ Path []string }

func (e *CycleError) Error() string { return "inheritance cycle: " + strings.Join(e.Path, " -> ") }

// ParentFetcher loads a parent frame's resolved doc plus its own edges so the
// resolver can recurse. ref is "org_slug/frame_name".
type ParentFetcher interface {
	FetchParent(ctx context.Context, ref, version string) (doc *Doc, extends []ExtendRef, excludes []string, err error)
}

// Resolve merges the extends graph of doc (later parents win; doc wins last),
// honoring excludes. It detects cycles and propagates unreadable-ancestor errors.
func Resolve(ctx context.Context, fetcher ParentFetcher, doc *Doc, extends []ExtendRef, excludes []string) (*Doc, error) {
	excludeSet := map[string]bool{}
	for _, e := range excludes {
		excludeSet[e] = true
	}
	acc := &Doc{Name: doc.Name, Description: doc.Description, Version: doc.Version}
	visiting := map[string]bool{}
	if err := mergeParents(ctx, fetcher, extends, excludeSet, acc, visiting, []string{doc.Name}); err != nil {
		return nil, err
	}
	mergeInto(acc, doc) // doc's own slots override all parents
	return acc, nil
}

func mergeParents(ctx context.Context, fetcher ParentFetcher, parents []ExtendRef, excludeSet map[string]bool, acc *Doc, visiting map[string]bool, path []string) error {
	for _, p := range parents {
		if excludeSet[p.Ref] {
			continue
		}
		key := p.Ref + "@" + p.Version
		if visiting[key] {
			return &CycleError{Path: append(path, p.Ref)}
		}
		pdoc, pextends, pexcludes, err := fetcher.FetchParent(ctx, p.Ref, p.Version)
		if err != nil {
			return fmt.Errorf("resolve parent %s: %w", key, err)
		}
		visiting[key] = true
		childExcludes := map[string]bool{}
		for k := range excludeSet {
			childExcludes[k] = true
		}
		for _, e := range pexcludes {
			childExcludes[e] = true
		}
		if err := mergeParents(ctx, fetcher, pextends, childExcludes, acc, visiting, append(path, p.Ref)); err != nil {
			return err
		}
		mergeInto(acc, pdoc)
		delete(visiting, key)
	}
	return nil
}

// mergeInto applies src's slots onto dst (src wins).
func mergeInto(dst, src *Doc) {
	dst.Slots.Terminology = mergeTerms(dst.Slots.Terminology, src.Slots.Terminology)
	dst.Slots.Rules = mergeStrings(dst.Slots.Rules, src.Slots.Rules)
	dst.Slots.Skills = mergeStrings(dst.Slots.Skills, src.Slots.Skills)
	dst.Slots.Prompts = mergeStrings(dst.Slots.Prompts, src.Slots.Prompts)
	dst.Slots.ToolSpecs = replaceIfSet(dst.Slots.ToolSpecs, src.Slots.ToolSpecs)
	dst.Slots.Goals = replaceIfSet(dst.Slots.Goals, src.Slots.Goals)
	dst.Slots.Style = replaceIfSet(dst.Slots.Style, src.Slots.Style)
	dst.Slots.Norms = replaceIfSet(dst.Slots.Norms, src.Slots.Norms)
	dst.Slots.Architecture = replaceIfSet(dst.Slots.Architecture, src.Slots.Architecture)
	dst.Slots.BusinessProcess = replaceIfSet(dst.Slots.BusinessProcess, src.Slots.BusinessProcess)
}

// mergeTerms merges by term; src definition wins on collision; order = existing then new.
func mergeTerms(existing, incoming []Term) []Term {
	idx := map[string]int{}
	out := make([]Term, 0, len(existing)+len(incoming))
	for _, t := range existing {
		idx[t.Term] = len(out)
		out = append(out, t)
	}
	for _, t := range incoming {
		if i, ok := idx[t.Term]; ok {
			out[i].Definition = t.Definition
			continue
		}
		idx[t.Term] = len(out)
		out = append(out, t)
	}
	return out
}

// mergeStrings concatenates then dedupes preserving the LAST occurrence.
func mergeStrings(existing, incoming []string) []string {
	combined := append(append([]string{}, existing...), incoming...)
	lastIndex := map[string]int{}
	for i, s := range combined {
		lastIndex[s] = i
	}
	out := make([]string, 0, len(combined))
	for i, s := range combined {
		if lastIndex[s] == i {
			out = append(out, s)
		}
	}
	return out
}

func replaceIfSet(existing, incoming string) string {
	if incoming != "" {
		return incoming
	}
	return existing
}
