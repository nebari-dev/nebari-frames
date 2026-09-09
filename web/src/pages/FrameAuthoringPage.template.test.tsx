import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, it, vi, beforeEach } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";

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
  isFetchedAfterMount: true,
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
      return { isLoading: false, error: null, isFetchedAfterMount: true, data: { template } };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateList.data };
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

it("sends the chosen template id when publishing", async () => {
  const mutateAsync = vi.fn().mockResolvedValue({ frame: { name: "vocab" }, version: { version: "1.0.0" } });
  useMutationMock.mockReturnValue({ mutateAsync, isPending: false });
  mockQueries(vocabularyTemplate);
  renderAt("/frames/new?template=builtin:domain-vocabulary");

  await userEvent.type(screen.getByLabelText(/frame name/i), "vocab");
  await userEvent.type(screen.getByLabelText(/^description$/i), "Our terms");
  // The header button only opens the publish dialog; the dialog's own button
  // is what actually submits, so clicking just the first would assert against
  // a form that never published.
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

  expect(mutateAsync).toHaveBeenCalled();
  const arg = mutateAsync.mock.calls[0][0];
  expect(arg.templateId).toBe("builtin:domain-vocabulary");
});

// The import and edit paths sending no template id is covered by
// `publishTemplateID` in src/lib/templates.test.ts. It is asserted there rather
// than here because a test that only checks the payload "if a submit happened"
// passes when nothing happened at all, which proves nothing. The page's job is
// to call the helper; the helper's job is to be right.

it("puts a required-slot violation under that section", async () => {
  const { ConnectError, Code } = await import("@connectrpc/connect");
  const { FieldViolationsSchema } = await import("@gen/frames/v1/frame_service_pb");
  const { create } = await import("@bufbuild/protobuf");
  const detail = create(FieldViolationsSchema, {
    violations: [{ field: "slots.terminology", message: 'required by the "Domain Vocabulary" template' }],
  });
  const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
    { desc: FieldViolationsSchema, value: detail },
  ]);
  useMutationMock.mockReturnValue({ mutateAsync: vi.fn().mockRejectedValue(err), isPending: false });
  mockQueries(vocabularyTemplate);
  renderAt("/frames/new?template=builtin:domain-vocabulary");

  await userEvent.type(screen.getByLabelText(/frame name/i), "vocab");
  await userEvent.type(screen.getByLabelText(/^description$/i), "Our terms");
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

  const message = await screen.findByText(/required by the "Domain Vocabulary" template/i);
  expect(message).toBeInTheDocument();
  // It has to be attached to the Terminology section, not floated to the top of
  // the form: that placement is the whole point of the FieldViolations detail.
  const section = screen.getByRole("heading", { name: /^terminology$/i }).closest("section");
  expect(section).toContainElement(message);
});

it("explains a failed template list instead of drawing an empty picker", async () => {
  // What a dev database missing the frame_templates table actually produces.
  // The built-ins are compiled into the binary, so "no templates" is never the
  // truth here: an empty picker would be the page lying about why it is empty.
  const refetch = vi.fn();
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: new ConnectError("no such table: frame_templates", Code.Internal),
    isFetchedAfterMount: true,
    data: undefined,
    refetch,
  });
  renderAt("/frames/new");

  expect(screen.getByText(/could not be loaded/i)).toBeInTheDocument();
  // A fixed sentence, not the server's text: every other read failure in this
  // app states one, because storage detail is not something a reader can act
  // on and retrying is.
  expect(screen.queryByText(/no such table/i)).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /try again/i }));
  expect(refetch).toHaveBeenCalled();
});

it("offers a way out when the chosen template cannot be read", async () => {
  // A deleted org template, another org's id, or a typo in ?template=. The id
  // stays in the URL and every publish carries it, so the form behind this is
  // a dead end: the server refuses each publish for a template it will not
  // return.
  const refetch = vi.fn();
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        error: new ConnectError('no template "01JGONE"', Code.NotFound),
        isFetchedAfterMount: true,
        data: undefined,
        refetch,
      };
    }
    return templateList;
  });
  renderAt("/frames/new?template=01JGONE");

  expect(screen.queryByLabelText(/frame name/i)).not.toBeInTheDocument();
  expect(screen.getByText(/could not be loaded/i)).toBeInTheDocument();

  // Back to the picker, with the bad id cleared from the URL rather than left
  // for the next publish to carry.
  await userEvent.click(screen.getByRole("button", { name: /choose a different template/i }));
  expect(screen.getByRole("heading", { name: /built-in/i })).toBeInTheDocument();
});

it("keeps an in-progress form when a later fetch of the chosen template fails", async () => {
  // The template loaded once and the author has been typing. A refetch can
  // still fail afterwards - reconnect, a token refresh, the template being
  // deleted in another session - and replacing the form at that point throws
  // away unsaved work for a template that is demonstrably readable.
  let failing = false;
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        isFetchedAfterMount: true,
        data: { template: vocabularyTemplate },
        error: failing ? new ConnectError("connection lost", Code.Unavailable) : null,
        refetch: vi.fn(),
      };
    }
    return templateList;
  });
  const { rerender } = renderAt("/frames/new?template=builtin:domain-vocabulary");
  await userEvent.type(screen.getByLabelText(/frame name/i), "vocab");

  failing = true;
  rerender(
    <MemoryRouter initialEntries={["/frames/new?template=builtin:domain-vocabulary"]}>
      <Routes>
        <Route path="/frames/new" element={<FrameAuthoringPage mode="create" />} />
      </Routes>
    </MemoryRouter>,
  );

  expect(screen.getByLabelText(/frame name/i)).toHaveValue("vocab");
  expect(screen.queryByText(/could not be loaded/i)).not.toBeInTheDocument();
});

it("seeds the create form only from a prefill fetched after this mount", async () => {
  // The mirror of AdminTemplatesPage's stale-seed test. Without it, deleting
  // this page's `isFetchedAfterMount` guard breaks no test at all, even though
  // it is half of the fix for seeding from a cached row.
  let fetched = false;
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return fetched
        ? { isLoading: false, error: null, isFetchedAfterMount: true, data: { template: orgTemplateWithPrefill } }
        : {
            isLoading: false,
            error: null,
            isFetchedAfterMount: false,
            // The cached copy: an earlier prefill for this same id.
            data: { template: { ...orgTemplateWithPrefill, prefill: new TextEncoder().encode("slots:\n  terminology:\n    - term: Stale\n      definition: From the cache.\n") } },
          };
    }
    return templateList;
  });

  const { rerender } = renderAt("/frames/new?template=01JORGTEMPLATE0000000000AB");
  expect(screen.queryByDisplayValue("Stale")).not.toBeInTheDocument();

  fetched = true;
  rerender(
    <MemoryRouter initialEntries={["/frames/new?template=01JORGTEMPLATE0000000000AB"]}>
      <Routes>
        <Route path="/frames/new" element={<FrameAuthoringPage mode="create" />} />
      </Routes>
    </MemoryRouter>,
  );
  expect(await screen.findByDisplayValue("Frame")).toBeInTheDocument();
});


it("does not seed from a cached prefill when the fetch that ran on mount failed", async () => {
  // query-core counts a FAILED fetch in isFetchedAfterMount too
  // (queryObserver: dataUpdateCount > initial || errorUpdateCount > initial),
  // and its error reducer keeps whatever data was already cached. So this is
  // the shape a forced refetch failure produces over a row an admin page had
  // open moments earlier - and seeding from it would silently restore exactly
  // the stale-content bug the guard exists to prevent, then latch and hide the
  // error.
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        isFetchedAfterMount: true,
        error: new ConnectError("connection lost", Code.Unavailable),
        data: {
          template: {
            ...orgTemplateWithPrefill,
            prefill: new TextEncoder().encode("slots:\n  terminology:\n    - term: Stale\n      definition: From the cache.\n"),
          },
        },
        refetch: vi.fn(),
      };
    }
    return templateList;
  });
  renderAt("/frames/new?template=01JORGTEMPLATE0000000000AB");

  expect(screen.queryByDisplayValue("Stale")).not.toBeInTheDocument();
  // Nothing was seeded, so this is still the dead end it looks like.
  expect(screen.getByText(/could not be loaded/i)).toBeInTheDocument();
});

it("treats a template whose stored content will not decode as a dead end", async () => {
  // The fetch succeeds, so nothing is wrong with the id as far as the query is
  // concerned - but nothing was seeded either. Calling that "seeded" would
  // leave the author on a defaults-only form with the id still in the URL and
  // still sent by every publish, whose required-slot check they never saw.
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        error: null,
        isFetchedAfterMount: true,
        data: { template: { ...orgTemplateWithPrefill, prefill: new TextEncoder().encode("slots: [not a mapping\n") } },
        refetch: vi.fn(),
      };
    }
    return templateList;
  });
  renderAt("/frames/new?template=01JORGTEMPLATE0000000000AB");

  expect(await screen.findByText(/could not be loaded/i)).toBeInTheDocument();
  expect(screen.getByText(/starting content could not be read/i)).toBeInTheDocument();
  // The two ways out, same as a failed fetch.
  expect(screen.getByRole("button", { name: /choose a different template/i })).toBeInTheDocument();
  expect(screen.queryByLabelText(/frame name/i)).not.toBeInTheDocument();
});
