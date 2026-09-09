package frames

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

// Built-in templates are compiled in rather than seeded into the database.
// Seeding them per org would mean N rows per org and reconciliation on every
// upgrade, and a shared "system org" is unreadable by design: rbac.Can opens
// with a cross-org deny, so no caller could see one. See ADR 0001.
//
// The definition files are our own format, deliberately not .frame.md: that is
// the external interchange shape framemd.go exists to translate, and a
// frame-spec revision moving a heading must not be able to break a starter
// document that has no reason to care.

//go:embed builtins/*.yaml
var builtinFS embed.FS

// builtinFile is the on-disk shape of one definition. There is no `prefill`
// key: built-ins carry rules and notes only, because shipping example prose
// that every author has to delete makes the blank page worse rather than
// better. Prefill is for org templates. Add the key when a built-in genuinely
// needs it.
type builtinFile struct {
	ID          string               `yaml:"id"`
	Title       string               `yaml:"title"`
	Description string               `yaml:"description"`
	FieldRules  map[string]FieldRule `yaml:"field_rules,omitempty"`
}

var builtins []Template

func init() {
	loaded, err := loadBuiltins()
	if err != nil {
		// A malformed starter must fail at startup, not at a user's first click
		// on the picker. The files are compiled in, so a failure here is a build
		// defect with no runtime recovery - the same reasoning as the
		// package-level regexp.MustCompile calls in validate.go.
		panic("frames: loading built-in templates: " + err.Error())
	}
	builtins = loaded
}

func loadBuiltins() ([]Template, error) {
	entries, err := builtinFS.ReadDir("builtins")
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		b, err := builtinFS.ReadFile("builtins/" + name)
		if err != nil {
			return nil, err
		}
		var f builtinFile
		if err := unmarshalStrict(b, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if !IsBuiltin(f.ID) {
			return nil, fmt.Errorf("%s: id %q must start with %q", name, f.ID, BuiltinPrefix)
		}
		if seen[f.ID] {
			return nil, fmt.Errorf("%s: duplicate id %q", name, f.ID)
		}
		seen[f.ID] = true
		if strings.TrimSpace(f.Title) == "" {
			return nil, fmt.Errorf("%s: title must not be empty", name)
		}
		if strings.TrimSpace(f.Description) == "" {
			return nil, fmt.Errorf("%s: description must not be empty", name)
		}
		for key, rule := range f.FieldRules {
			if _, ok := slotByKey(key); !ok {
				return nil, fmt.Errorf("%s: field_rules names unknown slot %q", name, key)
			}
			if strings.TrimSpace(rule.Note) == "" {
				return nil, fmt.Errorf("%s: field_rules[%s] has no note", name, key)
			}
		}
		out = append(out, Template{
			ID: f.ID, Title: f.Title, Description: f.Description, FieldRules: f.FieldRules,
		})
	}
	// Blank leads: it is the "no template" escape hatch and must be easy to
	// find. Everything else keeps filename order, which is alphabetical by
	// title. SliceStable so that order is preserved rather than reshuffled.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID == BlankTemplateID && out[j].ID != BlankTemplateID
	})
	return out, nil
}

// BuiltinTemplates returns the templates compiled into the binary, Blank first.
// Each call gets its own copy, rules map included: every request reads these, so
// a caller must not be able to mutate the package's copy.
func BuiltinTemplates() []Template {
	out := make([]Template, 0, len(builtins))
	for _, tmpl := range builtins {
		out = append(out, copyTemplate(tmpl))
	}
	return out
}

// BuiltinTemplate looks one up by ID, reporting whether it exists.
func BuiltinTemplate(id string) (Template, bool) {
	for _, tmpl := range builtins {
		if tmpl.ID == id {
			return copyTemplate(tmpl), true
		}
	}
	return Template{}, false
}

func copyTemplate(in Template) Template {
	out := in
	if in.FieldRules != nil {
		out.FieldRules = make(map[string]FieldRule, len(in.FieldRules))
		for key, rule := range in.FieldRules {
			out.FieldRules[key] = rule
		}
	}
	return out
}
