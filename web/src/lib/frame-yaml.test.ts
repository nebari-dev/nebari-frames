import { readFileSync } from "node:fs";
import path from "node:path";
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

// The legacy `slots:` renderer exists twice: here and in renderMarkdown in
// backend/internal/frames/legacy.go. The web copy is not display-only -
// FrameDetailPage re-serializes a rendered legacy body when a version is
// restored, so whatever this produces becomes canonical stored content, and a
// divergence from Go is a silent content rewrite rather than a rendering
// difference.
//
// testdata/legacy-slots is the single fixture both sides are pinned to, and the
// comparison is on the whole string. The substring assertions above are what let
// the two implementations drift on continuation-line indentation and on
// newline-only versus whitespace trimming while both suites stayed green.
//
// The Go-side assertion is TestLegacySlots_SharedFixture in
// backend/internal/frames/legacy_test.go. Changing the rendering means
// regenerating expected.md and updating both.
describe("legacy slots rendering matches the Go implementation", () => {
  const dir = path.resolve(__dirname, "../../../testdata/legacy-slots");
  const input = readFileSync(path.join(dir, "input.yaml"), "utf8");
  // expected.md carries a trailing newline so it is a well-formed text file;
  // the rendered body does not.
  const expected = readFileSync(path.join(dir, "expected.md"), "utf8").replace(/\n$/, "");

  it("renders the shared fixture byte for byte", () => {
    expect(parseFrameContent(input).body).toBe(expected);
  });

  // The case that makes the divergence matter: without the two-space
  // continuation indent, CommonMark lazy continuation absorbs the following
  // lines as siblings, so one rule with a nested list becomes three flat rules
  // and the nesting is gone from the stored content permanently.
  it("indents continuation lines so a multi-line item stays one item", () => {
    expect(expected).toContain("- Redact before logging:\n  - no names\n  - no account numbers");
  });

  // A blank line inside an item stays bare rather than becoming indented
  // whitespace, matching writeLegacyBullet.
  it("keeps a blank line inside an item blank", () => {
    expect(expected).toContain("- Cite the benchmark.\n\n  Link to the run that produced it.");
  });

  // Prose is trimmed of newlines only. A .trim() port would eat the leading
  // spaces, which markdown gives meaning to.
  it("trims newlines but not other whitespace from prose", () => {
    expect(expected).toContain("## Style\n\n  Two leading spaces, and a trailing newline to strip.");
  });
});
