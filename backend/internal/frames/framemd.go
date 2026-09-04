package frames

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file implements the .frame.md interchange format: the single-Markdown-
// file-with-YAML-frontmatter shape defined by Frame Spec v0.2
// (https://github.com/openteams-ai/frame-spec). The canonical stored form
// remains the YAML in schema.go; this is a codec on top of it.
//
// The spec requires four frontmatter fields and leaves the body entirely
// free-form, so the codec is simple: frontmatter maps to Doc metadata and
// everything after the closing --- is the body, verbatim. Parsing is strict
// about the frontmatter block (delimiters, unknown keys) because guessing
// would silently mangle an author's metadata; the body is never rejected.
//
// Round-tripping preserves content but normalizes in two known ways, both
// deliberate and both asserted in framemd_test.go. An empty visibility comes
// back as DefaultVisibility, because the spec requires the field and something
// has to be written. Leading and trailing blank lines around the body are
// dropped, because the body's offset from the closing --- is framing rather
// than content. Nothing else is normalized, and in particular no byte of the
// body between its first and last non-blank line is touched.

// SpecVersion is the Frame Spec release this codec reads and writes.
const SpecVersion = "0.2"

// MarkdownType is the `type` value emitted in .frame.md frontmatter.
const MarkdownType = "frame [" + SpecVersion + "]"

// DefaultVisibility is emitted for documents stored before `visibility` existed.
const DefaultVisibility = "internal"

// typeRe mirrors the reference validator in the frame-spec repo
// (tools/validate_frames.py): `frame` or `frame [<major>.<minor>]`.
var typeRe = regexp.MustCompile(`^frame(?: \[\d+\.\d+\])?$`)

// unknownKeyRe extracts the offending key from a yaml.v3 KnownFields error.
var unknownKeyRe = regexp.MustCompile(`line (\d+): field (\S+) not found`)

// frontmatterKeys is the recognized frontmatter key set, named in error
// messages. Keep in sync with the frontmatter struct below.
var frontmatterKeys = []string{
	"type", "name", "description", "visibility",
	"version", "scope", "maintainer", "inherits",
	"x-nebari-excludes", "x-nebari-template",
}

// stringOrSlice accepts either a scalar or a sequence, both of which Frame Spec
// v0.2 allows for `inherits`.
type stringOrSlice []string

func (s *stringOrSlice) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var v string
		if err := n.Decode(&v); err != nil {
			return err
		}
		*s = stringOrSlice{v}
	case yaml.SequenceNode:
		var v []string
		if err := n.Decode(&v); err != nil {
			return err
		}
		*s = v
	default:
		return fmt.Errorf("must be a string or a list of strings")
	}
	return nil
}

// frontmatter is the YAML block at the top of a .frame.md file. Field order is
// the emit order. Keep in sync with frontmatterKeys.
type frontmatter struct {
	Type        string        `yaml:"type"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Visibility  string        `yaml:"visibility"`
	Version     string        `yaml:"version,omitempty"`
	Scope       string        `yaml:"scope,omitempty"`
	Maintainer  string        `yaml:"maintainer,omitempty"`
	Inherits    stringOrSlice `yaml:"inherits,omitempty"`
	// Excludes and Template have no Frame Spec equivalent, so they live in an
	// `x-` namespace as the spec advises implementations to do for their own
	// fields.
	Excludes stringOrSlice `yaml:"x-nebari-excludes,omitempty"`
	Template bool          `yaml:"x-nebari-template,omitempty"`
}

// MarshalMarkdown renders a Doc as a spec-conformant .frame.md document. The
// body passes through verbatim between its first and last non-blank line; see
// the round-trip note at the top of this file for the two normalizations
// UnmarshalMarkdown applies on the way back.
func MarshalMarkdown(doc *Doc) ([]byte, error) {
	fm := frontmatter{
		Type:        MarkdownType,
		Name:        doc.Name,
		Description: doc.Description,
		Visibility:  doc.Visibility,
		Version:     doc.Version,
		Scope:       doc.Scope,
		Maintainer:  doc.Maintainer,
		Excludes:    doc.Excludes,
		Template:    doc.Template,
	}
	if fm.Visibility == "" {
		fm.Visibility = DefaultVisibility
	}
	for _, e := range doc.Extends {
		ref := e.Ref
		if e.Version != "" {
			ref += "@" + e.Version
		}
		fm.Inherits = append(fm.Inherits, ref)
	}

	head, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(head)
	b.WriteString("---\n")
	if body := strings.Trim(doc.Body, "\n"); body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}

// isFence reports whether a line is a frontmatter delimiter.
//
// The comparison is anchored at column 0 on purpose. Matching TrimSpace(line)
// instead would let an indented --- inside a multi-line frontmatter value close
// the block early, silently truncating the document and dropping every field
// after it - and a YAML block scalar, which is the only way to write a
// multi-line value, is always indented. Trailing whitespace is tolerated
// because editors add it and it cannot occur inside a scalar without the
// indentation that already disqualifies the line.
func isFence(line string) bool {
	return strings.TrimRight(line, " \t") == "---"
}

// UnmarshalMarkdown parses a .frame.md document into a Doc. Structural problems
// (all in the frontmatter block - the body is free-form and never rejected)
// are returned as a *ValidationError whose paths are all "markdown" and whose
// messages carry the 1-based line number, so the web editor can surface them
// against the source. Value-level problems are left for Validate.
func UnmarshalMarkdown(content []byte) (*Doc, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	var errs []FieldError
	add := func(line int, format string, a ...any) {
		errs = append(errs, FieldError{
			Path:    "markdown",
			Message: fmt.Sprintf("line %d: ", line) + fmt.Sprintf(format, a...),
		})
	}
	fail := func() error { return &ValidationError{Errors: errs} }

	// --- frontmatter block ---
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !isFence(lines[i]) {
		add(i+1, "a frame must begin with a YAML frontmatter block delimited by ---")
		return nil, fail()
	}
	start := i + 1
	end := -1
	for j := start; j < len(lines); j++ {
		if isFence(lines[j]) {
			end = j
			break
		}
	}
	if end == -1 {
		add(i+1, "frontmatter opening --- has no closing ---")
		return nil, fail()
	}

	var fm frontmatter
	dec := yaml.NewDecoder(strings.NewReader(strings.Join(lines[start:end], "\n")))
	dec.KnownFields(true) // unknown frontmatter keys are a structural error
	if err := dec.Decode(&fm); err != nil {
		add(0, "%s", frontmatterErr(err, start))
		// strip the "line 0: " placeholder that add() prepends
		errs[len(errs)-1].Message = strings.TrimPrefix(errs[len(errs)-1].Message, "line 0: ")
		return nil, fail()
	}

	// `type` is the discriminator that makes this a frame at all, so it is the
	// one frontmatter value checked here rather than deferred to Validate.
	switch {
	case strings.TrimSpace(fm.Type) == "":
		add(start+1, "missing required frontmatter field %q — add %q", "type", MarkdownType)
	case !typeRe.MatchString(fm.Type):
		add(start+1, "type must be \"frame\" or \"frame [<major>.<minor>]\" (for example %q), got %q", MarkdownType, fm.Type)
	}
	if len(errs) > 0 {
		return nil, fail()
	}

	doc := &Doc{
		Name:        fm.Name,
		Description: fm.Description,
		Version:     fm.Version,
		Visibility:  fm.Visibility,
		Scope:       fm.Scope,
		Maintainer:  fm.Maintainer,
		Excludes:    fm.Excludes,
		Template:    fm.Template,
		Body:        strings.Trim(strings.Join(lines[end+1:], "\n"), "\n"),
	}
	for _, ref := range fm.Inherits {
		doc.Extends = append(doc.Extends, parseInherit(ref))
	}
	return doc, nil
}

// parseInherit splits a spec `inherits` entry into a pinned ExtendRef. An entry
// without "@" yields an empty Version, which Validate reports as unpinned.
func parseInherit(ref string) ExtendRef {
	ref = strings.TrimSpace(ref)
	if at := strings.LastIndex(ref, "@"); at > 0 {
		return ExtendRef{Ref: ref[:at], Version: ref[at+1:]}
	}
	return ExtendRef{Ref: ref}
}

// frontmatterErr turns a yaml.v3 decode failure into a message that names the
// file line and, for unknown keys, the recognized key set.
func frontmatterErr(err error, offset int) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.ReplaceAll(msg, "unmarshal errors:\n  ", "")
	if m := unknownKeyRe.FindStringSubmatch(msg); m != nil {
		ln, _ := strconv.Atoi(m[1])
		return fmt.Sprintf("line %d: unknown frontmatter key %q — recognized keys are: %s",
			ln+offset, m[2], strings.Join(frontmatterKeys, ", "))
	}
	if m := regexp.MustCompile(`^line (\d+): (.*)$`).FindStringSubmatch(msg); m != nil {
		ln, _ := strconv.Atoi(m[1])
		return fmt.Sprintf("line %d: %s", ln+offset, m[2])
	}
	return "invalid frontmatter: " + msg
}
