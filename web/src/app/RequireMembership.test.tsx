import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";

const useQueryMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...args: unknown[]) => useQueryMock(...args),
}));

import { RequireMembership } from "./RequireMembership";

function renderAt() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route element={<RequireMembership />}>
          <Route path="/" element={<div>catalog</div>} />
        </Route>
        <Route path="/no-access" element={<div>no-access-page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

it("redirects to /no-access on permission_denied", async () => {
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: new ConnectError("no org membership", Code.PermissionDenied),
  });
  renderAt();
  await waitFor(() => expect(screen.getByText("no-access-page")).toBeInTheDocument());
});

it("renders the outlet when GetMe succeeds", async () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { canCreate: false } });
  renderAt();
  await waitFor(() => expect(screen.getByText("catalog")).toBeInTheDocument());
});
