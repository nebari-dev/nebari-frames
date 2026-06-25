import { render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { expect, it, vi } from "vitest";

// Mock useAuth to report an unauthenticated user. The real hook returns
// { status: "loading" | "anonymous" | "authenticated" } and RequireAuth
// redirects when status === "anonymous" (see RequireAuth.tsx + useAuth.ts).
vi.mock("@/lib/auth/useAuth", () => ({
  useAuth: () => ({ status: "anonymous" }),
}));

import { RequireAuth } from "./RequireAuth";

it("redirects unauthenticated users to /login", () => {
  render(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route element={<RequireAuth />}>
          <Route path="/" element={<div>protected</div>} />
        </Route>
        <Route path="/login" element={<div>login page</div>} />
      </Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText("login page")).toBeInTheDocument();
  expect(screen.queryByText("protected")).not.toBeInTheDocument();
});
