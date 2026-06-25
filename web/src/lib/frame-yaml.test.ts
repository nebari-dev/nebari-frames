import { describe, expect, it } from "vitest";
import { parseFrameContent, serializeFrameDoc, type FrameDoc } from "./frame-yaml";

const yamlDoc = `
name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
extends:
  - ref: openteams/company-frame
    version: 1.2.0
slots:
  terminology:
    - term: customer
      definition: An enterprise organization
  rules:
    - Never claim performance numbers without data
  goals: |
    Be helpful and on-brand.
`;

describe("parseFrameContent", () => {
  it("parses metadata, extends, and slots", () => {
    const doc = parseFrameContent(yamlDoc);
    expect(doc.name).toBe("brand-voice");
    expect(doc.extends?.[0]).toEqual({ ref: "openteams/company-frame", version: "1.2.0" });
    expect(doc.slots.terminology?.[0].term).toBe("customer");
    expect(doc.slots.rules).toEqual(["Never claim performance numbers without data"]);
    expect(doc.slots.goals).toContain("on-brand");
  });

  it("accepts a Uint8Array (the content wire type)", () => {
    const bytes = new TextEncoder().encode(yamlDoc);
    expect(parseFrameContent(bytes).name).toBe("brand-voice");
  });

  it("throws on malformed yaml", () => {
    expect(() => parseFrameContent("name: [unclosed")).toThrow();
  });
});

describe("serializeFrameDoc", () => {
  it("round-trips a full frame to an equal doc", () => {
    const yaml = `name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
extends:
  - ref: openteams/company
    version: 1.2.0
slots:
  terminology:
    - term: customer
      definition: An enterprise organization
  rules:
    - Cite benchmarks.
  goals: |
    Be clear.
`;
    const doc = parseFrameContent(yaml);
    const round = parseFrameContent(serializeFrameDoc(doc));
    expect(round).toEqual(doc);
  });

  it("omits empty slots, arrays, and strings", () => {
    const doc: FrameDoc = {
      name: "minimal",
      description: "d",
      version: "1.0.0",
      slots: { rules: [], goals: "", terminology: [] },
    };
    const out = serializeFrameDoc(doc);
    expect(out).not.toMatch(/rules/);
    expect(out).not.toMatch(/goals/);
    expect(out).not.toMatch(/terminology/);
    expect(out).not.toMatch(/extends/);
    expect(out).not.toMatch(/excludes/);
    // re-parse must succeed (no stray keys for KnownFields(true))
    expect(parseFrameContent(out).name).toBe("minimal");
  });
});
