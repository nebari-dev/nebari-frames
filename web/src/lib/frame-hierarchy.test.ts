import { describe, expect, it } from "vitest";
import { buildHierarchy, connectedTo, parentRefId, type FrameNodeInput } from "./frame-hierarchy";

function node(id: string): FrameNodeInput {
  const [orgSlug = "", name = ""] = id.split("/");
  return { id, orgSlug, name, description: `${name} desc`, latestVersion: "1.0.0" };
}

const levelOf = (g: ReturnType<typeof buildHierarchy>, id: string) =>
  g.nodes.find((n) => n.id === id)?.level;

describe("buildHierarchy", () => {
  it("places a base frame above the frame that extends it", () => {
    const g = buildHierarchy(
      [node("o/base"), node("o/child")],
      [{ child: "o/child", parent: "o/base", version: "1.0.0" }],
    );
    expect(levelOf(g, "o/base")).toBe(0);
    expect(levelOf(g, "o/child")).toBe(1);
    expect(g.edges).toHaveLength(1);
  });

  it("levels a multi-step chain by longest ancestor path", () => {
    const g = buildHierarchy(
      [node("o/a"), node("o/b"), node("o/c")],
      [
        { child: "o/b", parent: "o/a", version: "1" },
        { child: "o/c", parent: "o/b", version: "1" },
      ],
    );
    expect([levelOf(g, "o/a"), levelOf(g, "o/b"), levelOf(g, "o/c")]).toEqual([0, 1, 2]);
  });

  it("levels a diamond so the shared descendant sits below both parents", () => {
    const g = buildHierarchy(
      [node("o/a"), node("o/b"), node("o/c"), node("o/d")],
      [
        { child: "o/b", parent: "o/a", version: "1" },
        { child: "o/c", parent: "o/a", version: "1" },
        { child: "o/d", parent: "o/b", version: "1" },
        { child: "o/d", parent: "o/c", version: "1" },
      ],
    );
    expect(levelOf(g, "o/a")).toBe(0);
    expect(levelOf(g, "o/b")).toBe(1);
    expect(levelOf(g, "o/c")).toBe(1);
    expect(levelOf(g, "o/d")).toBe(2);
  });

  it("synthesizes an external placeholder for an unknown parent ref", () => {
    const g = buildHierarchy(
      [node("o/child")],
      [{ child: "o/child", parent: "other/base", version: "2.0.0" }],
    );
    const ext = g.nodes.find((n) => n.id === "other/base");
    expect(ext?.external).toBe(true);
    expect(ext?.orgSlug).toBe("other");
    expect(ext?.level).toBe(0);
    expect(levelOf(g, "o/child")).toBe(1);
  });

  it("drops edges whose child is not a known frame", () => {
    const g = buildHierarchy([node("o/a")], [{ child: "ghost/x", parent: "o/a", version: "1" }]);
    expect(g.edges).toHaveLength(0);
    expect(g.nodes).toHaveLength(1);
  });

  it("does not loop forever on an accidental cycle", () => {
    const g = buildHierarchy(
      [node("o/a"), node("o/b")],
      [
        { child: "o/a", parent: "o/b", version: "1" },
        { child: "o/b", parent: "o/a", version: "1" },
      ],
    );
    // Both levels are finite; the exact values don't matter, only that it terminates.
    expect(Number.isFinite(levelOf(g, "o/a")!)).toBe(true);
    expect(Number.isFinite(levelOf(g, "o/b")!)).toBe(true);
  });

  it("keeps an isolated frame at level 0 with no edges", () => {
    const g = buildHierarchy([node("o/lonely")], []);
    expect(levelOf(g, "o/lonely")).toBe(0);
    expect(g.rows[0].map((n) => n.id)).toContain("o/lonely");
  });

  it("assigns contiguous column indices within each row", () => {
    const g = buildHierarchy([node("o/a"), node("o/b"), node("o/c")], []);
    const cols = g.rows[0].map((n) => n.col).sort((x, y) => x - y);
    expect(cols).toEqual([0, 1, 2]);
  });
});

describe("connectedTo", () => {
  const g = buildHierarchy(
    [node("o/a"), node("o/b"), node("o/c"), node("o/unrelated")],
    [
      { child: "o/b", parent: "o/a", version: "1" }, // b extends a
      { child: "o/c", parent: "o/b", version: "1" }, // c extends b
    ],
  );

  it("returns the full lineage above and below the focus", () => {
    const set = connectedTo(g, "o/b");
    expect(set).toEqual(new Set(["o/a", "o/b", "o/c"]));
  });

  it("excludes frames outside the focus lineage", () => {
    expect(connectedTo(g, "o/b").has("o/unrelated")).toBe(false);
  });

  it("returns an empty set for an unknown focus id", () => {
    expect(connectedTo(g, "o/missing").size).toBe(0);
  });
});

describe("parentRefId", () => {
  it("joins org and name", () => {
    expect(parentRefId("openteams", "brand-voice")).toBe("openteams/brand-voice");
  });
});
