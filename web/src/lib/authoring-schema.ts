import { z } from "zod";
import { type FrameDoc, VISIBILITY_VALUES, DEFAULT_VISIBILITY } from "./frame-yaml";

const NAME_RE = /^[a-z0-9][a-z0-9-]{0,63}$/;

const extendSchema = z.object({
  ref: z.string().refine((r) => r.includes("/"), { message: "must be org_slug/frame_name" }),
  version: z.string().trim().min(1, "must be pinned to a version"),
});

export const authoringSchema = z.object({
  name: z.string().regex(NAME_RE, "must match [a-z0-9][a-z0-9-]{0,63}"),
  description: z.string().min(1, "must not be empty").max(280, "must be at most 280 characters"),
  version: z.string().trim().min(1, "must not be empty"),
  visibility: z
    .string()
    .refine((v) => (VISIBILITY_VALUES as readonly string[]).includes(v), {
      message: `must be one of ${VISIBILITY_VALUES.join(", ")}`,
    }),
  // Always present in form state (emptyFrameDoc and docToForm seed them), so
  // these stay plain strings rather than defaults, which would make the
  // schema's input type diverge from FrameDoc.
  scope: z.string(),
  maintainer: z.string(),
  extends: z.array(extendSchema).optional(),
  excludes: z.array(z.string().trim().min(1)).optional(),
  template: z.boolean(),
  // The body is free-form markdown; Frame Spec v0.2 imposes no structure and
  // an empty body is valid.
  body: z.string(),
}) satisfies z.ZodType<FrameDoc>;

// Resolver schema for the form: includes the version-scoped changelog. Used so
// zodResolver does not strip `changelog` from the submitted values (z.object
// drops unknown keys by default).
export const authoringFormSchema = authoringSchema.extend({
  changelog: z.string().optional(),
});

export function emptyFrameDoc(): FrameDoc {
  return {
    name: "",
    description: "",
    version: "",
    visibility: DEFAULT_VISIBILITY,
    scope: "",
    maintainer: "",
    template: false,
    body: "",
  };
}

export function suggestNextVersion(current: string): string {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(current.trim());
  if (!m) return current;
  return `${m[1]}.${m[2]}.${Number(m[3]) + 1}`;
}
