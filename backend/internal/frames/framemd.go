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
// remains the slot YAML in schema.go; this is a lossless codec on top of it.
//
// Parsing is strict about *structure* (frontmatter delimiters, unknown keys,
// unknown section headings, malformed bullets) because guessing would silently
// mangle an author's content. It is deliberately lenient about *values*:
// an empty description or an unpinned `inherits` ref produces a Doc that
// Validate then rejects, so the error lands on the relevant form field where it
// can be fixed, rather than blocking the import outright.

// SpecVersion is the Frame Spec release this codec reads and writes.
const SpecVersion = "0.2"

// MarkdownType is the `type` value emitted in .frame.md frontmatter.
const MarkdownType = "frame [" + SpecVersion + "]"

// DefaultVisibility is emitted for documents stored before `visibility` existed.
const DefaultVisibility = "internal"

// typeRe mirrors the reference validator in the frame-spec repo
// (tools/validate_frames.py): `frame` or `frame [<major>.<minor>]`.
var typeRe = regexp.MustCompile(`^frame(?: \[\d+\.\d+\])?$`)

// termRe matches a rendered terminology bullet body: "**term**: definition".
var termRe = regexp.MustCompile(`(?s)^\*\*(.+?)\*\*:[ \t]?(.*)$`)

// unknownKeyRe extracts the offending key from a yaml.v3 KnownFields error.
var unknownKeyRe = regexp.MustCompile(`line (\d+): field (\S+) not found`)

// frontmatterKeys is the recognized frontmatter key set, named in error
// messages. Keep in sync with the frontmatter struct below.
var frontmatterKeys = []string{
	"type", "name", "description", "visibility",
	"version", "scope", "maintainer", "inherits", "x-nebari-excludes",
}

// headingHints points common headings from frames authored elsewhere at the
// closest slot. These only enrich the rejection message; nothing is ever mapped
// automatically, because silently relocating an author's content is worse than
// telling them where it belongs.
var headingHints = map[string]string{
	"ways of working": "Norms",
	"how we work":     "Norms",
	"conventions":     "Norms",
	"constraints":     "Rules",
	"guardrails":      "Rules",
	"policies":        "Rules",
	"vocabulary":      "Terminology",
	"glossary":        "Terminology",
	"definitions":     "Terminology",
	"voice":           "Style",
	"tone":            "Style",
	"objectives":      "Goals",
	"purpose":         "Goals",
	"process":         "Business Process",
	"workflow":        "Business Process",
	"tools":           "Tool Specifications",
	"tooling":         "Tool Specifications",
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
	// Excludes has no Frame Spec equivalent, so it lives in an `x-` namespace
	// as the spec advises implementations to do for their own fields.
	Excludes stringOrSlice `yaml:"x-nebari-excludes,omitempty"`
}

// MarshalMarkdown renders a Doc as a spec-conformant .frame.md document.
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
	b.WriteString("---\n\n")
	if doc.Name != "" {
		fmt.Fprintf(&b, "# %s\n\n", doc.Name)
	}
	for _, d := range SlotTable {
		writeSlot(&b, d, &doc.Slots)
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil
}

func writeSlot(b *strings.Builder, d SlotDescriptor, s *Slots) {
	switch d.Kind {
	case SlotTerms:
		if len(s.Terminology) == 0 {
			return
		}
		fmt.Fprintf(b, "## %s\n\n", d.Heading)
		for _, t := range s.Terminology {
			WriteBullet(b, fmt.Sprintf("**%s**: %s", t.Term, t.Definition))
		}
		b.WriteString("\n")
	case SlotList:
		items := s.List(d.Key)
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(b, "## %s\n\n", d.Heading)
		for _, it := range items {
			WriteBullet(b, it)
		}
		b.WriteString("\n")
	case SlotProse:
		body := s.Prose(d.Key)
		if strings.TrimSpace(body) == "" {
			return
		}
		fmt.Fprintf(b, "## %s\n\n%s\n\n", d.Heading, strings.Trim(body, "\n"))
	}
}

// WriteBullet renders one list item as a markdown bullet. Continuation lines
// are indented two spaces so a multi-line item stays part of that item instead
// of terminating the list, and so it round-trips back to the same string.
func WriteBullet(b *strings.Builder, item string) {
	lines := strings.Split(strings.Trim(item, "\n"), "\n")
	fmt.Fprintf(b, "- %s\n", lines[0])
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(b, "  %s\n", l)
	}
}

// UnmarshalMarkdown parses a .frame.md document into a Doc. Structural problems
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
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		add(i+1, "a frame must begin with a YAML frontmatter block delimited by ---")
		return nil, fail()
	}
	start := i + 1
	end := -1
	for j := start; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
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

	doc := &Doc{
		Name:        fm.Name,
		Description: fm.Description,
		Version:     fm.Version,
		Visibility:  fm.Visibility,
		Scope:       fm.Scope,
		Maintainer:  fm.Maintainer,
		Excludes:    fm.Excludes,
	}
	for _, ref := range fm.Inherits {
		doc.Extends = append(doc.Extends, parseInherit(ref))
	}

	// --- body ---
	body := lines[end+1:]
	bodyOffset := end + 2 // 1-based file line number of body[0]

	var heads []int
	for idx, l := range body {
		if strings.HasPrefix(l, "## ") {
			heads = append(heads, idx)
		}
	}

	preEnd := len(body)
	if len(heads) > 0 {
		preEnd = heads[0]
	}
	seenH1 := false
	for idx := 0; idx < preEnd; idx++ {
		t := strings.TrimSpace(body[idx])
		if t == "" {
			continue
		}
		if !seenH1 && strings.HasPrefix(t, "# ") {
			seenH1 = true // the title heading; the name comes from frontmatter
			continue
		}
		add(bodyOffset+idx, "content before the first section heading; frame body content must sit under a recognized \"## \" section")
		break
	}

	seen := map[string]bool{}
	for h, hi := range heads {
		stop := len(body)
		if h+1 < len(heads) {
			stop = heads[h+1]
		}
		heading := strings.TrimSpace(strings.TrimPrefix(body[hi], "## "))
		d, ok := SlotByHeading(heading)
		if !ok {
			add(bodyOffset+hi, "unknown section \"## %s\"%s — recognized sections are: %s",
				heading, hint(heading), strings.Join(Headings(), ", "))
			continue
		}
		if seen[d.Key] {
			add(bodyOffset+hi, "duplicate section \"## %s\"", heading)
			continue
		}
		seen[d.Key] = true

		content := body[hi+1 : stop]
		switch d.Kind {
		case SlotTerms:
			items, starts, stray := parseBullets(content, bodyOffset+hi+1)
			for _, ln := range stray {
				add(ln, "unexpected content in \"## %s\" — every entry must be a \"- **term**: definition\" bullet", heading)
			}
			for n, it := range items {
				m := termRe.FindStringSubmatch(it)
				if m == nil {
					add(starts[n], "malformed terminology entry — expected \"- **term**: definition\"")
					continue
				}
				doc.Slots.Terminology = append(doc.Slots.Terminology, Term{
					Term: strings.TrimSpace(m[1]), Definition: strings.TrimSpace(m[2]),
				})
			}
		case SlotList:
			items, _, stray := parseBullets(content, bodyOffset+hi+1)
			for _, ln := range stray {
				add(ln, "unexpected content in \"## %s\" — every entry must be a \"- item\" bullet", heading)
			}
			doc.Slots.SetList(d.Key, items)
		case SlotProse:
			doc.Slots.SetProse(d.Key, strings.Trim(strings.Join(content, "\n"), "\n"))
		}
	}

	if len(errs) > 0 {
		return nil, fail()
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

// parseBullets splits a section body into list items. Lines indented two spaces
// continue the preceding item (the inverse of WriteBullet). base is the 1-based
// file line number of content[0]; stray holds lines that are neither.
func parseBullets(content []string, base int) (items []string, starts, stray []int) {
	var cur []string
	flush := func() {
		if cur != nil {
			items = append(items, strings.Trim(strings.Join(cur, "\n"), "\n"))
			cur = nil
		}
	}
	for i, l := range content {
		switch {
		case strings.HasPrefix(l, "- "):
			flush()
			starts = append(starts, base+i)
			cur = []string{strings.TrimPrefix(l, "- ")}
		case strings.TrimSpace(l) == "":
			if cur != nil {
				cur = append(cur, "")
			}
		case strings.HasPrefix(l, "  ") && cur != nil:
			cur = append(cur, strings.TrimPrefix(l, "  "))
		default:
			stray = append(stray, base+i)
		}
	}
	flush()
	return items, starts, stray
}

// hint suggests the closest recognized section for a rejected heading.
func hint(heading string) string {
	k := strings.ToLower(strings.TrimSpace(heading))
	for _, d := range SlotTable {
		if strings.EqualFold(d.Heading, k) {
			return fmt.Sprintf(" — did you mean \"## %s\"?", d.Heading)
		}
	}
	if h, ok := headingHints[k]; ok {
		return fmt.Sprintf(" — did you mean \"## %s\"?", h)
	}
	return ""
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
