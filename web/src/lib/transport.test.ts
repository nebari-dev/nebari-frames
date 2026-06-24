import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { authHeaderInterceptor, refreshOn401Interceptor } from "./transport";

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

describe("refreshOn401Interceptor", () => {
  it("refreshes and retries once on unauthenticated", async () => {
    const refresh = vi.fn(async () => "new-token");
    let calls = 0;
    const next = vi.fn(async (req: { header: Headers }) => {
      calls++;
      if (calls === 1) throw new ConnectError("nope", Code.Unauthenticated);
      return { header: req.header };
    });
    const interceptor = refreshOn401Interceptor(refresh);
    const req = { header: new Headers() };
    await interceptor(next as never)(req as never);
    expect(refresh).toHaveBeenCalledOnce();
    expect(calls).toBe(2);
  });

  it("does not retry on non-auth errors", async () => {
    const refresh = vi.fn();
    const next = vi.fn(async () => {
      throw new ConnectError("boom", Code.Internal);
    });
    const interceptor = refreshOn401Interceptor(refresh);
    await expect(interceptor(next as never)({ header: new Headers() } as never)).rejects.toThrow();
    expect(refresh).not.toHaveBeenCalled();
  });
});
