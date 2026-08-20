import { describe, expect, it } from "vitest";
import { parseFrameContent, serializeFrameDoc, type FrameDoc } from "./frame-yaml";

const yamlDoc = `
name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
extends:
  - ref: openteams/company-frame
    version: 1.2.0
body: |
  Be helpful and on-brand.

  Never claim performance numbers without data.
`;

describe("parseFrameContent", () => {
  it("parses metadata, extends, and the body", () => {
    const doc = parseFrameContent(yamlDoc);
    expect(doc.name).toBe("brand-voice");
    expect(doc.extends?.[0]).toEqual({ ref: "openteams/company-frame", version: "1.2.0" });
    expect(doc.body).toContain("on-brand");
    expect(doc.body).toContain("Never claim performance numbers without data.");
  });

  it("accepts a Uint8Array (the content wire type)", () => {
    const bytes = new TextEncoder().encode(yamlDoc);
    expect(parseFrameContent(bytes).name).toBe("brand-voice");
  });

  it("throws on malformed yaml", () => {
    expect(() => parseFrameContent("name: [unclosed")).toThrow();
  });

  // Versions published under the retired ten-slot schema must keep rendering:
  // a legacy slots: block folds into the body as the old markdown sections.
  it("folds legacy slots into the body", () => {
    const legacy = `
name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
slots:
  terminology:
    - term: customer
      definition: An enterprise organization
  rules:
    - Never claim performance numbers without data
  goals: |
    Be helpful and on-brand.
`;
    const doc = parseFrameContent(legacy);
    expect(doc.body).toContain("## Terminology");
    expect(doc.body).toContain("- **customer**: An enterprise organization");
    expect(doc.body).toContain("## Rules");
    expect(doc.body).toContain("- Never claim performance numbers without data");
    expect(doc.body).toContain("## Goals");
    expect(doc.body).toContain("Be helpful and on-brand.");
    // The legacy shape is read-only: serialization emits body, never slots.
    expect(serializeFrameDoc(doc)).not.toMatch(/slots:/);
  });
});

describe("serializeFrameDoc", () => {
  it("round-trips a full frame to an equal doc", () => {
    const doc = parseFrameContent(yamlDoc);
    const round = parseFrameContent(serializeFrameDoc(doc));
    expect(round).toEqual(doc);
  });

  it("omits empty body, arrays, and strings", () => {
    const doc: FrameDoc = {
      name: "minimal",
      description: "d",
      version: "1.0.0",
      visibility: "",
      scope: "",
      maintainer: "",
      template: false,
      body: "",
    };
    const out = serializeFrameDoc(doc);
    expect(out).not.toMatch(/body/);
    expect(out).not.toMatch(/template/);
    expect(out).not.toMatch(/visibility/);
    expect(out).not.toMatch(/scope/);
    expect(out).not.toMatch(/maintainer/);
    expect(out).not.toMatch(/extends/);
    expect(out).not.toMatch(/excludes/);
    // re-parse must succeed (no stray keys for KnownFields(true))
    expect(parseFrameContent(out).name).toBe("minimal");
  });
});
