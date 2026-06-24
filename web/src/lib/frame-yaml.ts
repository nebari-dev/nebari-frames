import { parse as parseYaml } from "yaml";
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

export const frameDocSchema = z.object({
  name: z.string(),
  description: z.string().default(""),
  version: z.string().default(""),
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
