import type { FrameDoc } from "@/lib/frame-yaml";

// The ten content sections of a Frame, in canonical (schema/spec) order.
// Single frontend source for section keys, labels, editor kind, and the
// one-line hints the authoring surface shows. Mirrors the order of
// backend/internal/frames/slots.go (SlotTable) and the .frame.md headings.
export type SlotKind = "terms" | "list" | "prose";

export interface SlotSectionDef {
  key: keyof FrameDoc["slots"];
  path: `slots.${string}`;
  label: string;
  kind: SlotKind;
  // Shown in the "+ Add section" menu and as the section's empty-state hint,
  // so an author knows what belongs in a section before committing to it.
  hint: string;
}

export const SLOT_SECTIONS: SlotSectionDef[] = [
  {
    key: "terminology",
    path: "slots.terminology",
    label: "Terminology",
    kind: "terms",
    hint: "Terms and their definitions — the vocabulary this frame establishes.",
  },
  {
    key: "rules",
    path: "slots.rules",
    label: "Rules",
    kind: "list",
    hint: "Hard constraints the AI must follow. One rule per entry.",
  },
  {
    key: "skills",
    path: "slots.skills",
    label: "Skills",
    kind: "list",
    hint: "Capabilities or know-how this frame assumes or provides.",
  },
  {
    key: "prompts",
    path: "slots.prompts",
    label: "Prompts",
    kind: "list",
    hint: "Reusable prompt snippets for common tasks in this context.",
  },
  {
    key: "tool_specs",
    path: "slots.tool_specs",
    label: "Tool Specifications",
    kind: "prose",
    hint: "Tools, integrations, or interfaces this frame's work relies on.",
  },
  {
    key: "goals",
    path: "slots.goals",
    label: "Goals",
    kind: "prose",
    hint: "What good outcomes look like for work done under this frame.",
  },
  {
    key: "style",
    path: "slots.style",
    label: "Style",
    kind: "prose",
    hint: "Voice, tone, and formatting guidance.",
  },
  {
    key: "norms",
    path: "slots.norms",
    label: "Norms",
    kind: "prose",
    hint: "Ways of working — how this team or org expects things to be done.",
  },
  {
    key: "architecture",
    path: "slots.architecture",
    label: "Architecture",
    kind: "prose",
    hint: "System or organizational structure worth knowing about.",
  },
  {
    key: "business_process",
    path: "slots.business_process",
    label: "Business Process",
    kind: "prose",
    hint: "Processes, stages, and handoffs work moves through.",
  },
];

// True when a section carries content in a parsed/form document value.
export function sectionHasContent(def: SlotSectionDef, slots: FrameDoc["slots"]): boolean {
  const v = slots[def.key];
  if (def.kind === "prose") return typeof v === "string" && v.trim() !== "";
  return Array.isArray(v) && v.length > 0;
}
