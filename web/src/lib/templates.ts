import { Requirement } from "@gen/frames/v1/frame_pb";
import { SLOT_SECTIONS, type SlotSectionDef } from "@/lib/slot-sections";

// A template's rules, keyed by slot. Shaped like the generated map field so the
// response can be handed straight in.
export type TemplateRules = Record<string, { level: Requirement; note: string }>;

/**
 * The sections a template wants an author to see up front: required and
 * recommended, in schema order. Optional slots stay behind "+ Add section" -
 * the point of a template is to narrow the decisions, not to reopen all ten.
 *
 * Driven off SLOT_SECTIONS rather than the rule map, so order is deterministic
 * and a rule naming a slot that no longer exists is ignored rather than
 * crashing the form.
 */
export function seededSections(rules: TemplateRules): SlotSectionDef[] {
  return SLOT_SECTIONS.filter((def) => {
    const level = rules[def.key]?.level;
    return level === Requirement.REQUIRED || level === Requirement.RECOMMENDED;
  });
}

/**
 * The template id a publish should carry.
 *
 * Create only, and only when a template was actually chosen: the backend uses
 * the id to check that template's required slots and then discards it, and that
 * check applies to a new Frame. Edit must never send one, and neither must the
 * .frame.md import path, whose document came from a file rather than a template.
 */
export function publishTemplateID({
  mode,
  importing,
  templateID,
}: {
  mode: "create" | "edit";
  importing: boolean;
  templateID: string;
}): string {
  return mode === "create" && !importing ? templateID : "";
}

/**
 * Whether a section may not be removed. A required section that an author
 * deletes guarantees a publish failure, so the remove control is withheld
 * rather than offered and then punished.
 */
export function isRequiredSection(rules: TemplateRules, key: string): boolean {
  return rules[key]?.level === Requirement.REQUIRED;
}

/**
 * What to show under a section heading. The template author's note beats the
 * generic per-slot hint: they know why their org wants this section, and the
 * generic text does not.
 */
export function sectionHint(rules: TemplateRules, def: SlotSectionDef): string {
  const note = rules[def.key]?.note?.trim();
  return note ? note : def.hint;
}
