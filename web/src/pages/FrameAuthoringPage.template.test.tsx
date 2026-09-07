import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi, beforeEach } from "vitest";

const useQueryMock = vi.fn();
const useMutationMock = vi.fn();
// Arguments are forwarded so tests can route by which RPC was asked for. Routing
// by call order instead breaks the moment React re-renders, which it does on
// every keystroke in the form.
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...args: unknown[]) => useQueryMock(...args),
  useMutation: (...args: unknown[]) => useMutationMock(...args),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));

import { FrameAuthoringPage } from "./FrameAuthoringPage";

const templateList = {
  isLoading: false,
  error: null,
  data: {
    canManage: false,
    templates: [
      { id: "builtin:blank", title: "Blank", description: "Empty.", builtin: true },
      { id: "builtin:domain-vocabulary", title: "Domain Vocabulary", description: "Terms.", builtin: true },
    ],
  },
};

beforeEach(() => {
  useQueryMock.mockReturnValue(templateList);
  useMutationMock.mockReturnValue({ mutateAsync: vi.fn(), isPending: false });
});

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/frames/new" element={<FrameAuthoringPage mode="create" />} />
      </Routes>
    </MemoryRouter>,
  );
}

it("shows the picker at /frames/new with no template chosen", () => {
  renderAt("/frames/new");
  expect(screen.getByRole("heading", { name: /built-in/i })).toBeInTheDocument();
  // The authoring form must not be on the page yet.
  expect(screen.queryByLabelText(/frame name/i)).not.toBeInTheDocument();
});

it("navigates to the seeded form when a template is chosen", async () => {
  renderAt("/frames/new");
  await userEvent.click(screen.getByRole("button", { name: /domain vocabulary/i }));
  // Choosing a template puts it in the URL, so the choice is linkable and a
  // reload does not lose it.
  expect(screen.queryByRole("heading", { name: /built-in/i })).not.toBeInTheDocument();
  expect(screen.getByLabelText(/frame name/i)).toBeInTheDocument();
});

it("skips the picker when a template is already in the URL", () => {
  renderAt("/frames/new?template=builtin:blank");
  expect(screen.queryByRole("heading", { name: /built-in/i })).not.toBeInTheDocument();
  expect(screen.getByLabelText(/frame name/i)).toBeInTheDocument();
});

it("skips the picker for the .frame.md import path", () => {
  renderAt("/frames/new?import=1");
  expect(screen.queryByRole("heading", { name: /built-in/i })).not.toBeInTheDocument();
});
