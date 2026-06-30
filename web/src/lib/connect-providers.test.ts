import { describe, expect, it } from "vitest";
import { connectProviders, getConnectProvider } from "./connect-providers";

describe("connect-providers", () => {
  it("includes a live Claude.ai provider with steps and a verified date", () => {
    const claude = getConnectProvider("claude");
    expect(claude).toBeDefined();
    expect(claude!.status).toBe("available");
    expect(claude!.steps && claude!.steps.length).toBeGreaterThan(0);
    expect(claude!.lastVerified).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("gives every available provider non-empty steps and a verified date, and any coming-soon provider none", () => {
    for (const p of connectProviders) {
      if (p.status === "available") {
        expect(p.steps && p.steps.length).toBeGreaterThan(0);
        expect(p.lastVerified).toBeTruthy();
      } else {
        expect(p.steps).toBeUndefined();
      }
    }
  });

  it("exposes ChatGPT and Gemini as live providers with steps", () => {
    for (const id of ["chatgpt", "gemini"]) {
      const p = getConnectProvider(id);
      expect(p?.status).toBe("available");
      expect(p?.steps && p.steps.length).toBeGreaterThan(0);
    }
  });

  it("returns undefined for an unknown id", () => {
    expect(getConnectProvider("nope")).toBeUndefined();
  });
});
