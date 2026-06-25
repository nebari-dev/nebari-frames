import { vi } from "vitest";

vi.mock("@/lib/auth/useAuth", () => ({
  useAuth: () => ({ status: "authenticated", user: { profile: { email: "u@example.com" } }, logout: vi.fn() }),
}));

// AppShell, RequireMembership, and RequireAdmin all call useQuery(getMe).
// Use a vi.fn() so individual tests can control the returned role/error.
const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => useQueryMock(),
}));

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, beforeEach } from "vitest";
import { AppShell } from "./AppShell";
import { AppRoutes } from "./routes";

// Default: no error, no role (member but not admin) - satisfies RequireMembership
beforeEach(() => {
  useQueryMock.mockReturnValue({ data: { role: "viewer" }, isLoading: false, error: null });
});

it("renders a Connect link in the header", () => {
  render(
    <MemoryRouter>
      <AppShell />
    </MemoryRouter>,
  );
  const link = screen.getByRole("link", { name: /^connect$/i });
  expect(link).toHaveAttribute("href", "/connect");
});

it("renders AdminHomePage at /admin for an admin user", () => {
  useQueryMock.mockReturnValue({ data: { role: "admin" }, isLoading: false, error: null });
  render(
    <MemoryRouter initialEntries={["/admin"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  expect(screen.getByRole("heading", { name: /^admin$/i })).toBeInTheDocument();
});

it("redirects /admin to catalog for a non-admin user", () => {
  useQueryMock.mockReturnValue({ data: { role: "viewer" }, isLoading: false, error: null });
  render(
    <MemoryRouter initialEntries={["/admin"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  // RequireAdmin redirects to "/" which renders CatalogPage
  // CatalogPage fetches frames - just verify we are NOT on the admin page
  expect(screen.queryByRole("heading", { name: /^admin$/i })).not.toBeInTheDocument();
});
