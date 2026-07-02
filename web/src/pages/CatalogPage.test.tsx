import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { buildHierarchy } from "@/lib/frame-hierarchy";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));
vi.mock("./useFrameHierarchy", () => ({
  useFrameHierarchy: () => ({
    graph: buildHierarchy(
      [
        { id: "openteams/brand-voice", orgSlug: "openteams", name: "brand-voice", description: "", latestVersion: "1.0.0" },
      ],
      [],
    ),
    isLoading: false,
    error: null,
  }),
}));

import { CatalogPage } from "./CatalogPage";

const twoFrames = {
  isLoading: false,
  error: null,
  data: {
    frames: [
      { orgSlug: "openteams", name: "brand-voice", description: "brand voice", ownerSub: "u1", latestVersion: "1.0.0" },
      { orgSlug: "openteams", name: "hipaa", description: "compliance", ownerSub: "u1", latestVersion: "2.0.0" },
    ],
  },
};

it("shows Create button only when can_create", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { frames: [], canCreate: true } });
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /create new frame/i })).toBeInTheDocument();
});

it("hides Create button when not can_create", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { frames: [], canCreate: false } });
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(screen.queryByRole("link", { name: /create new frame/i })).not.toBeInTheDocument();
});

it("renders skeletons while loading", () => {
  useQueryMock.mockReturnValue({ isLoading: true, error: null, data: undefined });
  const { container } = render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
});

it("renders an error state on failure", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: new Error("boom"), data: undefined });
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(screen.getByText(/could not load frames/i)).toBeInTheDocument();
});

it("lists frames and filters by search", async () => {
  useQueryMock.mockReturnValue(twoFrames);
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(screen.getByText("brand-voice")).toBeInTheDocument();
  expect(screen.getByText("hipaa")).toBeInTheDocument();

  await userEvent.type(screen.getByPlaceholderText(/search frames/i), "brand");
  expect(screen.getByText("brand-voice")).toBeInTheDocument();
  expect(screen.queryByText("hipaa")).not.toBeInTheDocument();
});

it("switches to the table view", async () => {
  useQueryMock.mockReturnValue(twoFrames);
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  await userEvent.click(screen.getByRole("button", { name: /table/i }));
  const table = screen.getByRole("table");
  expect(table).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: /owner/i })).toBeInTheDocument();
});

it("switches to the hierarchy view and hides the search box", async () => {
  useQueryMock.mockReturnValue(twoFrames);
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  await userEvent.click(screen.getByRole("button", { name: /hierarchy/i }));
  expect(screen.queryByPlaceholderText(/search frames/i)).not.toBeInTheDocument();
  // The mocked hierarchy graph renders its single frame as a link.
  expect(screen.getByRole("link", { name: /brand-voice/ })).toHaveAttribute(
    "href",
    "/frames/openteams/brand-voice",
  );
});

it("opens directly in the hierarchy view from a ?view=hierarchy URL", () => {
  useQueryMock.mockReturnValue(twoFrames);
  render(
    <MemoryRouter initialEntries={["/?view=hierarchy"]}><CatalogPage /></MemoryRouter>,
  );
  expect(screen.queryByPlaceholderText(/search frames/i)).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /hierarchy/i })).toHaveAttribute("aria-pressed", "true");
});
