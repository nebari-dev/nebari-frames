import { describe, it, expect } from "vitest";
import { authoringSchema, emptyFrameDoc, suggestNextVersion } from "./authoring-schema";

describe("authoringSchema", () => {
  it("accepts a minimal valid doc", () => {
    const r = authoringSchema.safeParse({ name: "ok-name", description: "d", version: "1.0.0", slots: {} });
    expect(r.success).toBe(true);
  });
  it("rejects bad name, empty description, empty version", () => {
    const r = authoringSchema.safeParse({ name: "Bad_Name", description: "", version: "", slots: {} });
    expect(r.success).toBe(false);
    const paths = !r.success ? r.error.issues.map((i) => i.path.join(".")) : [];
    expect(paths).toContain("name");
    expect(paths).toContain("description");
    expect(paths).toContain("version");
  });
  it("rejects description over 280 chars", () => {
    const r = authoringSchema.safeParse({ name: "n", description: "x".repeat(281), version: "1", slots: {} });
    expect(r.success).toBe(false);
  });
  it("rejects duplicate terminology terms", () => {
    const r = authoringSchema.safeParse({
      name: "n", description: "d", version: "1", slots: { terminology: [{ term: "a", definition: "x" }, { term: "a", definition: "y" }] },
    });
    expect(r.success).toBe(false);
  });
  it("rejects extends ref without slash or empty version", () => {
    const r = authoringSchema.safeParse({
      name: "n", description: "d", version: "1", slots: {}, extends: [{ ref: "noslash", version: "" }],
    });
    expect(r.success).toBe(false);
  });
});

describe("suggestNextVersion", () => {
  it("bumps the patch of semver", () => {
    expect(suggestNextVersion("1.2.0")).toBe("1.2.1");
  });
  it("passes through non-semver unchanged", () => {
    expect(suggestNextVersion("2024.4")).toBe("2024.4");
  });
});

describe("emptyFrameDoc", () => {
  it("produces a parseable empty shape", () => {
    const d = emptyFrameDoc();
    expect(d.name).toBe("");
    expect(d.slots).toEqual({});
  });
});
