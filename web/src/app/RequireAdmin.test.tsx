import { render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({ useQuery: () => useQueryMock() }));

import { RequireAdmin } from "./RequireAdmin";

function renderAt(role: string | undefined, isLoading = false) {
  useQueryMock.mockReturnValue({ isLoading, error: null, data: role ? { role } : undefined });
  return render(
    <MemoryRouter initialEntries={["/admin"]}>
      <Routes>
        <Route element={<RequireAdmin />}>
          <Route path="/admin" element={<div>admin area</div>} />
        </Route>
        <Route path="/" element={<div>catalog</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

it("renders the outlet for an admin", () => {
  renderAt("admin");
  expect(screen.getByText("admin area")).toBeInTheDocument();
});

it("redirects a non-admin to the catalog", () => {
  renderAt("viewer");
  expect(screen.getByText("catalog")).toBeInTheDocument();
  expect(screen.queryByText("admin area")).not.toBeInTheDocument();
});
