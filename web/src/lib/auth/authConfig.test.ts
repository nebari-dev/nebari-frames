import { afterEach, expect, it, vi } from "vitest";
import { fetchAuthConfig } from "./authConfig";

afterEach(() => vi.restoreAllMocks());

it("maps the backend /auth/config payload (issuer_url -> issuer)", async () => {
  // The backend emits snake_case `issuer_url` / `client_id` (see
  // backend/internal/server/server.go authConfigResponse, and the CLI parser
  // in cli/internal/auth/device_flow.go). The SPA maps issuer_url onto the
  // oidc-client-ts `authority` field, which buildUserManager reads as cfg.issuer.
  vi.stubGlobal("fetch", vi.fn(async () => new Response(
    JSON.stringify({ enabled: true, issuer_url: "https://idp.example/realm", client_id: "frames-web" }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  )));
  const cfg = await fetchAuthConfig();
  expect(cfg).toEqual({ enabled: true, issuer: "https://idp.example/realm", clientId: "frames-web" });
});

it("reports disabled auth (dev mode) with empty issuer/clientId", async () => {
  // FRAMES_DEV_MODE: backend returns only { enabled: false }; the URLs are
  // omitted, so the SPA must not try to construct an OIDC UserManager.
  vi.stubGlobal("fetch", vi.fn(async () => new Response(
    JSON.stringify({ enabled: false }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  )));
  const cfg = await fetchAuthConfig();
  expect(cfg).toEqual({ enabled: false, issuer: "", clientId: "" });
});
