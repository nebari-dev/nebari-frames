package frames

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinTemplatesLoad(t *testing.T) {
	got := BuiltinTemplates()
	if len(got) < 6 {
		t.Fatalf("loaded %d built-ins, want at least 6", len(got))
	}
	// Blank leads the picker: it is the escape hatch, and burying it among the
	// starters makes it hard to find.
	if got[0].ID != BlankTemplateID {
		t.Errorf("first built-in is %q, want %q", got[0].ID, BlankTemplateID)
	}
	if len(got[0].FieldRules) != 0 {
		t.Errorf("blank carries %d rules, want none", len(got[0].FieldRules))
	}
}

func TestBuiltinTemplatesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tmpl := range BuiltinTemplates() {
		t.Run(tmpl.ID, func(t *testing.T) {
			if !IsBuiltin(tmpl.ID) {
				t.Errorf("id %q does not start with %q", tmpl.ID, BuiltinPrefix)
			}
			if seen[tmpl.ID] {
				t.Errorf("duplicate id %q", tmpl.ID)
			}
			seen[tmpl.ID] = true
			if tmpl.Title == "" {
				t.Error("title is empty")
			}
			if tmpl.Description == "" {
				t.Error("description is empty")
			}
			// A built-in carrying prefill would mean shipping placeholder prose
			// authors have to delete. See the note in builtins.go.
			if len(tmpl.Prefill.Slots.Rules) > 0 || tmpl.Prefill.Slots.Style != "" ||
				len(tmpl.Prefill.Slots.Terminology) > 0 || len(tmpl.Prefill.Extends) > 0 {
				t.Error("built-in carries prefill, which built-ins deliberately do not")
			}
			for key, rule := range tmpl.FieldRules {
				if _, ok := slotByKey(key); !ok {
					t.Errorf("rule names unknown slot %q", key)
				}
				// A rule with no note is a missed opportunity: the note is what
				// the seeded form shows and what an MCP interview asks.
				if rule.Note == "" {
					t.Errorf("rule for %q has no note", key)
				}
			}
		})
	}
}

func TestBuiltinTemplateLookup(t *testing.T) {
	tests := []struct {
		id    string
		found bool
	}{
		{id: BlankTemplateID, found: true},
		{id: "builtin:domain-vocabulary", found: true},
		{id: "builtin:nope", found: false},
		{id: "01J000000000000000000000AB", found: false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got, ok := BuiltinTemplate(tt.id)
			if ok != tt.found {
				t.Fatalf("found = %v, want %v", ok, tt.found)
			}
			if ok && got.ID != tt.id {
				t.Errorf("returned id %q, want %q", got.ID, tt.id)
			}
		})
	}
}

func TestBuiltinTemplatesAreCopiedPerCall(t *testing.T) {
	// Every request reads these, so a caller must not be able to mutate the
	// package's own copy.
	first := BuiltinTemplates()
	target := ""
	for _, tmpl := range first {
		if len(tmpl.FieldRules) > 0 {
			target = tmpl.ID
			for key := range tmpl.FieldRules {
				tmpl.FieldRules[key] = FieldRule{Level: RequirementOptional, Note: "clobbered"}
			}
			break
		}
	}
	if target == "" {
		t.Fatal("no built-in has rules, so this test proves nothing")
	}
	again, ok := BuiltinTemplate(target)
	if !ok {
		t.Fatalf("%s vanished", target)
	}
	for key, rule := range again.FieldRules {
		if rule.Note == "clobbered" {
			t.Fatalf("mutating a returned template changed the package copy at %q", key)
		}
	}
}

// Built-ins are the other door a starter can arrive through, and they skip the
// write RPCs entirely - so the content rules a prefill is held to
// (validateTemplateInput) would not apply to one shipped in a builtin file.
// That is safe only because builtinFile has no prefill key: built-ins carry
// rules and notes, never content. This asserts that premise structurally
// rather than trusting a comment, in the same idiom internal/mcp uses to pin
// its input structs to the slot table.
//
// If this fails, you added a prefill to built-in templates. Run contentErrors
// over it in loadBuiltins before appending, so a malformed starter fails at
// startup rather than at an author's first click
// (docs/adr/0001-frame-templates-are-not-frames.md).
func TestBuiltinFileCarriesNoPrefill(t *testing.T) {
	ft := reflect.TypeOf(builtinFile{})
	for i := range ft.NumField() {
		field := ft.Field(i)
		tag, name := field.Tag.Get("yaml"), strings.ToLower(field.Name)
		if strings.Contains(tag, "prefill") || strings.Contains(name, "prefill") {
			t.Fatalf("builtinFile now has a prefill field (%s); loadBuiltins must validate it", field.Name)
		}
	}
}
