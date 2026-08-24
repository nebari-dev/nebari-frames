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

// Resolve merges the extends graph of doc, honoring excludes. Bodies are
// concatenated in merge order - ancestors first, the doc's own body last - so
// the resolved body reads from the most general context to the most specific,
// and later guidance overrides earlier guidance for a reader. It detects cycles
// and propagates unreadable-ancestor errors.
//
// Reading order is the whole precedence model, and that is a deliberate trade
// rather than an omission: a free-form body has no addressable sections to
// override, so a child appends to its parents and `excludes` operates on a whole
// ancestor. What that costs - a child can no longer redefine a single term or
// replace a single prose section - and why the alternative would rebuild the
// retired slot schema is recorded in
// docs/design/2026-05-21-nebari-frames-migration.md, §3.4, under "Why precedence
// is reading order, and what that costs".
func Resolve(ctx context.Context, fetcher ParentFetcher, doc *Doc, extends []ExtendRef, excludes []string) (*Doc, error) {
	excludeSet := map[string]bool{}
	for _, e := range excludes {
		excludeSet[e] = true
	}
	// Spec metadata describes the child itself and is never inherited, so it is
	// carried straight through rather than merged from parents.
	acc := &Doc{
		Name: doc.Name, Description: doc.Description, Version: doc.Version,
		Visibility: doc.Visibility, Scope: doc.Scope, Maintainer: doc.Maintainer,
	}
	visiting := map[string]bool{}
	merged := map[string]bool{}
	if err := mergeParents(ctx, fetcher, extends, excludeSet, acc, visiting, merged, []string{doc.Name}); err != nil {
		return nil, err
	}
	appendBody(acc, doc.Body) // doc's own body comes last
	return acc, nil
}

func mergeParents(ctx context.Context, fetcher ParentFetcher, parents []ExtendRef, excludeSet map[string]bool, acc *Doc, visiting, merged map[string]bool, path []string) error {
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
		if err := mergeParents(ctx, fetcher, pextends, childExcludes, acc, visiting, merged, append(path, p.Ref)); err != nil {
			return err
		}
		// A parent reachable through more than one path (a diamond) contributes
		// its body exactly once.
		if !merged[key] {
			merged[key] = true
			appendBody(acc, pdoc.Body)
		}
		delete(visiting, key)
	}
	return nil
}

// appendBody appends a contribution to the accumulated body, separated by a
// blank line. Empty contributions are skipped.
func appendBody(dst *Doc, body string) {
	body = strings.Trim(body, "\n")
	if strings.TrimSpace(body) == "" {
		return
	}
	if dst.Body == "" {
		dst.Body = body
		return
	}
	dst.Body += "\n\n" + body
}
