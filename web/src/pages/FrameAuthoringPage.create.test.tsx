import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";

const navigateMock = vi.fn();
vi.mock("react-router", async (orig) => ({
  ...(await orig<typeof import("react-router")>()),
  useNavigate: () => navigateMock,
}));

const mutateMock = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useQuery: () => ({ data: { org: { slug: "openteams" } }, isLoading: false, error: null }),
  useMutation: () => ({ mutate: mutateMock, isPending: false, isSuccess: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));

import { FrameAuthoringPage } from "./FrameAuthoringPage";

// Base UI popups (menus) do not open from userEvent.click under jsdom; drive
// them with explicit pointer events (see Header.test.tsx).
function pointerClick(el: Element) {
  fireEvent.pointerDown(el);
  fireEvent.pointerUp(el);
  fireEvent.click(el);
}

// The real create path now starts at the template picker; these tests exercise
// the authoring form itself, so they render past it with a template already
// chosen in the URL, same as a user who just picked one.
function renderCreate() {
  render(
    <MemoryRouter initialEntries={["/frames/new?template=builtin:blank"]}>
      <FrameAuthoringPage mode="create" />
    </MemoryRouter>,
  );
}

async function fillIdentity() {
  await userEvent.type(screen.getByLabelText(/frame name/i), "brand-voice");
  await userEvent.type(screen.getByLabelText(/^description$/i), "desc");
}

// Version and changelog live in the publish dialog, not on the page.
async function publishViaDialog() {
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));
}

it("publishes a filled document and calls the mutation", async () => {
  mutateMock.mockReset();
  renderCreate();
  await fillIdentity();
  await publishViaDialog();
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());
  const arg = mutateMock.mock.calls[0][0];
  const yaml = new TextDecoder().decode(arg.content);
  expect(yaml).toMatch(/name: brand-voice/);
  // A first version defaults to 1.0.0 without the author touching the field.
  expect(yaml).toMatch(/version: 1\.0\.0/);
});

it("writes a spec visibility into the published document", async () => {
  mutateMock.mockReset();
  renderCreate();
  await fillIdentity();
  await publishViaDialog();
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());
  expect(new TextDecoder().decode(mutateMock.mock.calls[0][0].content)).toMatch(
    /visibility: internal/,
  );
});

it("starts with no content sections and adds one from the menu", async () => {
  mutateMock.mockReset();
  renderCreate();
  // The document starts as identity + composition only - no slot editors.
  expect(screen.queryByRole("heading", { name: "Rules" })).not.toBeInTheDocument();

  pointerClick(screen.getByRole("button", { name: /add section/i }));
  pointerClick(await screen.findByRole("menuitem", { name: /rules/i }));

  expect(await screen.findByRole("heading", { name: "Rules" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /add rule/i })).toBeInTheDocument();
});

// Server violations arrive with bracket paths ("slots.rules[0]") while inputs
// register dotted ones ("slots.rules.0"). They must still meet on the input
// that caused them.
it("surfaces a server violation on the slot row that caused it", async () => {
  mutateMock.mockReset();
  const fv = create(FieldViolationsSchema, {
    violations: [{ field: "slots.rules[0]", message: "must not be empty" }],
  });
  const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
    { desc: FieldViolationsSchema, value: fv },
  ]);
  mutateMock.mockImplementation((_req, opts) => opts.onError(err));

  renderCreate();
  await fillIdentity();

  pointerClick(screen.getByRole("button", { name: /add section/i }));
  pointerClick(await screen.findByRole("menuitem", { name: /rules/i }));
  await userEvent.click(await screen.findByRole("button", { name: /add rule/i }));
  // A client-side-valid rule, so the publish reaches the (mocked) server and
  // the violation that comes back is the one being mapped.
  const boxes = screen.getAllByRole("textbox");
  await userEvent.type(boxes[boxes.length - 1], "Some rule");

  await publishViaDialog();
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());

  expect(await screen.findByRole("alert")).toHaveTextContent("must not be empty");
});
