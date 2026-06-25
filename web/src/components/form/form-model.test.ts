import { describe, it, expect } from "vitest";
import { formToDoc, docToForm } from "./form-model";

describe("form-model", () => {
  it("docToForm then formToDoc round-trips slot content", () => {
    const doc = {
      name: "n", description: "d", version: "1.0.0",
      extends: [{ ref: "o/p", version: "1.0.0" }],
      slots: { rules: ["a"], goals: "g" },
    };
    const form = docToForm(doc, "changed");
    expect(form.changelog).toBe("changed");
    expect(formToDoc(form)).toEqual(doc);
  });

  it("formToDoc drops empty extends and blank slots", () => {
    const doc = formToDoc({
      name: "n", description: "d", version: "1.0.0", changelog: "",
      extends: [{ ref: "", version: "" }],
      slots: { rules: [], goals: "" },
    });
    expect(doc.extends ?? []).toHaveLength(0);
    expect(doc.slots.rules ?? []).toHaveLength(0);
  });
});
