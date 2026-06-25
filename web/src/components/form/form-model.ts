import type { FrameDoc } from "@/lib/frame-yaml";

export interface AuthoringForm extends FrameDoc {
  changelog: string;
}

// Strips rows the user left blank so client + server agree on emptiness.
export function formToDoc(form: AuthoringForm): FrameDoc {
  const s = form.slots ?? {};

  const slots: FrameDoc["slots"] = {};

  const terminology = (s.terminology ?? []).filter(
    (t) => t.term.trim() !== "" || t.definition.trim() !== "",
  );
  if (terminology.length > 0) slots.terminology = terminology;

  const rules = (s.rules ?? []).filter((x) => x.trim() !== "");
  if (rules.length > 0) slots.rules = rules;

  const skills = (s.skills ?? []).filter((x) => x.trim() !== "");
  if (skills.length > 0) slots.skills = skills;

  const prompts = (s.prompts ?? []).filter((x) => x.trim() !== "");
  if (prompts.length > 0) slots.prompts = prompts;

  for (const key of [
    "tool_specs",
    "goals",
    "style",
    "norms",
    "architecture",
    "business_process",
  ] as const) {
    const v = s[key];
    if (typeof v === "string" && v.trim() !== "") {
      (slots as Record<string, unknown>)[key] = v;
    }
  }

  const doc: FrameDoc = {
    name: form.name,
    description: form.description,
    version: form.version,
    slots,
  };

  const extendsFiltered = (form.extends ?? []).filter(
    (e) => e.ref.trim() !== "",
  );
  if (extendsFiltered.length > 0) doc.extends = extendsFiltered;

  const excludesFiltered = (form.excludes ?? []).filter(
    (x) => x.trim() !== "",
  );
  if (excludesFiltered.length > 0) doc.excludes = excludesFiltered;

  return doc;
}

export function docToForm(doc: FrameDoc, changelog: string): AuthoringForm {
  return { ...doc, changelog };
}
