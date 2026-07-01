import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));
vi.mock("@/lib/auth/useAuth", () => ({
  useAuth: () => ({
    status: "authenticated",
    logout: vi.fn(),
    login: vi.fn(),
    user: { profile: { email: "a@x.io" } },
  }),
}));

import { ThemeProvider } from "@/lib/theme/ThemeContext";
import { AppShell } from "./AppShell";

function renderShell() {
  return render(
    <ThemeProvider>
      <MemoryRouter>
        <AppShell />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

it("shows the Admin link for an admin", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { role: "admin" } });
  renderShell();
  expect(screen.getByRole("link", { name: /admin/i })).toBeInTheDocument();
});

it("hides the Admin link for a non-admin", () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { role: "viewer" } });
  renderShell();
  expect(screen.queryByRole("link", { name: /admin/i })).not.toBeInTheDocument();
});
