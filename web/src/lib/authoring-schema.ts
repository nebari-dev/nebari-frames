import { z } from "zod";
import type { FrameDoc } from "./frame-yaml";

const NAME_RE = /^[a-z0-9][a-z0-9-]{0,63}$/;

const termSchema = z.object({
  term: z.string().trim().min(1, "must not be empty"),
  definition: z.string().trim().min(1, "must not be empty"),
});

const nonEmptyList = z.array(z.string().trim().min(1, "must not be empty")).optional();

const slotsSchema = z.object({
  terminology: z
    .array(termSchema)
    .optional()
    .refine(
      (terms) => !terms || new Set(terms.map((t) => t.term)).size === terms.length,
      { message: "duplicate term within slot" },
    ),
  rules: nonEmptyList,
  skills: nonEmptyList,
  prompts: nonEmptyList,
  tool_specs: z.string().optional(),
  goals: z.string().optional(),
  style: z.string().optional(),
  norms: z.string().optional(),
  architecture: z.string().optional(),
  business_process: z.string().optional(),
});

const extendSchema = z.object({
  ref: z.string().refine((r) => r.includes("/"), { message: "must be org_slug/frame_name" }),
  version: z.string().trim().min(1, "must be pinned to a version"),
});

export const authoringSchema = z.object({
  name: z.string().regex(NAME_RE, "must match [a-z0-9][a-z0-9-]{0,63}"),
  description: z.string().min(1, "must not be empty").max(280, "must be at most 280 characters"),
  version: z.string().trim().min(1, "must not be empty"),
  extends: z.array(extendSchema).optional(),
  excludes: z.array(z.string().trim().min(1)).optional(),
  slots: slotsSchema,
}) satisfies z.ZodType<FrameDoc>;

// Resolver schema for the form: includes the version-scoped changelog. Used so
// zodResolver does not strip `changelog` from the submitted values (z.object
// drops unknown keys by default).
export const authoringFormSchema = authoringSchema.extend({
  changelog: z.string().optional(),
});

export function emptyFrameDoc(): FrameDoc {
  return { name: "", description: "", version: "", slots: {} };
}

export function suggestNextVersion(current: string): string {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(current.trim());
  if (!m) return current;
  return `${m[1]}.${m[2]}.${Number(m[3]) + 1}`;
}
