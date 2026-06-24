import { describe, expect, it } from "vitest";
import { parseFrameContent } from "./frame-yaml";

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
