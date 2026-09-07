import { parse as parseYaml, stringify as stringifyYaml } from "yaml";
import { z } from "zod";

// Mirrors backend/internal/frames/schema.go. Keep in sync with that file:
// it is the canonical Frame content schema.
const termSchema = z.object({ term: z.string(), definition: z.string() });

const slotsSchema = z.object({
  terminology: z.array(termSchema).optional(),
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

const extendRefSchema = z.object({ ref: z.string(), version: z.string() });

// Frame Spec v0.2 metadata. Optional in the stored document so frames published
// before these fields existed still parse; the authoring form always writes a
// visibility, and the backend defaults it on export.
export const VISIBILITY_VALUES = ["private", "internal", "shared", "public"] as const;
export const DEFAULT_VISIBILITY = "internal";

export const frameDocSchema = z.object({
  // A template's prefill is a strict subset of this schema that never sets
  // identity fields (backend/internal/frames/templates.go ParsePrefill), so
  // name must decode from an absent key the same way its sibling metadata
  // fields already do - otherwise the one shared parser used by both the
  // edit path and template seeding would reject every prefill.
  name: z.string().default(""),
  description: z.string().default(""),
  version: z.string().default(""),
  visibility: z.string().default(""),
  scope: z.string().default(""),
  maintainer: z.string().default(""),
  extends: z.array(extendRefSchema).optional(),
  excludes: z.array(z.string()).optional(),
  slots: slotsSchema.default({}),
});

export type FrameDoc = z.infer<typeof frameDocSchema>;

// Decodes and validates the YAML in FrameVersion.content.
export function parseFrameContent(content: Uint8Array | string): FrameDoc {
  const text = typeof content === "string" ? content : new TextDecoder().decode(content);
  const raw = parseYaml(text);
  return frameDocSchema.parse(raw);
}

// Builds a plain object with keys in schema.go order, omitting empty values,
// so the YAML round-trips through the backend Parse (KnownFields(true)).
function compactSlots(s: FrameDoc["slots"]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (s.terminology && s.terminology.length > 0) out.terminology = s.terminology;
  if (s.rules && s.rules.length > 0) out.rules = s.rules;
  if (s.skills && s.skills.length > 0) out.skills = s.skills;
  if (s.prompts && s.prompts.length > 0) out.prompts = s.prompts;
  for (const key of ["tool_specs", "goals", "style", "norms", "architecture", "business_process"] as const) {
    const v = s[key];
    if (typeof v === "string" && v.trim() !== "") out[key] = v;
  }
  return out;
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
  out.slots = compactSlots(doc.slots);
  return stringifyYaml(out);
}
