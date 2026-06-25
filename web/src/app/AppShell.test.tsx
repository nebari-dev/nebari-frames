import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));
vi.mock("@/lib/auth/useAuth", () => ({
  useAuth: () => ({ logout: vi.fn(), user: { profile: { email: "a@x.io" } } }),
}));

import { AppShell } from "./AppShell";

it("shows the Admin link for an admin", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { role: "admin" } });
  render(<MemoryRouter><AppShell /></MemoryRouter>);
  expect(screen.getByRole("link", { name: /admin/i })).toBeInTheDocument();
});

it("hides the Admin link for a non-admin", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { role: "viewer" } });
  render(<MemoryRouter><AppShell /></MemoryRouter>);
  expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
});
