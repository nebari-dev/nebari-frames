import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));

import { CatalogPage } from "./CatalogPage";

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

it("lists frames and filters by search", async () => {
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: null,
    data: {
      frames: [
        { orgSlug: "openteams", name: "brand-voice", description: "brand voice", ownerSub: "u1", latestVersion: "1.0.0" },
        { orgSlug: "openteams", name: "hipaa", description: "compliance", ownerSub: "u1", latestVersion: "2.0.0" },
      ],
    },
  });
  render(<MemoryRouter><CatalogPage /></MemoryRouter>);
  expect(screen.getByText("brand-voice")).toBeInTheDocument();
  expect(screen.getByText("hipaa")).toBeInTheDocument();

  await userEvent.type(screen.getByPlaceholderText("Search frames..."), "brand");
  expect(screen.getByText("brand-voice")).toBeInTheDocument();
  expect(screen.queryByText("hipaa")).not.toBeInTheDocument();
});
