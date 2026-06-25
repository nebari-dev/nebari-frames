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

  it("marks every available provider with non-empty steps and a coming-soon provider with none", () => {
    for (const p of connectProviders) {
      if (p.status === "available") {
        expect(p.steps && p.steps.length).toBeGreaterThan(0);
        expect(p.lastVerified).toBeTruthy();
      } else {
        expect(p.steps).toBeUndefined();
      }
    }
    // at least one coming-soon provider exists (ChatGPT / Gemini)
    expect(connectProviders.some((p) => p.status === "coming-soon")).toBe(true);
  });

  it("returns undefined for an unknown id", () => {
    expect(getConnectProvider("nope")).toBeUndefined();
  });
});
