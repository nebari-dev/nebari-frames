// Runtime branding, fetched from the backend at /config.json before the app
// mounts: browser-tab title, header logo, favicon, and theme token overrides.
//
// The backend builds that document from BRANDING_* environment variables and the
// JSON file the Helm chart renders into a ConfigMap (see
// backend/internal/branding), so a deployment is rebranded without rebuilding
// the image. Every field is optional and falls back to the built-in Nebari
// default, so an unbranded deployment looks exactly as it does without any
// branding configured.
//
// Call loadBranding() once before rendering (see main.tsx) and pass the result
// to applyBranding(); afterwards read the cached value with getBranding() or
// brandLogo().

import logoDark from "@/assets/nebari-logo_dark.svg";
import logoLight from "@/assets/nebari-logo_light.svg";
import { env } from "@/lib/env";

/**
 * Theme token overrides keyed by the camelCase token name (e.g.
 * `primaryForeground`). Each is applied at runtime as the kebab-case CSS custom
 * property (`--primary-foreground`), scoped to `:root` (light) or `.dark`.
 * Mirrors the tokens documented as overridable in the chart's values.yaml.
 *
 * This is a compile-time type only: it is erased at runtime, and toCssVars()
 * applies whatever keys the document actually carries. Any token defined in
 * styles.css can therefore be set - but only the ones listed here are
 * supported.
 */
export type ThemeTokens = Partial<
  Record<
    | "primary"
    | "primaryForeground"
    | "primaryHover"
    | "background"
    | "foreground"
    | "card"
    | "cardForeground"
    | "secondary"
    | "secondaryForeground"
    | "muted"
    | "mutedForeground"
    | "accent"
    | "accentForeground"
    | "border"
    | "ring"
    | "radius"
    | "headerBackground"
    | "headerBorder"
    | "headerForeground"
    | "headerActionHover",
    string
  >
>;

export type Branding = {
  /** Browser-tab title. Also used as the logo's alt text. */
  title?: string;
  /** Header logo URL (light mode / default). */
  logoUrl?: string;
  /** Dark-mode header logo URL; falls back to logoUrl, then the built-in. */
  logoUrlDark?: string;
  /** Favicon URL. */
  faviconUrl?: string;
  /** CSS variable overrides applied at runtime, per mode. */
  theme?: { light?: ThemeTokens; dark?: ThemeTokens };
};

// Block CSS injection vectors: rule terminators, braces, HTML chars, quotes,
// backslashes, and url()/expression()/javascript: functions. A token value
// containing any of these is dropped rather than applied.
const UNSAFE_CSS = /[;<>{}"'\\]|url\s*\(|expression\s*\(|javascript:/i;

/** Returns the value unchanged if it is a safe CSS token, otherwise undefined. */
export function safeCssValue(value: string | undefined): string | undefined {
  return value && !UNSAFE_CSS.test(value) ? value : undefined;
}

// Image MIME types accepted as inline data: URIs (only when base64-encoded).
const ALLOWED_DATA_MIME_TYPES = new Set([
  "image/png",
  "image/jpeg",
  "image/jpg",
  "image/svg+xml",
  "image/webp",
  "image/gif",
  "image/x-icon",
  "image/vnd.microsoft.icon",
]);

// Accept only non-empty, well-formed http(s) URLs, root-relative paths, or
// base64-encoded data: image URIs from the allow-list above; anything else
// (including "") becomes undefined so a bad value can't land in an <img src> or
// <link href>. Note a cross-origin logo also needs its origin in the CSP
// img-src, which the backend adds from the same configuration.
export function sanitizeUrl(value: string | undefined): string | undefined {
  if (!value) {
    return undefined;
  }
  if (value.startsWith("/")) {
    return value;
  }
  try {
    const { protocol } = new URL(value);
    if (protocol === "http:" || protocol === "https:") {
      return value;
    }
    if (protocol === "data:") {
      // Only accept base64-encoded images whose MIME type is on the allow-list;
      // reject data:text/html, non-base64 payloads, and anything else.
      const match = value.match(/^data:([^;,]+)(;base64)?,/);
      if (!match) {
        return undefined;
      }
      const mime = match[1].toLowerCase();
      const isBase64 = Boolean(match[2]);
      return isBase64 && ALLOWED_DATA_MIME_TYPES.has(mime) ? value : undefined;
    }
    return undefined;
  } catch {
    return undefined;
  }
}

let cached: Branding | null = null;

/**
 * Seeds the cached branding, dropping malformed logo/favicon URLs. Called by
 * loadBranding(); exported so tests can set branding without a network fetch.
 * Pass null to clear it.
 */
export function setBranding(branding: Branding | null): void {
  cached =
    branding === null
      ? null
      : {
          ...branding,
          logoUrl: sanitizeUrl(branding.logoUrl),
          logoUrlDark: sanitizeUrl(branding.logoUrlDark),
          faviconUrl: sanitizeUrl(branding.faviconUrl),
        };
}

// The app waits on this request before mounting, so a stalled backend would
// otherwise leave a blank page indefinitely. Branding is cosmetic: past this
// budget, give up and let the app render its built-in defaults.
export const FETCH_TIMEOUT_MS = 3000;

/** Fetches and caches /config.json. The network request happens at most once. */
export async function loadBranding(): Promise<Branding> {
  if (cached) {
    return cached;
  }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const res = await fetch(`${env.backendBaseUrl}/config.json`, { signal: controller.signal });
    if (!res.ok) {
      throw new Error(`branding request failed: ${res.status}`);
    }
    setBranding((await res.json()) as Branding);
    return cached ?? {};
  } finally {
    clearTimeout(timeout);
  }
}

/** Returns the cached branding, or null if loadBranding() has not resolved. */
export function getBranding(): Branding | null {
  return cached;
}

/**
 * Resolves the header/auth-screen logo for the current mode: the branded
 * dark-mode logo, then the branded default, then the bundled Nebari wordmark.
 * The alt text follows the branded title so a rebranded app doesn't announce
 * itself as Nebari.
 */
export function brandLogo(isDarkMode: boolean): { src: string; alt: string } {
  const branding = getBranding();
  return {
    src: isDarkMode
      ? (branding?.logoUrlDark ?? branding?.logoUrl ?? logoDark)
      : (branding?.logoUrl ?? logoLight),
    alt: branding?.title || "Nebari",
  };
}

const toKebab = (s: string) => s.replace(/([A-Z])/g, "-$1").toLowerCase();

// Tokens the stylesheet hard-codes to a Nebari-magenta shade of --primary /
// --ring. Overriding only `primary` would otherwise leave button hover states
// and focus rings flashing magenta on a rebranded app, so they are derived from
// the override unless the deployer pinned a shade explicitly. `mix` is the
// hover step: the Nebari light hover is a 15% darkening of --primary, the dark
// hover an 18% lightening. Mixed in oklab (not oklch) because black and white
// are achromatic and would drag the hue during polar interpolation.
const DERIVED_TOKENS: Record<"light" | "dark", Array<[string, string, keyof ThemeTokens]>> = {
  light: [
    ["primary-hover", "color-mix(in oklab, var(--primary), black 15%)", "primary"],
    ["ring", "var(--primary)", "primary"],
  ],
  dark: [
    ["primary-hover", "color-mix(in oklab, var(--primary), white 18%)", "primary"],
    ["ring", "var(--primary)", "primary"],
  ],
};

/** Renders a token map to CSS declarations, dropping empty/unsafe values. */
function toCssVars(tokens: ThemeTokens, mode: "light" | "dark"): string {
  const explicit = Object.entries(tokens)
    .map(([k, v]) => [toKebab(k), safeCssValue(v)] as const)
    .filter((entry): entry is readonly [string, string] => entry[1] !== undefined);

  const declared = new Set(explicit.map(([k]) => k));
  const derived = DERIVED_TOKENS[mode]
    .filter(([name, , from]) => !declared.has(name) && declared.has(toKebab(from)))
    .map(([name, value]) => [name, value] as const);

  return [...explicit, ...derived].map(([k, v]) => `  --${k}: ${v};`).join("\n");
}

/**
 * Applies branding to the document before React mounts: page title, favicon,
 * and theme token overrides. Each field falls back to the built-in Nebari
 * default when unset, so an empty branding document (the default) leaves the app
 * visually identical.
 *
 * Theme overrides are injected as a <style> appended last to <head> so they win
 * the cascade over the base tokens in styles.css.
 */
export function applyBranding(branding: Branding): void {
  if (branding.title) {
    document.title = branding.title;
  }

  const faviconUrl = sanitizeUrl(branding.faviconUrl);
  if (faviconUrl) {
    const link = (document.querySelector("link[rel~='icon']") ??
      Object.assign(document.createElement("link"), { rel: "icon" })) as HTMLLinkElement;
    link.href = faviconUrl;
    // Vite emits the built-in favicon with a type="image/svg+xml" hint; a
    // branded URL may be any image type, so let the server's content type win.
    link.removeAttribute("type");
    document.head.appendChild(link);
  }

  if (branding.theme) {
    let css = "";
    for (const mode of ["light", "dark"] as const) {
      const tokens = branding.theme[mode];
      if (!tokens) continue;
      const vars = toCssVars(tokens, mode);
      if (vars) {
        css += `${mode === "light" ? ":root" : ".dark"} {\n${vars}\n}\n`;
      }
    }
    if (css) {
      const style = document.createElement("style");
      style.setAttribute("data-branding", "");
      style.textContent = css;
      document.head.appendChild(style);
    }
  }
}
