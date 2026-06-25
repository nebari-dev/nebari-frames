import { ConnectError, Code } from "@connectrpc/connect";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";

export interface PublishErrorResult {
  fieldErrors: Record<string, string>;
  formError: string | null;
}

export function mapPublishError(err: unknown): PublishErrorResult {
  const ce = ConnectError.from(err);
  const fieldErrors: Record<string, string> = {};

  if (ce.code === Code.AlreadyExists) {
    return { fieldErrors: { version: ce.rawMessage || "frame version already exists" }, formError: null };
  }

  if (ce.code === Code.InvalidArgument) {
    for (const fv of ce.findDetails(FieldViolationsSchema)) {
      for (const v of fv.violations) {
        if (v.field) fieldErrors[v.field] = v.message;
      }
    }
    if (Object.keys(fieldErrors).length > 0) {
      return { fieldErrors, formError: null };
    }
    return { fieldErrors, formError: ce.rawMessage || "The frame is invalid." };
  }

  return { fieldErrors, formError: ce.rawMessage || "Could not publish the frame." };
}
