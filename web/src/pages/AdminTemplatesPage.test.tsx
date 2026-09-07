import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi, beforeEach } from "vitest";
// The mock factory below routes by RPC descriptor identity, which means it
// references FrameService by name. Import bindings are resolved before any
// module-level code runs (unlike the mock-prefixed const workaround genuinely
// local variables need), so this is safe to reference inside vi.mock's factory.
import { FrameService } from "@gen/frames/v1/frame_service_pb";

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

beforeEach(() => {
  createMock.mockResolvedValue({ template: { id: "new" } });
  updateMock.mockResolvedValue({ template: { id: "t1" } });
  deleteMock.mockResolvedValue({});
  useQueryMock.mockReturnValue({
    isLoading: false,
    error: null,
    data: {
      canManage: true,
      templates: [
        { id: "builtin:blank", title: "Blank", description: "Empty.", builtin: true },
        { id: "t1", title: "Our House Style", description: "Ours.", builtin: false },
      ],
    },
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
  expect(Object.keys(arg.fieldRules ?? {}).length).toBeGreaterThan(0);
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
