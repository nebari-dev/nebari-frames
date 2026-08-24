import { parse as parseYaml, stringify as stringifyYaml } from "yaml";
import { z } from "zod";

// Mirrors backend/internal/frames/schema.go. Keep in sync with that file:
// it is the canonical Frame content schema. The content of a Frame is a single
// free-form markdown body, matching Frame Spec v0.2.

const extendRefSchema = z.object({ ref: z.string(), version: z.string() });

// Frame Spec v0.2 metadata. Optional in the stored document so frames published
// before these fields existed still parse; the authoring form always writes a
// visibility, and the backend defaults it on export.
export const VISIBILITY_VALUES = ["private", "internal", "shared", "public"] as const;
export const DEFAULT_VISIBILITY = "internal";

// The retired ten-slot content schema, accepted read-only so versions
// published before the free-form body still render. Mirrors legacy.go.
const legacyTermSchema = z.object({ term: z.string(), definition: z.string() });
const legacySlotsSchema = z.object({
  terminology: z.array(legacyTermSchema).optional(),
  rules: z.array(z.string()).optional(),
  skills: z.array(z.string()).optional(),
  prompts: z.array(z.string()).optional(),
  tool_specs: z.string().optional(),
  goals: z.string().optional(),
  style: z.string().optional(),
  norms: z.string().optional(),
  architecture: z.string().optional(),
  business_process: z.string().optional(),
});

export const frameDocSchema = z.object({
  name: z.string(),
  description: z.string().default(""),
  version: z.string().default(""),
  visibility: z.string().default(""),
  scope: z.string().default(""),
  maintainer: z.string().default(""),
  extends: z.array(extendRefSchema).optional(),
  excludes: z.array(z.string()).optional(),
  // Marks the frame as a starting point offered by the template picker.
  template: z.boolean().default(false),
  body: z.string().default(""),
  // Legacy documents only; folded into body by parseFrameContent and never
  // serialized back out.
  slots: legacySlotsSchema.optional(),
});

export type FrameDoc = Omit<z.infer<typeof frameDocSchema>, "slots">;
type LegacySlots = z.infer<typeof legacySlotsSchema>;

// legacySlotsToMarkdown renders retired slot content as the markdown sections
// the old .frame.md codec emitted.
//
// This is a byte-for-byte mirror of renderMarkdown in
// backend/internal/frames/legacy.go, down to the string building, because its
// output is not display-only: FrameDetailPage feeds it back through
// serializeFrameDoc when a legacy version is restored, so whatever this
// produces becomes canonical stored content. A divergence is a silent content
// rewrite, not a rendering difference.
//
// The two sides are pinned to one shared fixture - testdata/legacy-slots -
// which carries the cases where a naive port drifts: a multi-line list item
// (Go indents continuation lines two spaces; without that, CommonMark lazy
// continuation flattens nested markup into sibling items) and prose padded
// with blank lines (Go trims newlines only, not all whitespace).
function legacySlotsToMarkdown(s: LegacySlots): string {
  let out = "";

  // Mirrors writeLegacyBullet: continuation lines are indented two spaces so a
  // multi-line item stays part of that item, and a blank line stays blank.
  const writeBullet = (item: string) => {
    const lines = item.replace(/^\n+|\n+$/g, "").split("\n");
    out += `- ${lines[0]}\n`;
    for (const l of lines.slice(1)) {
      out += l.trim() === "" ? "\n" : `  ${l}\n`;
    }
  };

  if (s.terminology && s.terminology.length > 0) {
    out += "## Terminology\n\n";
    for (const t of s.terminology) writeBullet(`**${t.term}**: ${t.definition}`);
    out += "\n";
  }

  const lists: [string, string[] | undefined][] = [
    ["Rules", s.rules],
    ["Skills", s.skills],
    ["Prompts", s.prompts],
  ];
  for (const [heading, items] of lists) {
    if (!items || items.length === 0) continue;
    out += `## ${heading}\n\n`;
    for (const it of items) writeBullet(it);
    out += "\n";
  }

  const prose: [string, string | undefined][] = [
    ["Tool Specifications", s.tool_specs],
    ["Goals", s.goals],
    ["Style", s.style],
    ["Norms", s.norms],
    ["Architecture", s.architecture],
    ["Business Process", s.business_process],
  ];
  for (const [heading, text] of prose) {
    if (!text || text.trim() === "") continue;
    // Newlines only, matching Go's strings.Trim(body, "\n"): trimming all
    // whitespace would strip indentation that is meaningful in markdown.
    out += `## ${heading}\n\n${text.replace(/^\n+|\n+$/g, "")}\n\n`;
  }

  return out.replace(/\n+$/, "");
}

// Decodes and validates the YAML in FrameVersion.content. A legacy `slots:`
// block is folded into the body so old versions remain readable.
export function parseFrameContent(content: Uint8Array | string): FrameDoc {
  const text = typeof content === "string" ? content : new TextDecoder().decode(content);
  const raw = parseYaml(text);
  const { slots, ...doc } = frameDocSchema.parse(raw);
  if (slots && doc.body === "") {
    doc.body = legacySlotsToMarkdown(slots);
  }
  return doc;
}

export function serializeFrameDoc(doc: FrameDoc): string {
  const out: Record<string, unknown> = {
    name: doc.name,
    description: doc.description,
    version: doc.version,
  };
  for (const key of ["visibility", "scope", "maintainer"] as const) {
    const v = doc[key];
    if (typeof v === "string" && v.trim() !== "") out[key] = v;
  }
  if (doc.extends && doc.extends.length > 0) out.extends = doc.extends;
  if (doc.excludes && doc.excludes.length > 0) out.excludes = doc.excludes;
  if (doc.template) out.template = true;
  if (doc.body.trim() !== "") out.body = doc.body;
  return stringifyYaml(out);
}
