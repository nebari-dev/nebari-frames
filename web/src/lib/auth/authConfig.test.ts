import { afterEach, expect, it, vi } from "vitest";
import { fetchAuthConfig } from "./authConfig";

afterEach(() => vi.restoreAllMocks());

it("maps the backend /auth/config payload", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(
    JSON.stringify({ issuer: "https://idp.example/realm", client_id: "frames-web" }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  )));
  const cfg = await fetchAuthConfig();
  expect(cfg).toEqual({ issuer: "https://idp.example/realm", clientId: "frames-web" });
});
