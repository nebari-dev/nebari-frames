import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi } from "vitest";
import { buildHierarchy, type FrameNodeInput } from "@/lib/frame-hierarchy";

const hook = vi.fn();
vi.mock("./useFrameHierarchy", () => ({ useFrameHierarchy: () => hook() }));

import { FrameHierarchyPage } from "./FrameHierarchyPage";

function node(id: string): FrameNodeInput {
  const [orgSlug = "", name = ""] = id.split("/");
  return { id, orgSlug, name, description: "", latestVersion: "1.0.0" };
}

const graph = buildHierarchy(
  [node("o/base"), node("o/child"), node("o/unrelated")],
  [{ child: "o/child", parent: "o/base", version: "1.0.0" }],
);

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/hierarchy" element={<FrameHierarchyPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

it("renders a node per frame, linking to its detail page", () => {
  hook.mockReturnValue({ graph, isLoading: false, error: null });
  renderAt("/hierarchy");
  expect(screen.getByRole("link", { name: /base/ })).toHaveAttribute("href", "/frames/o/base");
  expect(screen.getByRole("link", { name: /child/ })).toHaveAttribute("href", "/frames/o/child");
});

it("shows a loading skeleton", () => {
  hook.mockReturnValue({ graph: undefined, isLoading: true, error: null });
  const { container } = render(
    <MemoryRouter><FrameHierarchyPage /></MemoryRouter>,
  );
  expect(container.querySelector('[data-slot="skeleton"]')).toBeInTheDocument();
});

it("shows an error message", () => {
  hook.mockReturnValue({ graph: undefined, isLoading: false, error: new Error("boom") });
  render(<MemoryRouter><FrameHierarchyPage /></MemoryRouter>);
  expect(screen.getByText(/could not load the frame hierarchy/i)).toBeInTheDocument();
});

it("shows an empty state when there are no frames", () => {
  hook.mockReturnValue({ graph: buildHierarchy([], []), isLoading: false, error: null });
  render(<MemoryRouter><FrameHierarchyPage /></MemoryRouter>);
  expect(screen.getByText(/no frames found/i)).toBeInTheDocument();
});

it("notes when no inheritance relationships exist", () => {
  hook.mockReturnValue({ graph: buildHierarchy([node("o/a")], []), isLoading: false, error: null });
  render(<MemoryRouter><FrameHierarchyPage /></MemoryRouter>);
  expect(screen.getByText(/no frame extends another/i)).toBeInTheDocument();
});

it("dims frames outside the focused lineage", () => {
  hook.mockReturnValue({ graph, isLoading: false, error: null });
  renderAt("/hierarchy?focus=o/child");
  // The unrelated frame is dimmed; the focused frame's ancestor is not.
  expect(screen.getByRole("link", { name: /unrelated/ }).className).toMatch(/opacity-40/);
  expect(screen.getByRole("link", { name: /base/ }).className).not.toMatch(/opacity-40/);
});
