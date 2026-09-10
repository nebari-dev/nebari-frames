import { describe, expect, it } from "vitest";
import { Requirement } from "@gen/frames/v1/frame_pb";
import { seededSections, isRequiredSection, sectionHint, publishTemplateID, type TemplateRules } from "./templates";
import { SLOT_SECTIONS } from "./slot-sections";

const terminology = SLOT_SECTIONS.find((s) => s.key === "terminology")!;

describe("seededSections", () => {
  it("returns nothing for a template with no rules", () => {
    expect(seededSections({})).toEqual([]);
  });

  it("includes required and recommended sections but not optional ones", () => {
    const rules: TemplateRules = {
      terminology: { level: Requirement.REQUIRED, note: "" },
      rules: { level: Requirement.RECOMMENDED, note: "" },
      style: { level: Requirement.OPTIONAL, note: "" },
    };
    expect(seededSections(rules).map((s) => s.key)).toEqual(["terminology", "rules"]);
  });

  it("returns sections in schema order, not rule-map order", () => {
    const rules: TemplateRules = {
      business_process: { level: Requirement.REQUIRED, note: "" },
      terminology: { level: Requirement.REQUIRED, note: "" },
      style: { level: Requirement.REQUIRED, note: "" },
    };
    expect(seededSections(rules).map((s) => s.key)).toEqual(["terminology", "style", "business_process"]);
  });

  it("ignores a rule naming a slot that does not exist", () => {
    const rules: TemplateRules = { nosuchslot: { level: Requirement.REQUIRED, note: "" } };
    expect(seededSections(rules)).toEqual([]);
  });
});

describe("publishTemplateID", () => {
  // Which id a publish carries is a pure rule, so it is tested as one rather
  // than inferred from whether a form submission happened to fire.
  it.each([
    [{ mode: "create" as const, importing: false, templateID: "builtin:blank" }, "builtin:blank"],
    // Edit never carries a template: requirements are checked on create only.
    [{ mode: "edit" as const, importing: false, templateID: "builtin:blank" }, ""],
    // The import path brought its own document; applying a standard the author
    // never chose would be a surprise.
    [{ mode: "create" as const, importing: true, templateID: "builtin:blank" }, ""],
    [{ mode: "create" as const, importing: false, templateID: "" }, ""],
  ])("%o -> %s", (input, want) => {
    expect(publishTemplateID(input)).toBe(want);
  });
});

describe("isRequiredSection", () => {
  it.each([
    [Requirement.REQUIRED, true],
    [Requirement.RECOMMENDED, false],
    [Requirement.OPTIONAL, false],
    [Requirement.UNSPECIFIED, false],
  ])("level %s -> %s", (level, want) => {
    expect(isRequiredSection({ terminology: { level, note: "" } }, "terminology")).toBe(want);
  });

  it("is false for a slot with no rule", () => {
    expect(isRequiredSection({}, "terminology")).toBe(false);
  });
});

describe("sectionHint", () => {
  it("prefers the template's note over the generic hint", () => {
    const rules: TemplateRules = { terminology: { level: Requirement.REQUIRED, note: "One entry per term of art." } };
    expect(sectionHint(rules, terminology)).toBe("One entry per term of art.");
  });

  it("falls back to the generic hint when the note is empty", () => {
    const rules: TemplateRules = { terminology: { level: Requirement.REQUIRED, note: "" } };
    expect(sectionHint(rules, terminology)).toBe(terminology.hint);
  });

  it("falls back to the generic hint when there is no rule at all", () => {
    expect(sectionHint({}, terminology)).toBe(terminology.hint);
  });
});
