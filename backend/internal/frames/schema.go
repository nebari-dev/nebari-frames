// Package frames implements the Frame content schema, validation, and the
// inheritance resolver.
package frames

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExtendRef is a pinned reference to another frame.
type ExtendRef struct {
	Ref     string `yaml:"ref"`
	Version string `yaml:"version"`
}

// Doc is the parsed representation of a Frame YAML document.
//
// The content of a Frame is a single free-form markdown Body, matching Frame
// Spec v0.2: the spec requires four frontmatter fields and defines no body
// structure at all.
//
// Visibility, Scope, and Maintainer carry the Frame Spec v0.2 metadata fields
// through the canonical form so the .frame.md codec round-trips them. They are
// optional: documents published before these fields existed decode with the
// zero value and stay valid. Visibility is declared intent only - frame_grants
// remains the sole access-control authority (see rbac).
type Doc struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Version     string      `yaml:"version"`
	Visibility  string      `yaml:"visibility,omitempty"`
	Scope       string      `yaml:"scope,omitempty"`
	Maintainer  string      `yaml:"maintainer,omitempty"`
	Extends     []ExtendRef `yaml:"extends,omitempty"`
	Excludes    []string    `yaml:"excludes,omitempty"`
	// Template marks this Frame as a starting point offered by the authoring
	// UI's template picker. Registry metadata, not spec metadata: it exports
	// as x-nebari-template in the .frame.md form.
	Template bool   `yaml:"template,omitempty"`
	Body     string `yaml:"body,omitempty"`
}

// docYAML is the on-disk decode shape. It accepts the legacy `slots:` key so
// documents published before the free-form body still parse; see legacy.go.
type docYAML struct {
	Doc   `yaml:",inline"`
	Slots *legacySlots `yaml:"slots,omitempty"`
}

// docKeys is the recognized top-level key set, named in unknown-key errors.
// Keep in sync with Doc and docYAML.
var docKeys = []string{
	"name", "description", "version", "visibility", "scope",
	"maintainer", "extends", "excludes", "template", "body",
}

// unknownFieldRe extracts the offending key from a yaml.v3 KnownFields error.
// The message yaml.v3 produces names the Go type it was decoding into, which
// here is the unexported docYAML - meaningless to an API client, and this error
// reaches one unwrapped through the publish and convert endpoints.
var unknownFieldRe = regexp.MustCompile(`field (\S+) not found in type \S+`)

// Parse decodes YAML content into a Doc. Unknown keys are rejected because the
// Frame schema is fixed and not extensible. A legacy `slots:` block is folded
// into Body so versions published under the slot schema remain readable.
func Parse(content []byte) (*Doc, error) {
	var d docYAML
	dec := yaml.NewDecoder(newReader(content))
	dec.KnownFields(true) // reject unknown top-level keys: schema is fixed
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parse frame yaml: %s", parseErr(err))
	}
	// A document carrying both keys is a legacy version someone has edited. The
	// explicit body wins: `slots:` is only ever a fallback rendering of content
	// nothing writes any more, so preferring it would discard the deliberate
	// edit in favour of a reconstruction.
	if d.Slots != nil && d.Body == "" {
		d.Body = d.Slots.renderMarkdown()
	}
	return &d.Doc, nil
}

// parseErr replaces the internal decode type in a yaml.v3 unknown-key error
// with the recognized key set, since the raw message reaches API clients.
func parseErr(err error) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.ReplaceAll(msg, "unmarshal errors:\n  ", "")
	return unknownFieldRe.ReplaceAllString(msg,
		`unknown field $1 - recognized fields are: `+strings.Join(docKeys, ", "))
}

// Marshal encodes a Doc to YAML.
func Marshal(doc *Doc) ([]byte, error) { return yaml.Marshal(doc) }

func newReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
