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

// Validate checks a Doc's metadata fields and extends references. The body is
// free-form markdown and is never rejected. Returns *ValidationError (non-nil
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
