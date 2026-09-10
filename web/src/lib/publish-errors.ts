import { ConnectError, Code } from "@connectrpc/connect";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";

export interface PublishErrorResult {
  fieldErrors: Record<string, string>;
  formError: string | null;
}

// Reads the FieldViolations details the server attaches, mapping each named
// field to its message. Empty when the error carries no detail.
function violations(ce: ConnectError): Record<string, string> {
  const fieldErrors: Record<string, string> = {};
  for (const fv of ce.findDetails(FieldViolationsSchema)) {
    for (const v of fv.violations) {
      if (v.field) fieldErrors[v.field] = v.message;
    }
  }
  return fieldErrors;
}

export function mapPublishError(err: unknown): PublishErrorResult {
  const ce = ConnectError.from(err);

  if (ce.code === Code.AlreadyExists) {
    // Two collisions share this code: a frame name already taken (which a
    // publish that asserts creation - anything carrying a template id - now
    // reports) and a version already published. They are fixed in different
    // inputs, so the server's own field wins when it names one; the version
    // input stays the default, which is where a bare republish belongs.
    const fieldErrors = violations(ce);
    if (Object.keys(fieldErrors).length > 0) {
      return { fieldErrors, formError: null };
    }
    // No violation means a republished version. A violation carrying an empty
    // field name would land here too and be reported as a version conflict,
    // which would be wrong - the server never sends one (it names "name"), and
    // this is the assumption to revisit if that ever changes.
    return { fieldErrors: { version: ce.rawMessage || "frame version already exists" }, formError: null };
  }

  if (ce.code === Code.InvalidArgument) {
    const fieldErrors = violations(ce);
    if (Object.keys(fieldErrors).length > 0) {
      return { fieldErrors, formError: null };
    }
    return { fieldErrors, formError: ce.rawMessage || "The frame is invalid." };
  }

  return { fieldErrors: {}, formError: ce.rawMessage || "Could not publish the frame." };
}
