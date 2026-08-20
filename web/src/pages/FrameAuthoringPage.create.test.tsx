import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";
import { FRAME_TEMPLATES } from "@/lib/frame-templates";

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

function renderCreate() {
  render(<MemoryRouter><FrameAuthoringPage mode="create" /></MemoryRouter>);
}

async function fillIdentity() {
  await userEvent.type(screen.getByLabelText(/^name$/i), "brand-voice");
  await userEvent.type(screen.getByLabelText(/^description$/i), "desc");
}

// Version and changelog live in the publish dialog, not on the page.
async function publishViaDialog() {
  await userEvent.click(screen.getByRole("button", { name: /publish…/i }));
  await userEvent.click(await screen.findByRole("button", { name: /^publish$/i }));
}

it("focuses the frame name on a fresh create screen", () => {
  renderCreate();
  expect(screen.getByLabelText(/^name$/i)).toHaveFocus();
});

it("publishes a filled document and calls the mutation", async () => {
  mutateMock.mockReset();
  renderCreate();
  await fillIdentity();
  await userEvent.type(screen.getByLabelText(/^content$/i), "Cite benchmarks.");
  await publishViaDialog();
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());
  const arg = mutateMock.mock.calls[0][0];
  const yaml = new TextDecoder().decode(arg.content);
  expect(yaml).toMatch(/name: brand-voice/);
  expect(yaml).toMatch(/Cite benchmarks\./);
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

// The template picker is a Base UI select: open the trigger, then click the
// option by its visible label rather than driving a native <select>.
async function pickTemplate(optionName: string | RegExp) {
  await userEvent.click(screen.getByRole("combobox", { name: /start from a template/i }));
  await userEvent.click(await screen.findByRole("option", { name: optionName }));
}

it("starts from a template: picking one fills the body and its scope", async () => {
  mutateMock.mockReset();
  renderCreate();
  const body = screen.getByLabelText(/^content$/i);
  expect(body).toHaveValue("");

  const tpl = FRAME_TEMPLATES.find((t) => t.id === "code-review-norms")!;
  await pickTemplate(tpl.label);

  expect(screen.getByLabelText(/^content$/i)).toHaveValue(tpl.body);
  // The template's suggested scope fills the still-empty scope field.
  expect(screen.getByLabelText(/scope/i)).toHaveValue(tpl.scope);
});

it("switching back to Blank clears an unedited template body", async () => {
  mutateMock.mockReset();
  renderCreate();
  const tpl = FRAME_TEMPLATES.find((t) => t.id === "team-norms")!;
  await pickTemplate(tpl.label);
  expect(screen.getByLabelText(/^content$/i)).not.toHaveValue("");

  await pickTemplate("Blank");
  expect(screen.getByLabelText(/^content$/i)).toHaveValue("");
});

// Server violations must land on the field that caused them.
it("surfaces a server violation on the field that caused it", async () => {
  mutateMock.mockReset();
  const fv = create(FieldViolationsSchema, {
    violations: [{ field: "description", message: "must be at most 280 characters" }],
  });
  const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
    { desc: FieldViolationsSchema, value: fv },
  ]);
  mutateMock.mockImplementation((_req, opts) => opts.onError(err));

  renderCreate();
  await fillIdentity();
  await publishViaDialog();
  await waitFor(() => expect(mutateMock).toHaveBeenCalled());

  expect(await screen.findByRole("alert")).toHaveTextContent("must be at most 280 characters");
});
