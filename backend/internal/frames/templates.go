package frames

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file implements Frame templates: an authoring affordance local to this
// implementation of the Frame Spec. A template is NOT a Frame and NOT a spec
// concept. It is copied once when a Frame is created and has no further
// relationship to that Frame: nothing is recorded on the Frame, and editing or
// deleting a template cannot affect anything already made from it. The
// reasoning, including why templates are not modelled as Frames, is in
// docs/adr/0001-frame-templates-are-not-frames.md.

// Requirement is how strongly a template wants a slot filled. It is stricter
// than the Frame schema, which requires no slot at all: an org uses it to hold
// its own Frames to a house standard the spec does not impose.
type Requirement int

const (
	// RequirementOptional is the default for any slot a template omits.
	RequirementOptional Requirement = iota
	// RequirementRecommended is never enforced. It tells the authoring surfaces
	// to offer the section, not to insist on it.
	RequirementRecommended
	// RequirementRequired refuses the create-from-template publish while the
	// slot is empty.
	RequirementRequired
)

func (r Requirement) String() string {
	switch r {
	case RequirementRecommended:
		return "recommended"
	case RequirementRequired:
		return "required"
	default:
		return "optional"
	}
}

// UnmarshalYAML accepts the level names the built-in template files use.
// An unknown value is an error rather than a silent fall back to optional: a
// typo in a level would otherwise quietly drop a requirement its author meant
// to impose, and the whole point of the field is to be a control.
func (r *Requirement) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "optional":
		*r = RequirementOptional
	case "recommended":
		*r = RequirementRecommended
	case "required":
		*r = RequirementRequired
	default:
		return fmt.Errorf("unknown requirement level %q: want optional, recommended, or required", s)
	}
	return nil
}

// MarshalYAML writes the level by name, so a definition file round-trips as
// something a person would want to edit.
func (r Requirement) MarshalYAML() (any, error) { return r.String(), nil }

// FieldRule is a template's expectation for one slot: how strongly the slot is
// wanted, and what belongs in it. Note carries the template author's own
// guidance, which is more use than the generic per-slot hints the surfaces
// would otherwise show, and is the question an MCP client asks when it runs an
// authoring interview (see issue #61).
type FieldRule struct {
	Level Requirement `yaml:"level"`
	Note  string      `yaml:"note,omitempty"`
}

// Prefill is the content a template seeds into a new Frame. It is a strict
// subset of Doc on purpose: a template seeds content and must never dictate a
// Frame's identity, so name, description, and version are not representable.
type Prefill struct {
	Slots   Slots
	Extends []ExtendRef
}

// prefillExtendRef is a narrower view of ExtendRef used only for marshalling:
// prefill encoding omits version to keep templates' extend specs minimal.
// Decoding through Parse preserves version normally.
type prefillExtendRef struct {
	Ref string `yaml:"ref"`
}

// prefillDoc is the serialized shape of a Prefill. Decoding goes through Parse,
// which yields a full Doc and so catches identity fields a template must not
// set; encoding uses this narrower type so those keys are omitted entirely
// rather than emitted empty. The asymmetry is deliberate: it means a blob this
// package writes is always one it will accept back.
type prefillDoc struct {
	Extends []prefillExtendRef `yaml:"extends,omitempty"`
	Slots   Slots              `yaml:"slots"`
}

// Template is one starter definition. Built-ins are compiled in; org templates
// are rows in frame_templates. Nothing downstream distinguishes them beyond the
// ID prefix, which only the RPC layer reads (to refuse mutations on a built-in).
type Template struct {
	ID          string
	Title       string
	Description string
	Prefill     Prefill
	FieldRules  map[string]FieldRule // slot key from SlotTable -> rule
}

// BuiltinPrefix marks the IDs of templates compiled into the binary. They have
// no org and no row, so they cannot be edited or deleted.
const BuiltinPrefix = "builtin:"

// BlankTemplateID is the "start from nothing" option. It is a real template with
// no prefill and no rules rather than a special case in the picker, which keeps
// the seed path uniform: starting blank is not a branch.
const BlankTemplateID = BuiltinPrefix + "blank"

// IsBuiltin reports whether id names a template compiled into the binary.
func IsBuiltin(id string) bool { return strings.HasPrefix(id, BuiltinPrefix) }

// slotByKey resolves a slot key to its descriptor. SlotTable is ten entries and
// this runs at load and validation time only, so a linear scan is the right
// cost; SlotByHeading next door does the same.
func slotByKey(key string) (SlotDescriptor, bool) {
	for _, d := range SlotTable {
		if d.Key == key {
			return d, true
		}
	}
	return SlotDescriptor{}, false
}

// slotHasContent reports whether a slot carries anything an author would call
// content. Emptiness is defined here rather than trusting the frontend's
// sectionHasContent: this is what the publish path checks against, and a client
// computing it differently must not be able to slip a blank required section
// past the check.
func slotHasContent(d SlotDescriptor, slots *Slots) bool {
	switch d.Kind {
	case SlotTerms:
		// A term with no definition is not content: it is the start of one.
		for _, term := range slots.Terminology {
			if strings.TrimSpace(term.Term) != "" && strings.TrimSpace(term.Definition) != "" {
				return true
			}
		}
		return false
	case SlotList:
		for _, item := range slots.List(d.Key) {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
		return false
	case SlotProse:
		return strings.TrimSpace(slots.Prose(d.Key)) != ""
	}
	return false
}

// Check reports the slots this template requires that doc leaves empty.
//
// Pure and side-effect free: the publish path merges these with the schema's own
// violations so an author sees every problem at once, instead of fixing the
// schema's and only then discovering the template's.
//
// Only RequirementRequired produces a violation. Recommended is guidance for the
// authoring surfaces and is deliberately not enforced, and this runs on create
// only - see the ADR on why requirements are an authoring aid rather than
// ongoing governance.
func (t *Template) Check(doc *Doc) []FieldError {
	if t == nil || len(t.FieldRules) == 0 {
		return nil
	}
	var errs []FieldError
	// Walk SlotTable rather than ranging the map, so violations come out in
	// schema order: that matches how Validate reports and how the form lays its
	// sections out, and map order would make the output nondeterministic.
	for _, d := range SlotTable {
		rule, ok := t.FieldRules[d.Key]
		if !ok || rule.Level != RequirementRequired {
			continue
		}
		if slotHasContent(d, &doc.Slots) {
			continue
		}
		msg := fmt.Sprintf("required by the %q template", t.Title)
		if rule.Note != "" {
			msg += ": " + rule.Note
		}
		errs = append(errs, FieldError{Path: "slots." + d.Key, Message: msg})
	}
	return errs
}

// ParsePrefill decodes a prefill blob: the canonical YAML subset carrying slots
// and suggested parents.
//
// It rejects every identity field. Rejecting beats ignoring: a template author
// who set `name` meant something by it, and silently dropping it would surprise
// them the first time a Frame came out with a different name. Excludes are
// refused for the same reason - an exclusion is a statement about one Frame's
// relationship to another, not something a starter can decide in advance.
func ParsePrefill(content []byte) (Prefill, error) {
	doc, err := Parse(content)
	if err != nil {
		return Prefill{}, err
	}
	for _, f := range []struct {
		field string
		set   bool
	}{
		{"name", doc.Name != ""},
		{"description", doc.Description != ""},
		{"version", doc.Version != ""},
		{"visibility", doc.Visibility != ""},
		{"scope", doc.Scope != ""},
		{"maintainer", doc.Maintainer != ""},
	} {
		if f.set {
			return Prefill{}, fmt.Errorf(
				"prefill must not set %s: a template seeds content, not identity", f.field)
		}
	}
	if len(doc.Excludes) > 0 {
		return Prefill{}, fmt.Errorf(
			"prefill must not set excludes: an exclusion belongs to a Frame, not to the template it started from")
	}
	return Prefill{Slots: doc.Slots, Extends: doc.Extends}, nil
}

// MarshalPrefill encodes a Prefill to the canonical YAML subset the API carries
// and the CLI scaffold writes.
func MarshalPrefill(p Prefill) ([]byte, error) {
	extends := make([]prefillExtendRef, len(p.Extends))
	for i, e := range p.Extends {
		extends[i] = prefillExtendRef{Ref: e.Ref}
	}
	return yaml.Marshal(prefillDoc{Extends: extends, Slots: p.Slots})
}

// unmarshalStrict decodes YAML into v, rejecting unknown keys. Template
// definition files are ours and their key set is fixed, so a typo must fail
// loudly rather than leave a rule silently unset.
func unmarshalStrict(b []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(v)
}
