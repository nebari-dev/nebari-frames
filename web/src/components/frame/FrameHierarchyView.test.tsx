import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";
import { buildHierarchy, type FrameNodeInput } from "@/lib/frame-hierarchy";
import { FrameHierarchyView } from "./FrameHierarchyView";

function node(id: string): FrameNodeInput {
  const [orgSlug = "", name = ""] = id.split("/");
  return { id, orgSlug, name, description: "", latestVersion: "1.0.0" };
}

const graph = buildHierarchy(
  [node("o/base"), node("o/child"), node("o/unrelated")],
  [{ child: "o/child", parent: "o/base", version: "1.0.0" }],
);

it("renders a node per frame, linking to its detail page", () => {
  render(<MemoryRouter><FrameHierarchyView graph={graph} /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /base/ })).toHaveAttribute("href", "/frames/o/base");
  expect(screen.getByRole("link", { name: /child/ })).toHaveAttribute("href", "/frames/o/child");
});

it("dims frames outside the focused lineage", () => {
  render(<MemoryRouter><FrameHierarchyView graph={graph} focus="o/child" /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /unrelated/ }).className).toMatch(/opacity-40/);
  expect(screen.getByRole("link", { name: /base/ }).className).not.toMatch(/opacity-40/);
});

it("does not dim anything when the focus id is absent from the graph", () => {
  render(<MemoryRouter><FrameHierarchyView graph={graph} focus="o/missing" /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /unrelated/ }).className).not.toMatch(/opacity-40/);
});
