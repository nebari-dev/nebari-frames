package frames

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// FieldError is a single validation failure at a specific field path.
type FieldError struct {
	Path    string
	Message string
}

// ValidationError is returned by Validate when one or more fields are invalid.
type ValidationError struct{ Errors []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.Path + ": " + fe.Message
	}
	return strings.Join(parts, "; ")
}

// Validate checks the fixed 10-slot schema. Returns *ValidationError (non-nil
// .Errors) or nil.
func Validate(doc *Doc) error {
	var errs []FieldError
	add := func(path, msg string) { errs = append(errs, FieldError{Path: path, Message: msg}) }

	if !nameRe.MatchString(doc.Name) {
		add("name", "must match [a-z0-9][a-z0-9-]{0,63}")
	}
	if doc.Description == "" {
		add("description", "must not be empty")
	} else if utf8.RuneCountInString(doc.Description) > 280 {
		add("description", "must be at most 280 characters")
	}
	if strings.TrimSpace(doc.Version) == "" {
		add("version", "must not be empty")
	}

	seenTerm := map[string]bool{}
	for i, term := range doc.Slots.Terminology {
		if strings.TrimSpace(term.Term) == "" {
			add(fmt.Sprintf("slots.terminology[%d].term", i), "must not be empty")
		} else if seenTerm[term.Term] {
			add(fmt.Sprintf("slots.terminology[%d].term", i), "duplicate term within slot")
		}
		seenTerm[term.Term] = true
		if strings.TrimSpace(term.Definition) == "" {
			add(fmt.Sprintf("slots.terminology[%d].definition", i), "must not be empty")
		}
	}

	checkList := func(name string, items []string) {
		for i, s := range items {
			if strings.TrimSpace(s) == "" {
				add(fmt.Sprintf("slots.%s[%d]", name, i), "must not be empty")
			}
		}
	}
	checkList("rules", doc.Slots.Rules)
	checkList("skills", doc.Slots.Skills)
	checkList("prompts", doc.Slots.Prompts)

	for i, e := range doc.Extends {
		if !strings.Contains(e.Ref, "/") {
			add(fmt.Sprintf("extends[%d].ref", i), "must be org_slug/frame_name")
		}
		if strings.TrimSpace(e.Version) == "" {
			add(fmt.Sprintf("extends[%d].version", i), "must be pinned to a version")
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}
