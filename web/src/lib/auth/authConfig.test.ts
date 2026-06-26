import { afterEach, expect, it, vi } from "vitest";
import { fetchAuthConfig } from "./authConfig";

afterEach(() => vi.restoreAllMocks());

it("maps the backend /auth/config payload (issuer_url -> issuer)", async () => {
  // The backend emits snake_case `issuer_url` / `client_id` (see
  // backend/internal/server/server.go authConfigResponse, and the CLI parser
  // in cli/internal/auth/device_flow.go). The SPA maps issuer_url onto the
  // oidc-client-ts `authority` field, which buildUserManager reads as cfg.issuer.
  vi.stubGlobal("fetch", vi.fn(async () => new Response(
    JSON.stringify({ issuer_url: "https://idp.example/realm", client_id: "frames-web" }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  )));
  const cfg = await fetchAuthConfig();
  expect(cfg).toEqual({ issuer: "https://idp.example/realm", clientId: "frames-web" });
});
