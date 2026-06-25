import { render, screen, act } from "@testing-library/react";
import { useState } from "react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { DeleteBlockedSchema } from "@gen/frames/v1/frame_service_pb";

const mutate = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
  useMutation: () => ({ mutate, isPending: false }),
  createConnectQueryKey: () => ["k"],
}));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));
vi.mock("react-router", () => ({ useNavigate: () => vi.fn() }));

import { DeleteFrameDialog } from "./DeleteFrameDialog";

it("blocks then offers force when the frame is a parent", async () => {
  // first mutate call -> onError with a block detail
  mutate.mockImplementationOnce((_vars: unknown, opts: { onError: (err: unknown) => void }) => {
    const detail = create(DeleteBlockedSchema, { blockingFrames: ["openteams/child"] });
    opts.onError(new ConnectError("blocked", Code.FailedPrecondition, undefined, [{ desc: DeleteBlockedSchema, value: detail }]));
  });
  render(<DeleteFrameDialog org="openteams" name="parent" open onOpenChange={() => {}} onDeleted={() => {}} />);
  await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));
  expect(screen.getByText(/openteams\/child/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /delete anyway/i })).toBeInTheDocument();
});

it("resets to initial confirm prompt after close then reopen", async () => {
  // first mutate call -> onError with a block detail
  mutate.mockImplementationOnce((_vars: unknown, opts: { onError: (err: unknown) => void }) => {
    const detail = create(DeleteBlockedSchema, { blockingFrames: ["openteams/child"] });
    opts.onError(new ConnectError("blocked", Code.FailedPrecondition, undefined, [{ desc: DeleteBlockedSchema, value: detail }]));
  });

  // Stateful wrapper: onOpenChange drives React state so rerenders are real
  let externalSetOpen!: (v: boolean) => void;
  const Wrapper = () => {
    const [open, setOpen] = useState(true);
    externalSetOpen = setOpen;
    return (
      <DeleteFrameDialog
        org="openteams"
        name="parent"
        open={open}
        onOpenChange={setOpen}
        onDeleted={() => {}}
      />
    );
  };

  render(<Wrapper />);

  // Trigger block error - force screen appears
  await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));
  expect(screen.getByRole("button", { name: /delete anyway/i })).toBeInTheDocument();

  // Click the "Close" button - this calls DialogClose's onClose which is handleOpenChange(false).
  // handleOpenChange(false) resets blocking/error state, then calls onOpenChange(false) = setOpen(false).
  await userEvent.click(screen.getByRole("button", { name: /^close$/i }));

  // Reopen by driving the wrapper's state directly
  act(() => externalSetOpen(true));

  // Should show initial confirm screen, NOT the force screen
  expect(screen.queryByRole("button", { name: /delete anyway/i })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /^delete$/i })).toBeInTheDocument();
});
