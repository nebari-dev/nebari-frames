import { describe, it, expect } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";
import { mapPublishError } from "./publish-errors";

function withViolations(code: Code, message: string, pairs: [string, string][]): ConnectError {
  const fv = create(FieldViolationsSchema, {
    violations: pairs.map(([field, msg]) => ({ field, message: msg })),
  });
  return new ConnectError(message, code, undefined, [{ desc: FieldViolationsSchema, value: fv }]);
}

function invalidArgWithViolations(pairs: [string, string][]): ConnectError {
  return withViolations(Code.InvalidArgument, "invalid", pairs);
}

describe("mapPublishError", () => {
  it("maps FieldViolations to fieldErrors", () => {
    const res = mapPublishError(
      invalidArgWithViolations([
        ["name", "must match [a-z0-9]..."],
        ["slots.terminology[1].definition", "must not be empty"],
      ]),
    );
    expect(res.fieldErrors["name"]).toMatch(/must match/);
    expect(res.fieldErrors["slots.terminology[1].definition"]).toBe("must not be empty");
    expect(res.formError).toBeNull();
  });

  it("maps a bare AlreadyExists to the version field", () => {
    const res = mapPublishError(new ConnectError("frame version already exists", Code.AlreadyExists));
    expect(res.fieldErrors["version"]).toMatch(/already exists/);
  });

  it("maps an AlreadyExists that names a field onto that field", () => {
    // A taken frame name and a republished version share this code, and they
    // are fixed in different inputs. When the server names the field, that
    // wins: routing a name collision to the version input sends the author to
    // change the one thing that cannot help.
    const res = mapPublishError(
      withViolations(Code.AlreadyExists, 'a frame named "brand-voice" already exists; update it instead', [
        ["name", 'a frame named "brand-voice" already exists; update it instead'],
      ]),
    );
    expect(res.fieldErrors["name"]).toMatch(/already exists/);
    expect(res.fieldErrors["version"]).toBeUndefined();
    expect(res.formError).toBeNull();
  });

  it("returns a formError for non-field errors", () => {
    const res = mapPublishError(new ConnectError("edit permission required", Code.PermissionDenied));
    expect(res.formError).toMatch(/permission/i);
    expect(Object.keys(res.fieldErrors)).toHaveLength(0);
  });

  it("returns a generic formError for invalid-arg with no detail", () => {
    const res = mapPublishError(new ConnectError("bad", Code.InvalidArgument));
    expect(res.formError).toBeTruthy();
  });
});
