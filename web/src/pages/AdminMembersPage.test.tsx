import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

const useQueryMock = vi.fn();
const addMutate = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => useQueryMock(),
  useMutation: (m: unknown) => {
    void m;
    return { mutate: addMutate, isPending: false };
  },
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { AdminMembersPage } from "./AdminMembersPage";

it("renders members with active and pending status", () => {
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: null,
    data: {
      members: [
        { userSub: "s1", email: "a@x.io", role: "admin" },
        { userSub: "", email: "p@x.io", role: "viewer" },
      ],
    },
  });
  render(<MemoryRouter><AdminMembersPage /></MemoryRouter>);
  expect(screen.getByText("a@x.io")).toBeInTheDocument();
  expect(screen.getByText("p@x.io")).toBeInTheDocument();
  expect(screen.getByText(/pending/i)).toBeInTheDocument();
});

it("submits an add-member form", async () => {
  useQueryMock.mockReturnValue({ isLoading: false, error: null, data: { members: [] } });
  render(<MemoryRouter><AdminMembersPage /></MemoryRouter>);
  await userEvent.type(screen.getByLabelText(/email/i), "new@x.io");
  await userEvent.click(screen.getByRole("button", { name: /add member/i }));
  expect(addMutate).toHaveBeenCalled();
});
