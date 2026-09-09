import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi, beforeEach } from "vitest";
// The mock factory below routes by RPC descriptor identity, which means it
// references FrameService by name. Import bindings are resolved before any
// module-level code runs (unlike the mock-prefixed const workaround genuinely
// local variables need), so this is safe to reference inside vi.mock's factory.
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Requirement } from "@gen/frames/v1/frame_pb";

const useQueryMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const deleteMock = vi.fn();
// Routed by RPC descriptor OBJECT IDENTITY, not by call order and not by a name
// string. A modulo counter over the call sequence silently binds the wrong mock
// as soon as the component adds a mutation or re-renders, and the test then
// asserts against the wrong spy while still passing. A name string fails
// differently but just as quietly: DescMethod.name is the proto-declared
// PascalCase name ("CreateFrameTemplate"), so a lowerCamelCase comparison never
// matches and every mutation falls through to the default.
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: (...args: unknown[]) => useQueryMock(...args),
  useMutation: (method: unknown) => {
    if (method === FrameService.method.createFrameTemplate) {
      return { mutateAsync: createMock, isPending: false };
    }
    if (method === FrameService.method.updateFrameTemplate) {
      return { mutateAsync: updateMock, isPending: false };
    }
    if (method === FrameService.method.deleteFrameTemplate) {
      return { mutateAsync: deleteMock, isPending: false };
    }
    return { mutateAsync: vi.fn(), isPending: false };
  },
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));

import { AdminTemplatesPage } from "./AdminTemplatesPage";
import { parseFrameContent } from "@/lib/frame-yaml";

const templateListData = {
  canManage: true,
  templates: [
    { id: "builtin:blank", title: "Blank", description: "Empty.", builtin: true },
    { id: "t1", title: "Our House Style", description: "Ours.", builtin: false },
  ],
};

beforeEach(() => {
  createMock.mockResolvedValue({ template: { id: "new" } });
  updateMock.mockResolvedValue({ template: { id: "t1" } });
  deleteMock.mockResolvedValue({});
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: null,
    // The dialog seeds only from a row this mount fetched, so a mock that
    // means "fresh data" has to say so.
    isFetchedAfterMount: true,
    data: templateListData,
  });
});

const renderPage = () => render(<MemoryRouter><AdminTemplatesPage /></MemoryRouter>);

it("lists the org's templates and the built-ins separately", () => {
  renderPage();
  expect(screen.getByText("Our House Style")).toBeInTheDocument();
  expect(screen.getByText("Blank")).toBeInTheDocument();
});

it("offers edit and delete on an org template but not on a built-in", () => {
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  const builtinRow = screen.getByText("Blank").closest("li, tr")!;
  expect(orgRow.querySelector('[data-testid="edit-template"]')).toBeTruthy();
  expect(orgRow.querySelector('[data-testid="delete-template"]')).toBeTruthy();
  // A built-in is compiled into the binary. Offering controls that can only
  // fail would be a lie about what the page can do.
  expect(builtinRow.querySelector('[data-testid="edit-template"]')).toBeFalsy();
  expect(builtinRow.querySelector('[data-testid="delete-template"]')).toBeFalsy();
});

it("creates a template with a title, a description, and a rule", async () => {
  renderPage();
  await userEvent.click(screen.getByRole("button", { name: /new template/i }));
  await userEvent.type(screen.getByLabelText(/^title$/i), "Vocabulary");
  await userEvent.type(screen.getByLabelText(/^description$/i), "Our terms");
  await userEvent.click(screen.getAllByRole("radio", { name: /required/i })[0]);
  await userEvent.click(screen.getByRole("button", { name: /^create$/i }));
  await waitFor(() => expect(createMock).toHaveBeenCalled());
  const arg = createMock.mock.calls[0][0];
  expect(arg.title).toBe("Vocabulary");
  expect(arg.description).toBe("Our terms");
  // The clicked radio is the first "required" on the page, which belongs to
  // the terminology section (SLOT_SECTIONS' first entry). Asserting the exact
  // slot and level - not just that some rule exists - is what would catch a
  // regression that maps every level to the same one, or maps the click to
  // the wrong slot key.
  expect(arg.fieldRules.terminology).toEqual({ level: Requirement.REQUIRED, note: "" });
});

it("asks before deleting", async () => {
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="delete-template"]') as HTMLElement);
  // Deleting a template cannot break an existing Frame, but it is still
  // destructive to the org's standard, so it is confirmed.
  expect(screen.getByText(/frames already created from it are unaffected/i)).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));
  await waitFor(() => expect(deleteMock).toHaveBeenCalledWith({ id: "t1" }));
});

it("surfaces a duplicate-title error on the title field", async () => {
  const { ConnectError, Code } = await import("@connectrpc/connect");
  createMock.mockRejectedValue(
    new ConnectError('a template titled "Vocabulary" already exists in this organization', Code.AlreadyExists),
  );
  renderPage();
  await userEvent.click(screen.getByRole("button", { name: /new template/i }));
  await userEvent.type(screen.getByLabelText(/^title$/i), "Vocabulary");
  await userEvent.type(screen.getByLabelText(/^description$/i), "Our terms");
  await userEvent.click(screen.getByRole("button", { name: /^create$/i }));
  expect(await screen.findByText(/already exists in this organization/i)).toBeInTheDocument();
});

// The row the org template's edit dialog fetches. Carries a suggested parent
// and a field rule so there is something real for an edit to lose: update
// replaces the stored row wholesale, so anything the form does not re-send
// (extends has no editor on this page) must still survive the round trip.
const existingOrgTemplate = {
  id: "t1",
  title: "Our House Style",
  description: "Ours.",
  builtin: false,
  prefill: new TextEncoder().encode(
    "extends:\n  - ref: acme/base\n    version: 1.0.0\nslots: {}\n",
  ),
  fieldRules: { terminology: { level: 2, note: "Keep these." } },
};

// Routes by RPC descriptor identity - the same convention as the mock above,
// but reachable per-test so getFrameTemplate can answer with a real row
// instead of falling through to the list fixture every other test relies on.
function mockRowFetch(template: unknown) {
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return { isLoading: false, error: null, isFetchedAfterMount: true, data: { template } };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateListData };
  });
}

it("carries an existing extends suggestion through an edit that only changes the title", async () => {
  mockRowFetch(existingOrgTemplate);
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);

  const titleInput = await screen.findByLabelText(/^title$/i);
  await userEvent.clear(titleInput);
  await userEvent.type(titleInput, "Renamed Style");
  await userEvent.click(screen.getByRole("button", { name: /^save changes$/i }));

  await waitFor(() => expect(updateMock).toHaveBeenCalled());
  const arg = updateMock.mock.calls[0][0];
  expect(arg.title).toBe("Renamed Style");
  // The whole point: extends is not editable on this page, so the only way it
  // survives an edit (which replaces the stored row wholesale) is by being
  // carried through untouched. Decoding the actual bytes sent, rather than
  // just checking prefill is non-empty, is what makes this assertion mean
  // something.
  const sentDoc = parseFrameContent(arg.prefill);
  expect(sentDoc.extends).toEqual([{ ref: "acme/base", version: "1.0.0" }]);
});

it("seeds the edit form only from a row fetched after the dialog opened", async () => {
  // React Query hands a cached row back synchronously when a query mounts and
  // refetches in the background (invalidated data stays in the cache, and only
  // ACTIVE queries are refetched, so saving does not evict it). A dialog that
  // seeded from that copy and then latched would show pre-edit content on a
  // second open, and Save would write it back over the first edit.
  let fetched = false;
  const freshRow = { ...existingOrgTemplate, title: "Renamed Style" };
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return fetched
        ? { isLoading: false, error: null, isFetchedAfterMount: true, data: { template: freshRow } }
        : { isLoading: false, error: null, isFetchedAfterMount: false, data: { template: existingOrgTemplate } };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateListData };
  });

  const { rerender } = renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);

  // Still loading as far as this dialog is concerned: the only row available
  // is the one that was already in the cache, so no form is shown at all.
  expect(screen.queryByLabelText(/^title$/i)).not.toBeInTheDocument();

  fetched = true;
  rerender(<MemoryRouter><AdminTemplatesPage /></MemoryRouter>);

  await waitFor(() => expect(screen.getByLabelText(/^title$/i)).toHaveValue("Renamed Style"));
});

it("says why the edit form is missing when its row cannot be read", async () => {
  // The dialog waits for a fetch of its own before seeding, so a failure that
  // is not reported would leave the admin looking at a loading skeleton.
  const { ConnectError, Code } = await import("@connectrpc/connect");
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        error: new ConnectError('no template "t1"', Code.NotFound),
        isFetchedAfterMount: true,
        data: undefined,
      };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateListData };
  });
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);

  expect(await screen.findByText(/could not be loaded/i)).toBeInTheDocument();
  // A fixed sentence rather than the server's, matching this page's own list
  // error and every other read failure in the app.
  expect(screen.queryByText(/no template/i)).not.toBeInTheDocument();
  expect(screen.queryByLabelText(/^title$/i)).not.toBeInTheDocument();
});

it("says so when a stored template's content cannot be decoded", async () => {
  // Stored bytes that no longer parse (hand-edited row, a format change).
  // The seed cannot complete, so the form never renders - which is exactly
  // why this message cannot live inside the form.
  mockRowFetch({ ...existingOrgTemplate, prefill: new TextEncoder().encode("slots: [not a mapping\n") });
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);

  expect(await screen.findByText(/could not be read/i)).toBeInTheDocument();
  expect(screen.queryByLabelText(/^title$/i)).not.toBeInTheDocument();
});

it("keeps an in-progress edit when a later row fetch fails", async () => {
  // Same rule as the authoring form: once the dialog has a row and the admin
  // is typing, a failed refetch must not replace their unsaved edit with an
  // error screen.
  const { ConnectError, Code } = await import("@connectrpc/connect");
  let failing = false;
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        isFetchedAfterMount: true,
        data: { template: existingOrgTemplate },
        error: failing ? new ConnectError("connection lost", Code.Unavailable) : null,
      };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateListData };
  });

  const { rerender } = renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);
  const titleInput = await screen.findByLabelText(/^title$/i);
  await userEvent.clear(titleInput);
  await userEvent.type(titleInput, "Renamed Style");

  failing = true;
  rerender(<MemoryRouter><AdminTemplatesPage /></MemoryRouter>);

  expect(screen.getByLabelText(/^title$/i)).toHaveValue("Renamed Style");
  expect(screen.queryByText(/could not be loaded/i)).not.toBeInTheDocument();
});

it("does not seed the edit form from a cached row when the fetch failed, and says which failure it was", async () => {
  // Same query-core shape as the authoring page's test: a failed fetch counts
  // as "fetched after mount" and leaves the cached row in place.
  const { ConnectError, Code } = await import("@connectrpc/connect");
  useQueryMock.mockImplementation((method: unknown) => {
    if (method === FrameService.method.getFrameTemplate) {
      return {
        isLoading: false,
        isFetchedAfterMount: true,
        error: new ConnectError("connection lost", Code.Unavailable),
        data: { template: { ...existingOrgTemplate, title: "Stale Cached Title" } },
      };
    }
    return { isLoading: false, error: null, isFetchedAfterMount: true, data: templateListData };
  });
  renderPage();
  const orgRow = screen.getByText("Our House Style").closest("li, tr")!;
  await userEvent.click(orgRow.querySelector('[data-testid="edit-template"]') as HTMLElement);

  expect(await screen.findByText(/could not be loaded/i)).toBeInTheDocument();
  expect(screen.queryByDisplayValue("Stale Cached Title")).not.toBeInTheDocument();
  // The failure has to be reported as the failure it was: "no content" is a
  // different claim, and asserting the exact sentence is what stops this test
  // from passing whichever one appears.
  expect(screen.getByText(/may have been deleted, or the registry could not be reached/i)).toBeInTheDocument();
  expect(screen.queryByText(/returned no content/i)).not.toBeInTheDocument();
});

it("refuses to save a template whose terminology row is incomplete", async () => {
  // Client-side, per row, rather than a server round trip: serialization drops
  // rows that are entirely empty, so an incomplete row reported by the server
  // can carry an index that no longer matches the form. The authoring form has
  // always held content to these rules; the template dialog now does too.
  // beforeEach re-stubs the mutation mocks but does not clear their call
  // history, so this test counts only its own calls.
  createMock.mockClear();
  renderPage();
  await userEvent.click(screen.getByRole("button", { name: /new template/i }));
  await userEvent.type(screen.getByLabelText(/^title$/i), "Vocabulary");
  await userEvent.type(screen.getByLabelText(/^description$/i), "Our terms");
  await userEvent.click(screen.getByRole("button", { name: /add term/i }));
  // Term filled, definition left blank: a row the admin meant to write and
  // did not finish, which is exactly the row that must not be dropped
  // silently and must not travel to the server either.
  await userEvent.type(screen.getByPlaceholderText("Term"), "Frame");

  await userEvent.click(screen.getByRole("button", { name: /^create$/i }));

  expect(await screen.findByText(/must not be empty/i)).toBeInTheDocument();
  expect(createMock).not.toHaveBeenCalled();
});
