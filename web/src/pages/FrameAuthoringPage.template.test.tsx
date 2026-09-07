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
import { FrameService } from "@gen/frames/v1/frame_service_pb";

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

const vocabularyTemplate = {
  id: "builtin:domain-vocabulary",
  title: "Domain Vocabulary",
  description: "Terms.",
  builtin: true,
  prefill: new TextEncoder().encode("slots: {}\n"),
  fieldRules: {
    terminology: { level: 3, note: "One entry per term of art." },
    rules: { level: 2, note: "Usage constraints worth stating." },
  },
};

const orgTemplateWithPrefill = {
  id: "01JORGTEMPLATE0000000000AB",
  title: "Our House Style",
  description: "Ours.",
  builtin: false,
  prefill: new TextEncoder().encode(
    "slots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n    - term: Slot\n      definition: One section of a Frame.\n",
  ),
  fieldRules: { terminology: { level: 3, note: "Keep these, add your own." } },
};

// Routes on the RPC descriptor's object identity rather than call order (the
// convention already used in FrameAuthoringPage.edit.test.tsx and
// FrameDetailPage.test.tsx), so the mock keeps answering correctly across the
// re-renders a keystroke in the form causes.
function mockQueries(template: unknown) {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return { isLoading: false, error: null, data: { template } };
    }
    return { isLoading: false, error: null, data: templateList.data };
  });
}

it("pre-adds the required and recommended sections and hides optional ones", () => {
  mockQueries(vocabularyTemplate);
  renderAt("/frames/new?template=builtin:domain-vocabulary");
  expect(screen.getByRole("heading", { name: /^terminology$/i })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: /^rules$/i })).toBeInTheDocument();
  // Optional slots stay behind "+ Add section": a template narrows the
  // decisions rather than reopening all ten.
  expect(screen.queryByRole("heading", { name: /^architecture$/i })).not.toBeInTheDocument();
});

it("shows the template's note in place of the generic hint", () => {
  mockQueries(vocabularyTemplate);
  renderAt("/frames/new?template=builtin:domain-vocabulary");
  expect(screen.getByText("One entry per term of art.")).toBeInTheDocument();
  expect(screen.getByText("Usage constraints worth stating.")).toBeInTheDocument();
});

it("withholds the remove control on a required section but offers it on a recommended one", () => {
  mockQueries(vocabularyTemplate);
  renderAt("/frames/new?template=builtin:domain-vocabulary");
  // Removing a required section guarantees a publish failure, so it is not
  // offered. A recommended one is genuinely optional.
  const removes = screen.getAllByRole("button", { name: /remove section/i });
  expect(removes).toHaveLength(1);
});

it("populates prefilled content and lets it be edited", async () => {
  mockQueries(orgTemplateWithPrefill);
  renderAt("/frames/new?template=01JORGTEMPLATE0000000000AB");
  expect(screen.getByDisplayValue("Frame")).toBeInTheDocument();
  expect(screen.getByDisplayValue("A scoped context artifact.")).toBeInTheDocument();
  expect(screen.getByDisplayValue("Slot")).toBeInTheDocument();
  const term = screen.getByDisplayValue("Frame");
  await userEvent.clear(term);
  await userEvent.type(term, "Frame v2");
  expect(screen.getByDisplayValue("Frame v2")).toBeInTheDocument();
});

it("seeds nothing when the template is blank", () => {
  mockQueries({
    id: "builtin:blank", title: "Blank", description: "Empty.", builtin: true,
    prefill: new TextEncoder().encode("slots: {}\n"), fieldRules: {},
  });
  renderAt("/frames/new?template=builtin:blank");
  expect(screen.getByLabelText(/frame name/i)).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: /^terminology$/i })).not.toBeInTheDocument();
});
