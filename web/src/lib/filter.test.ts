import { describe, expect, it } from "vitest";
import { filterFrames } from "./filter";

const frames = [
  { name: "brand-voice", description: "OpenTeams brand voice" },
  { name: "healthcare-compliance", description: "HIPAA rules" },
] as Parameters<typeof filterFrames>[0];

describe("filterFrames", () => {
  it("returns all when query empty", () => {
    expect(filterFrames(frames, "")).toHaveLength(2);
  });
  it("matches on name, case-insensitive", () => {
    expect(filterFrames(frames, "BRAND")).toEqual([frames[0]]);
  });
  it("matches on description", () => {
    expect(filterFrames(frames, "hipaa")).toEqual([frames[1]]);
  });
  it("returns empty when nothing matches", () => {
    expect(filterFrames(frames, "zzz")).toHaveLength(0);
  });
});
