import { describe, expect, it, vi } from "vitest";
import { authHeaderInterceptor } from "./transport";

describe("authHeaderInterceptor", () => {
  it("attaches a bearer token when present", async () => {
    const interceptor = authHeaderInterceptor(() => "tok-123");
    const next = vi.fn(async (req: { header: Headers }) => ({ header: req.header }));
    const req = { header: new Headers() };
    await interceptor(next as never)(req as never);
    expect(req.header.get("Authorization")).toBe("Bearer tok-123");
  });

  it("omits the header when no token", async () => {
    const interceptor = authHeaderInterceptor(() => undefined);
    const next = vi.fn(async (req: { header: Headers }) => ({ header: req.header }));
    const req = { header: new Headers() };
    await interceptor(next as never)(req as never);
    expect(req.header.get("Authorization")).toBeNull();
  });
});
