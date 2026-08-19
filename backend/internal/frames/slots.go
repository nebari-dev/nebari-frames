package frames

// SlotKind classifies the shape of a slot's content, which determines how the
// slot is rendered to and parsed back from the markdown (.frame.md) form.
type SlotKind int

const (
	// SlotTerms is a []Term slot rendered as "- **term**: definition" bullets.
	SlotTerms SlotKind = iota
	// SlotList is a []string slot rendered as plain "- item" bullets.
	SlotList
	// SlotProse is a string slot rendered as a raw markdown body.
	SlotProse
)

// SlotDescriptor names one slot of the fixed schema.
type SlotDescriptor struct {
	Key     string // key under `slots:` in the canonical YAML
	Heading string // canonical "## " heading in the markdown form
	Kind    SlotKind
}

// SlotTable is the canonical slot list in schema order. It is the single source
// of truth for slot keys, markdown headings, and content shape: the .frame.md
// codec and the MCP markdown composer both read it, so a slot is added or
// renamed in exactly one place.
var SlotTable = []SlotDescriptor{
	{Key: "terminology", Heading: "Terminology", Kind: SlotTerms},
	{Key: "rules", Heading: "Rules", Kind: SlotList},
	{Key: "skills", Heading: "Skills", Kind: SlotList},
	{Key: "prompts", Heading: "Prompts", Kind: SlotList},
	{Key: "tool_specs", Heading: "Tool Specifications", Kind: SlotProse},
	{Key: "goals", Heading: "Goals", Kind: SlotProse},
	{Key: "style", Heading: "Style", Kind: SlotProse},
	{Key: "norms", Heading: "Norms", Kind: SlotProse},
	{Key: "architecture", Heading: "Architecture", Kind: SlotProse},
	{Key: "business_process", Heading: "Business Process", Kind: SlotProse},
}

// SlotByHeading resolves a markdown "## " heading to its descriptor.
func SlotByHeading(heading string) (SlotDescriptor, bool) {
	for _, d := range SlotTable {
		if d.Heading == heading {
			return d, true
		}
	}
	return SlotDescriptor{}, false
}

// Headings returns every recognized section heading, in schema order.
func Headings() []string {
	out := make([]string, len(SlotTable))
	for i, d := range SlotTable {
		out[i] = d.Heading
	}
	return out
}

// listPtr returns the addressable []string field for a SlotList key.
func (s *Slots) listPtr(key string) *[]string {
	switch key {
	case "rules":
		return &s.Rules
	case "skills":
		return &s.Skills
	case "prompts":
		return &s.Prompts
	}
	return nil
}

// prosePtr returns the addressable string field for a SlotProse key.
func (s *Slots) prosePtr(key string) *string {
	switch key {
	case "tool_specs":
		return &s.ToolSpecs
	case "goals":
		return &s.Goals
	case "style":
		return &s.Style
	case "norms":
		return &s.Norms
	case "architecture":
		return &s.Architecture
	case "business_process":
		return &s.BusinessProcess
	}
	return nil
}

// List reads a SlotList slot by key; nil for any other key.
func (s *Slots) List(key string) []string {
	if p := s.listPtr(key); p != nil {
		return *p
	}
	return nil
}

// SetList writes a SlotList slot by key; a no-op for any other key.
func (s *Slots) SetList(key string, items []string) {
	if p := s.listPtr(key); p != nil {
		*p = items
	}
}

// Prose reads a SlotProse slot by key; "" for any other key.
func (s *Slots) Prose(key string) string {
	if p := s.prosePtr(key); p != nil {
		return *p
	}
	return ""
}

// SetProse writes a SlotProse slot by key; a no-op for any other key.
func (s *Slots) SetProse(key, body string) {
	if p := s.prosePtr(key); p != nil {
		*p = body
	}
}
