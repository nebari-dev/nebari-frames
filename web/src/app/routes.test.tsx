import { vi } from "vitest";
vi.mock("@/lib/auth/useAuth", () => ({
  useAuth: () => ({ user: { profile: { email: "u@example.com" } }, logout: vi.fn() }),
}));
// AppShell calls useQuery(getMe) to decide whether to show the Admin link.
// Stub it so this router-level render needs no QueryClient/transport.
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: undefined, isLoading: false, error: null }),
}));

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";
import { AppShell } from "./AppShell";

it("renders a Connect link in the header", () => {
  render(
    <MemoryRouter>
      <AppShell />
    </MemoryRouter>,
  );
  const link = screen.getByRole("link", { name: /^connect$/i });
  expect(link).toHaveAttribute("href", "/connect");
});
