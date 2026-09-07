package frames

import (
	"regexp"
	"strings"
	"testing"
)

func TestRequirementUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    Requirement
		wantErr string
	}{
		{name: "optional", yaml: "level: optional", want: RequirementOptional},
		{name: "recommended", yaml: "level: recommended", want: RequirementRecommended},
		{name: "required", yaml: "level: required", want: RequirementRequired},
		{name: "case and space insensitive", yaml: "level: \"  Required \"", want: RequirementRequired},
		{name: "unknown level is an error", yaml: "level: mandatory", wantErr: "unknown requirement level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FieldRule
			err := unmarshalStrict([]byte(tt.yaml), &got)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Level != tt.want {
				t.Errorf("level = %v, want %v", got.Level, tt.want)
			}
		})
	}
}

func TestTemplateCheck(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		rules     map[string]FieldRule
		slots     Slots
		wantPaths []string
		wantMsg   string
	}{
		{
			name:      "required prose slot empty is a violation",
			rules:     map[string]FieldRule{"style": {Level: RequirementRequired}},
			slots:     Slots{},
			wantPaths: []string{"slots.style"},
		},
		{
			name:      "required prose slot of only whitespace is empty",
			rules:     map[string]FieldRule{"style": {Level: RequirementRequired}},
			slots:     Slots{Style: "  \n\t "},
			wantPaths: []string{"slots.style"},
		},
		{
			name:      "required prose slot filled passes",
			rules:     map[string]FieldRule{"style": {Level: RequirementRequired}},
			slots:     Slots{Style: "Plain, direct sentences."},
			wantPaths: nil,
		},
		{
			name:      "required list slot of only blank entries is empty",
			rules:     map[string]FieldRule{"rules": {Level: RequirementRequired}},
			slots:     Slots{Rules: []string{"", "   "}},
			wantPaths: []string{"slots.rules"},
		},
		{
			name:      "required list slot with one real entry passes",
			rules:     map[string]FieldRule{"rules": {Level: RequirementRequired}},
			slots:     Slots{Rules: []string{"", "Never ship on a Friday."}},
			wantPaths: nil,
		},
		{
			name:      "required terms slot needs a definition too",
			rules:     map[string]FieldRule{"terminology": {Level: RequirementRequired}},
			slots:     Slots{Terminology: []Term{{Term: "Frame", Definition: "  "}}},
			wantPaths: []string{"slots.terminology"},
		},
		{
			name:      "required terms slot with a complete entry passes",
			rules:     map[string]FieldRule{"terminology": {Level: RequirementRequired}},
			slots:     Slots{Terminology: []Term{{Term: "Frame", Definition: "A scoped context artifact."}}},
			wantPaths: nil,
		},
		{
			name:      "recommended is never enforced",
			rules:     map[string]FieldRule{"style": {Level: RequirementRecommended}},
			slots:     Slots{},
			wantPaths: nil,
		},
		{
			name:      "optional is never enforced",
			rules:     map[string]FieldRule{"style": {Level: RequirementOptional}},
			slots:     Slots{},
			wantPaths: nil,
		},
		{
			name:      "a slot with no rule is never enforced",
			rules:     map[string]FieldRule{},
			slots:     Slots{},
			wantPaths: nil,
		},
		{
			name: "violations come out in SlotTable order, not map order",
			rules: map[string]FieldRule{
				"business_process": {Level: RequirementRequired},
				"terminology":      {Level: RequirementRequired},
				"style":            {Level: RequirementRequired},
			},
			slots:     Slots{},
			wantPaths: []string{"slots.terminology", "slots.style", "slots.business_process"},
		},
		{
			name:      "the rule's note is carried into the message",
			title:     "Domain Vocabulary",
			rules:     map[string]FieldRule{"terminology": {Level: RequirementRequired, Note: "One entry per term of art."}},
			slots:     Slots{},
			wantPaths: []string{"slots.terminology"},
			wantMsg:   `required by the "Domain Vocabulary" template: One entry per term of art.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{ID: "builtin:test", Title: tt.title, FieldRules: tt.rules}
			got := tmpl.Check(&Doc{Slots: tt.slots})
			if len(got) != len(tt.wantPaths) {
				t.Fatalf("got %d violations %v, want %d %v", len(got), got, len(tt.wantPaths), tt.wantPaths)
			}
			for i, want := range tt.wantPaths {
				if got[i].Path != want {
					t.Errorf("violation %d path = %q, want %q", i, got[i].Path, want)
				}
			}
			if tt.wantMsg != "" && got[0].Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", got[0].Message, tt.wantMsg)
			}
		})
	}
}

func TestTemplateCheckNilTemplateIsNoOp(t *testing.T) {
	var tmpl *Template
	if got := tmpl.Check(&Doc{}); got != nil {
		t.Errorf("nil template produced %v, want nil", got)
	}
}

func TestParsePrefill(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  string
		wantTerm string
		wantExt  int
	}{
		{
			name:     "slots and extends are kept",
			yaml:     "extends:\n  - ref: acme/base\n    version: 1.0.0\nslots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n",
			wantTerm: "Frame",
			wantExt:  1,
		},
		{name: "name is rejected", yaml: "name: brand-voice\nslots: {}\n", wantErr: "must not set name"},
		{name: "version is rejected", yaml: "version: 1.0.0\nslots: {}\n", wantErr: "must not set version"},
		{name: "description is rejected", yaml: "description: hello\nslots: {}\n", wantErr: "must not set description"},
		{name: "visibility is rejected", yaml: "visibility: internal\nslots: {}\n", wantErr: "must not set visibility"},
		{name: "scope is rejected", yaml: "scope: team\nslots: {}\n", wantErr: "must not set scope"},
		{name: "maintainer is rejected", yaml: "maintainer: a@b.c\nslots: {}\n", wantErr: "must not set maintainer"},
		{name: "excludes is rejected", yaml: "excludes:\n  - acme/other\nslots: {}\n", wantErr: "must not set excludes"},
		{name: "an unknown key is rejected", yaml: "colour: red\nslots: {}\n", wantErr: "parse frame yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrefill([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Extends) != tt.wantExt {
				t.Errorf("extends = %d, want %d", len(got.Extends), tt.wantExt)
			}
			if tt.wantTerm != "" {
				if len(got.Slots.Terminology) != 1 || got.Slots.Terminology[0].Term != tt.wantTerm {
					t.Errorf("terminology = %v, want one entry %q", got.Slots.Terminology, tt.wantTerm)
				}
			}
		})
	}
}

func TestPrefillRoundTripsWithoutIdentityKeys(t *testing.T) {
	in := Prefill{
		Slots:   Slots{Style: "Plain sentences.", Rules: []string{"No em dashes."}},
		Extends: []ExtendRef{{Ref: "acme/base", Version: "1.0.0"}},
	}
	b, err := MarshalPrefill(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Identity keys must not be emitted at all: ParsePrefill rejects them, so a
	// blob this function produced has to survive being read back.
	//
	// Anchored to the start of a line. A bare substring check would also match
	// the nested `extends[].version` key, which is legitimate and must be kept:
	// that field is the pinned parent version, and Validate rejects an extends
	// ref without one.
	for _, key := range []string{"name:", "version:", "description:", "visibility:", "scope:", "maintainer:", "excludes:"} {
		if regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key)).Match(b) {
			t.Errorf("marshalled prefill emits top-level %q:\n%s", key, b)
		}
	}
	out, err := ParsePrefill(b)
	if err != nil {
		t.Fatalf("round trip rejected its own output: %v\n%s", err, b)
	}
	if out.Slots.Style != in.Slots.Style {
		t.Errorf("round trip lost slot content: %+v", out.Slots)
	}
	// The pin specifically: dropping the version would leave every Frame seeded
	// from a template with suggested parents unpublishable.
	if len(out.Extends) != 1 || out.Extends[0].Ref != "acme/base" || out.Extends[0].Version != "1.0.0" {
		t.Errorf("round trip lost the pinned parent: %+v", out.Extends)
	}
}

// TestSlotHasContentCoversEverySlot is the reflective guard the spec calls for,
// mirroring the ones in backend/internal/mcp/resources_test.go. It walks
// SlotTable so that adding a slot without teaching slotHasContent about its kind
// fails here. Without it, a new slot kind would fall through the switch, always
// read as empty, and make a `required` rule on it permanently unsatisfiable -
// silently, from the author's point of view.
func TestSlotHasContentCoversEverySlot(t *testing.T) {
	for _, d := range SlotTable {
		t.Run(d.Key, func(t *testing.T) {
			// Empty must read as empty.
			var empty Slots
			if slotHasContent(d, &empty) {
				t.Errorf("an empty %s slot reads as having content", d.Key)
			}
			// A populated instance of this slot's kind must read as content.
			filled := &Slots{}
			switch d.Kind {
			case SlotTerms:
				filled.Terminology = []Term{{Term: "Frame", Definition: "A scoped context artifact."}}
			case SlotList:
				filled.SetList(d.Key, []string{"An entry."})
			case SlotProse:
				filled.SetProse(d.Key, "Some prose.")
			default:
				t.Fatalf("slot %s has kind %v, which slotHasContent does not handle; add a case", d.Key, d.Kind)
			}
			if !slotHasContent(d, filled) {
				t.Errorf("a populated %s slot reads as empty; slotHasContent is missing a case for kind %v", d.Key, d.Kind)
			}
		})
	}
}

func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{id: "builtin:blank", want: true},
		{id: "builtin:domain-vocabulary", want: true},
		{id: "01J000000000000000000000AB", want: false},
		{id: "", want: false},
		{id: "notbuiltin:x", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsBuiltin(tt.id); got != tt.want {
				t.Errorf("IsBuiltin(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestSlotByKey(t *testing.T) {
	tests := []struct {
		key     string
		wantOK  bool
		wantKey string
	}{
		{key: "terminology", wantOK: true, wantKey: "terminology"},
		{key: "style", wantOK: true, wantKey: "style"},
		{key: "business_process", wantOK: true, wantKey: "business_process"},
		{key: "unknown", wantOK: false},
		{key: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := slotByKey(tt.key)
			if ok != tt.wantOK {
				t.Errorf("slotByKey(%q) ok = %v, want %v", tt.key, ok, tt.wantOK)
			}
			if ok && got.Key != tt.wantKey {
				t.Errorf("slotByKey(%q) key = %q, want %q", tt.key, got.Key, tt.wantKey)
			}
		})
	}
}
