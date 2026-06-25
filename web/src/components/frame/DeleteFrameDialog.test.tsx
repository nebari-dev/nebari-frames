import { render, screen } from "@testing-library/react";
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
