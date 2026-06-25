import { ConnectError, Code } from "@connectrpc/connect";
import { DeleteBlockedSchema } from "@gen/frames/v1/frame_service_pb";

export interface DeleteErrorResult {
  blockingFrames: string[] | null;
  message: string | null;
}

export function mapDeleteError(err: unknown): DeleteErrorResult {
  const ce = ConnectError.from(err);
  if (ce.code === Code.FailedPrecondition) {
    for (const d of ce.findDetails(DeleteBlockedSchema)) {
      return { blockingFrames: d.blockingFrames, message: null };
    }
  }
  return { blockingFrames: null, message: ce.rawMessage || "Could not delete the frame." };
}
