import { ConnectError, Code } from "@connectrpc/connect";
import { expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { DeleteBlockedSchema } from "@gen/frames/v1/frame_service_pb";
import { mapDeleteError } from "./delete-errors";

it("extracts blocking frames from a FailedPrecondition detail", () => {
  const detail = create(DeleteBlockedSchema, { blockingFrames: ["openteams/child"] });
  const err = new ConnectError("blocked", Code.FailedPrecondition, undefined, [
    { desc: DeleteBlockedSchema, value: detail },
  ]);
  const res = mapDeleteError(err);
  expect(res.blockingFrames).toEqual(["openteams/child"]);
});

it("returns a message for a generic error", () => {
  const res = mapDeleteError(new ConnectError("nope", Code.Internal));
  expect(res.blockingFrames).toBeNull();
  expect(res.message).toBeTruthy();
});
