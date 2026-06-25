import { describe, it, expect } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { FieldViolationsSchema } from "@gen/frames/v1/frame_service_pb";
import { mapPublishError } from "./publish-errors";

function invalidArgWithViolations(pairs: [string, string][]): ConnectError {
  const fv = create(FieldViolationsSchema, {
    violations: pairs.map(([field, message]) => ({ field, message })),
  });
  return new ConnectError("invalid", Code.InvalidArgument, undefined, [
    { desc: FieldViolationsSchema, value: fv },
  ]);
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

  it("maps AlreadyExists to the version field", () => {
    const res = mapPublishError(new ConnectError("frame version already exists", Code.AlreadyExists));
    expect(res.fieldErrors["version"]).toMatch(/already exists/);
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
