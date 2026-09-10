import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi, beforeEach } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";

const navigateMock = vi.fn();
vi.mock("react-router", async (orig) => ({
  ...(await orig<typeof import("react-router")>()),
  useNavigate: () => navigateMock,
}));

// The page issues two distinct mutations; route each to its own mock so a test
// can drive conversion without also standing in for publish.
const { convertMock, publishMock } = vi.hoisted(() => ({
  convertMock: vi.fn(),
  publishMock: vi.fn(),
}));

vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({
    // Rendering past the picker means a template was chosen AND fetched: the
    // page withholds the form until the seed lands, because reset() would
    // otherwise discard anything typed in the meantime. One object serves both
    // readers - each looks at its own key.
    data: {
      org: { slug: "openteams" },
      template: {
        id: "builtin:blank",
        title: "Blank",
        description: "Empty.",
        builtin: true,
        prefill: new TextEncoder().encode("slots: {}\n"),
        fieldRules: {},
      },
    },
    isLoading: false,
    error: null,
    isFetchedAfterMount: true,
  }),
  useMutation: (method: { name?: string }) => ({
    mutate: method?.name === "ConvertFrame" ? convertMock : publishMock,
    isPending: false,
    isSuccess: false,
  }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { FrameAuthoringPage } from "./FrameAuthoringPage";

const encode = (s: string) => new TextEncoder().encode(s);

// Base UI menus do not open from userEvent.click under jsdom; drive them with
// explicit pointer events (see Header.test.tsx).
function pointerClick(el: Element) {
  fireEvent.pointerDown(el);
  fireEvent.pointerUp(el);
  fireEvent.click(el);
}

beforeEach(() => {
  convertMock.mockReset();
  publishMock.mockReset();
});

// The real create path now starts at the template picker; these tests exercise
// the authoring form itself, so they render past it with a template already
// chosen in the URL, same as a user who just picked one.
function renderCreate() {
  return render(
    <MemoryRouter initialEntries={["/frames/new?template=builtin:blank"]}>
      <FrameAuthoringPage mode="create" />
    </MemoryRouter>,
  );
}

// Markdown is a secondary mode behind the overflow menu, not a header toggle.
async function openMarkdownMode() {
  pointerClick(screen.getByRole("button", { name: /more actions/i }));
  pointerClick(await screen.findByRole("menuitem", { name: /edit as markdown/i }));
}

it("Edit as Markdown converts the document and shows the .frame.md source", async () => {
  const md = "---\ntype: frame [0.2]\nname: brand-voice\n---\n\n## Goals\n\n- Ship it.\n";
  convertMock.mockImplementation((_req, opts) => opts.onSuccess({ markdown: encode(md) }));

  renderCreate();
  await userEvent.type(screen.getByLabelText(/frame name/i), "brand-voice");
  await openMarkdownMode();

  await waitFor(() => expect(convertMock).toHaveBeenCalled());
  // The document is converted by sending its YAML, never by re-serializing in TS.
  const req = convertMock.mock.calls[0][0];
  expect(req.source.case).toBe("yaml");
  expect(new TextDecoder().decode(req.source.value)).toMatch(/name: brand-voice/);

  const source = await screen.findByLabelText(/frame markdown source/i);
  expect(source).toHaveValue(md);
});

it("shows a structural error and stays in Markdown when the source will not parse", async () => {
  const fv = create(FieldViolationsSchema, {
    violations: [{ field: "markdown", message: 'line 9: unknown section "## Ways of Working"' }],
  });
  const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
    { desc: FieldViolationsSchema, value: fv },
  ]);

  // First call (document -> markdown) succeeds; the next (markdown -> document) fails.
  convertMock
    .mockImplementationOnce((_req, opts) => opts.onSuccess({ markdown: encode("---\n---\n") }))
    .mockImplementation((_req, opts) => opts.onError(err));

  renderCreate();
  await openMarkdownMode();
  await screen.findByLabelText(/frame markdown source/i);

  await userEvent.click(screen.getByRole("button", { name: /back to editor/i }));

  expect(await screen.findByText(/unknown section "## Ways of Working"/)).toBeInTheDocument();
  // Still in Markdown: the parse must succeed before the document can take over.
  expect(screen.getByLabelText(/frame markdown source/i)).toBeInTheDocument();
});

it("publishing from Markdown converts first, then publishes the canonical YAML", async () => {
  const yaml = "name: brand-voice\ndescription: d\nversion: 1.0.0\nvisibility: internal\nslots: {}\n";
  convertMock
    .mockImplementationOnce((_req, opts) => opts.onSuccess({ markdown: encode("---\n---\n") }))
    .mockImplementation((_req, opts) => opts.onSuccess({ yaml: encode(yaml) }));

  renderCreate();
  await openMarkdownMode();
  await screen.findByLabelText(/frame markdown source/i);

  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  // In Markdown mode the version comes from the frontmatter, so the dialog
  // offers no version input - only the changelog and the confirm.
  expect(screen.queryByPlaceholderText("1.0.0")).not.toBeInTheDocument();
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

  await waitFor(() => expect(publishMock).toHaveBeenCalled());
  expect(new TextDecoder().decode(publishMock.mock.calls[0][0].content)).toBe(yaml);
});

it("carries the chosen template id through a Markdown-mode publish, and none when importing", async () => {
  const yaml = "name: brand-voice\ndescription: d\nversion: 1.0.0\nvisibility: internal\nslots: {}\n";

  // A template was chosen (renderCreate() lands on ?template=builtin:blank);
  // switching to Markdown before publishing must not drop the check that
  // choice opted into. Two conversions happen here: entering Markdown mode
  // (doc -> markdown), then publish's own markdown -> yaml step.
  convertMock
    .mockImplementationOnce((_req, opts) => opts.onSuccess({ markdown: encode("---\n---\n") }))
    .mockImplementationOnce((_req, opts) => opts.onSuccess({ yaml: encode(yaml) }));

  const templated = renderCreate();
  await openMarkdownMode();
  await screen.findByLabelText(/frame markdown source/i);
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

  await waitFor(() => expect(publishMock).toHaveBeenCalled());
  expect(publishMock.mock.calls[0][0].templateId).toBe("builtin:blank");
  templated.unmount();

  // The ?import=1 path lands in Markdown mode with its source document already
  // in hand, never a template: applying a standard the author never picked
  // would be a surprise, same as the document-editor path.
  publishMock.mockClear();
  convertMock.mockImplementationOnce((_req, opts) => opts.onSuccess({ yaml: encode(yaml) }));

  render(
    <MemoryRouter initialEntries={["/frames/new?import=1"]}>
      <FrameAuthoringPage mode="create" />
    </MemoryRouter>,
  );
  await screen.findByLabelText(/frame markdown source/i);
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

  await waitFor(() => expect(publishMock).toHaveBeenCalled());
  expect(publishMock.mock.calls[0][0].templateId).toBe("");
});

it("import mode opens straight into an empty Markdown editor", async () => {
  render(
    <MemoryRouter initialEntries={["/frames/new?import=1"]}>
      <FrameAuthoringPage mode="create" />
    </MemoryRouter>,
  );
  expect(screen.getByRole("heading", { name: /import frame/i })).toBeInTheDocument();
  expect(screen.getByLabelText(/frame markdown source/i)).toHaveValue("");
  // No conversion is needed to start pasting.
  expect(convertMock).not.toHaveBeenCalled();
});
