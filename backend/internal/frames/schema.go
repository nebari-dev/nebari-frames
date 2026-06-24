// Package frames implements the Frame content schema, validation, and the
// inheritance resolver.
package frames

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ExtendRef is a pinned reference to another frame.
type ExtendRef struct {
	Ref     string `yaml:"ref"`
	Version string `yaml:"version"`
}

// Term is a single vocabulary entry in the terminology slot.
type Term struct {
	Term       string `yaml:"term"`
	Definition string `yaml:"definition"`
}

// Slots holds the ten content slots defined by the Frame schema.
type Slots struct {
	Terminology     []Term   `yaml:"terminology,omitempty"`
	Rules           []string `yaml:"rules,omitempty"`
	Skills          []string `yaml:"skills,omitempty"`
	Prompts         []string `yaml:"prompts,omitempty"`
	ToolSpecs       string   `yaml:"tool_specs,omitempty"`
	Goals           string   `yaml:"goals,omitempty"`
	Style           string   `yaml:"style,omitempty"`
	Norms           string   `yaml:"norms,omitempty"`
	Architecture    string   `yaml:"architecture,omitempty"`
	BusinessProcess string   `yaml:"business_process,omitempty"`
}

// Doc is the parsed representation of a Frame YAML document.
type Doc struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Version     string      `yaml:"version"`
	Extends     []ExtendRef `yaml:"extends,omitempty"`
	Excludes    []string    `yaml:"excludes,omitempty"`
	Slots       Slots       `yaml:"slots"`
}

// Parse decodes YAML content into a Doc. Unknown keys are rejected because the
// Frame schema is fixed and not extensible.
func Parse(content []byte) (*Doc, error) {
	var d Doc
	dec := yaml.NewDecoder(newReader(content))
	dec.KnownFields(true) // reject unknown top-level/slot keys: schema is fixed
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parse frame yaml: %w", err)
	}
	return &d, nil
}

// Marshal encodes a Doc to YAML.
func Marshal(doc *Doc) ([]byte, error) { return yaml.Marshal(doc) }

func newReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
