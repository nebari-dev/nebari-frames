import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyBranding,
  brandLogo,
  FETCH_TIMEOUT_MS,
  getBranding,
  loadBranding,
  safeCssValue,
  sanitizeUrl,
  setBranding,
} from "./branding";

/** The <style> element applyBranding injects, or null when it injected none. */
function brandingStyle(): HTMLStyleElement | null {
  return document.querySelector("style[data-branding]");
}

function brandingCss(): string {
  return brandingStyle()?.textContent ?? "";
}

const originalTitle = "Nebari Frames";

beforeEach(() => {
  document.title = originalTitle;
});

afterEach(() => {
  setBranding(null);
  for (const el of document.querySelectorAll("style[data-branding], link[rel~='icon']")) {
    el.remove();
  }
  vi.unstubAllGlobals();
});

describe("safeCssValue", () => {
  it("keeps ordinary color and length values", () => {
    expect(safeCssValue("oklch(55% 0.19 250)")).toBe("oklch(55% 0.19 250)");
    expect(safeCssValue("#0066cc")).toBe("#0066cc");
    expect(safeCssValue("0.5rem")).toBe("0.5rem");
  });

  it("drops empty and CSS-injecting values", () => {
    for (const value of [
      undefined,
      "",
      "red; background: url(http://evil.example/x)",
      "} body { display: none }",
      'red" onload="alert(1)',
      "url(http://evil.example/x.png)",
      "expression(alert(1))",
      "javascript:alert(1)",
      "red\\",
      "<script>",
    ]) {
      expect(safeCssValue(value)).toBeUndefined();
    }
  });
});

describe("sanitizeUrl", () => {
  it("accepts http(s) URLs, root-relative paths, and base64 image data URIs", () => {
    expect(sanitizeUrl("https://cdn.acme.example/logo.svg")).toBe("https://cdn.acme.example/logo.svg");
    expect(sanitizeUrl("http://assets.acme.test:8080/logo.png")).toBe("http://assets.acme.test:8080/logo.png");
    expect(sanitizeUrl("/logo.svg")).toBe("/logo.svg");
    const dataUri = "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=";
    expect(sanitizeUrl(dataUri)).toBe(dataUri);
  });

  it("rejects anything else", () => {
    for (const value of [
      undefined,
      "",
      "javascript:alert(1)",
      "data:text/html;base64,PHNjcmlwdD4=",
      "data:image/svg+xml,<svg onload='alert(1)'/>",
      "ftp://acme.example/logo.svg",
      "logo.svg",
    ]) {
      expect(sanitizeUrl(value)).toBeUndefined();
    }
  });
});

describe("loadBranding", () => {
  it("fetches, sanitizes, and caches the document", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        title: "Acme Frames",
        logoUrl: "https://cdn.acme.example/logo.svg",
        faviconUrl: "javascript:alert(1)",
      }),
    });
    vi.stubGlobal("fetch", fetchMock);

    const branding = await loadBranding();
    expect(branding.title).toBe("Acme Frames");
    expect(branding.logoUrl).toBe("https://cdn.acme.example/logo.svg");
    // A malformed URL is dropped rather than passed to <link href>.
    expect(branding.faviconUrl).toBeUndefined();

    await loadBranding();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(getBranding()).toEqual(branding);
  });

  it("gives up on a stalled request so the app still mounts", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", (_url: string, init?: RequestInit) => {
      return new Promise((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () =>
          reject(new DOMException("The operation was aborted.", "AbortError")),
        );
      });
    });

    // Assert on the promise before advancing the clock: attaching the handler
    // afterwards would surface the rejection as unhandled.
    const rejected = expect(loadBranding()).rejects.toThrow(/abort/i);
    await vi.advanceTimersByTimeAsync(FETCH_TIMEOUT_MS);

    await rejected;
    expect(getBranding()).toBeNull();
    vi.useRealTimers();
  });

  it("throws when the endpoint fails so the caller can fall back", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    await expect(loadBranding()).rejects.toThrow(/500/);
    expect(getBranding()).toBeNull();
  });
});

describe("applyBranding", () => {
  it("does nothing when the document is empty", () => {
    applyBranding({});
    expect(document.title).toBe(originalTitle);
    expect(brandingStyle()).toBeNull();
    expect(document.querySelector("link[rel~='icon']")).toBeNull();
  });

  it("sets the page title", () => {
    applyBranding({ title: "Acme Frames" });
    expect(document.title).toBe("Acme Frames");
  });

  it("points the favicon at the branded URL", () => {
    applyBranding({ faviconUrl: "/brand-favicon.svg" });
    expect(document.querySelector("link[rel~='icon']")?.getAttribute("href")).toBe(
      "/brand-favicon.svg",
    );
  });

  it("reuses an existing favicon link instead of adding a second one", () => {
    const existing = document.createElement("link");
    existing.rel = "icon";
    existing.type = "image/svg+xml";
    existing.href = "/src/assets/favicon.svg";
    document.head.appendChild(existing);

    applyBranding({ faviconUrl: "https://cdn.acme.example/favicon.png" });

    const links = document.querySelectorAll("link[rel~='icon']");
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe("https://cdn.acme.example/favicon.png");
    // The bundled SVG type hint must not survive onto another image type.
    expect(links[0].hasAttribute("type")).toBe(false);
  });

  it("ignores an unsafe favicon URL", () => {
    applyBranding({ faviconUrl: "javascript:alert(1)" });
    expect(document.querySelector("link[rel~='icon']")).toBeNull();
  });

  it("injects theme tokens as kebab-case custom properties per mode", () => {
    applyBranding({
      theme: {
        light: { primary: "#0066cc", primaryForeground: "#ffffff", radius: "0.5rem" },
        dark: { primary: "#3399ff" },
      },
    });

    const css = brandingCss();
    expect(css).toContain(":root {");
    expect(css).toContain("--primary: #0066cc;");
    expect(css).toContain("--primary-foreground: #ffffff;");
    expect(css).toContain("--radius: 0.5rem;");
    expect(css).toContain(".dark {");
    expect(css).toContain("--primary: #3399ff;");
  });

  it("derives hover and focus tokens from an overridden primary", () => {
    applyBranding({ theme: { light: { primary: "#0066cc" }, dark: { primary: "#3399ff" } } });

    const css = brandingCss();
    expect(css).toContain("--primary-hover: color-mix(in oklab, var(--primary), black 15%);");
    expect(css).toContain("--primary-hover: color-mix(in oklab, var(--primary), white 18%);");
    // Two occurrences: one per mode.
    expect(css.match(/--ring: var\(--primary\);/g)).toHaveLength(2);
  });

  it("keeps an explicitly pinned shade instead of the derived one", () => {
    applyBranding({ theme: { light: { primary: "#0066cc", primaryHover: "#004c99", ring: "#66aaff" } } });

    const css = brandingCss();
    expect(css).toContain("--primary-hover: #004c99;");
    expect(css).toContain("--ring: #66aaff;");
    expect(css).not.toContain("color-mix");
  });

  it("derives nothing when primary is not overridden", () => {
    applyBranding({ theme: { light: { background: "#ffffff" } } });

    const css = brandingCss();
    expect(css).toContain("--background: #ffffff;");
    expect(css).not.toContain("--primary-hover");
    expect(css).not.toContain("--ring");
  });

  it("drops unsafe token values and injects no style when nothing survives", () => {
    applyBranding({ theme: { light: { primary: "red; background: red" } } });
    expect(brandingStyle()).toBeNull();
  });
});

describe("brandLogo", () => {
  it("falls back to the bundled Nebari wordmark when unbranded", () => {
    const light = brandLogo(false);
    const dark = brandLogo(true);
    expect(light.alt).toBe("Nebari");
    expect(light.src).toContain("nebari-logo_light");
    expect(dark.src).toContain("nebari-logo_dark");
  });

  it("uses the branded logos and title", () => {
    setBranding({
      title: "Acme Frames",
      logoUrl: "https://cdn.acme.example/logo.svg",
      logoUrlDark: "https://cdn.acme.example/logo-dark.svg",
    });
    expect(brandLogo(false)).toEqual({
      src: "https://cdn.acme.example/logo.svg",
      alt: "Acme Frames",
    });
    expect(brandLogo(true)).toEqual({
      src: "https://cdn.acme.example/logo-dark.svg",
      alt: "Acme Frames",
    });
  });

  it("falls back to the light logo in dark mode when no dark logo is set", () => {
    setBranding({ logoUrl: "https://cdn.acme.example/logo.svg" });
    expect(brandLogo(true).src).toBe("https://cdn.acme.example/logo.svg");
    // No branded title: the alt text stays the built-in default.
    expect(brandLogo(true).alt).toBe("Nebari");
  });
});
