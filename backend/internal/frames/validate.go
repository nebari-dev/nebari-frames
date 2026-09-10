package frames

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// VisibilityValues are the values Frame Spec v0.2 defines for `visibility`.
// It is declared intent that travels with the document, not an access control:
// frame_grants remains authoritative for who may read a frame.
var VisibilityValues = []string{"private", "internal", "shared", "public"}

func validVisibility(v string) bool {
	for _, ok := range VisibilityValues {
		if v == ok {
			return true
		}
	}
	return false
}

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
	// Frame Spec v0.2 requires visibility, but documents published before the
	// field existed do not carry one, so it is only checked when set. The
	// authoring form always writes a value, making it required in practice
	// without invalidating anything already published.
	if v := doc.Visibility; v != "" && !validVisibility(v) {
		add("visibility", "must be one of "+strings.Join(VisibilityValues, ", "))
	}

	errs = append(errs, contentErrors(&doc.Slots, doc.Extends)...)

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// contentErrors checks the part of a document that is not identity: the ten
// slots and the parents it declares.
//
// Split out of Validate so a template prefill can be held to the same rules.
// A prefill is exactly this part of a document - it must not carry identity at
// all (see ParsePrefill) - and it is spliced verbatim into an author's
// scaffold, so content that no publish would accept has to be refused where the
// template is saved (validateTemplateInput) rather than where a Frame is
// published from it.
func contentErrors(slots *Slots, extends []ExtendRef) []FieldError {
	var errs []FieldError
	add := func(path, msg string) { errs = append(errs, FieldError{Path: path, Message: msg}) }

	seenTerm := map[string]bool{}
	for i, term := range slots.Terminology {
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
	checkList("rules", slots.Rules)
	checkList("skills", slots.Skills)
	checkList("prompts", slots.Prompts)

	for i, e := range extends {
		if !strings.Contains(e.Ref, "/") {
			add(fmt.Sprintf("extends[%d].ref", i), "must be org_slug/frame_name")
		}
		if strings.TrimSpace(e.Version) == "" {
			add(fmt.Sprintf("extends[%d].version", i), "must be pinned to a version")
		}
	}
	return errs
}
