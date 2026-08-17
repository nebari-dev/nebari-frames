---
title: Configuration
---

Full reference for `chart/values.yaml`, grouped by area. See [Installation](/installation/) for how these combine into the three auth modes and the NebariApp integration.

## Image

| Key | Default | Description |
| --- | --- | --- |
| `image.repository` | `ghcr.io/nebari-dev/nebari-frames` | Image repository. |
| `image.tag` | `""` | Image tag. Empty defaults to the chart `appVersion`. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |

## Replicas

| Key | Default | Description |
| --- | --- | --- |
| `replicaCount` | `1` | Pod replicas. SQLite is single-writer - **do not raise this**. |

## Persistence

| Key | Default | Description |
| --- | --- | --- |
| `persistence.enabled` | `true` | Use a PersistentVolumeClaim for the database. If `false`, storage is ephemeral and data is lost on pod restart. |
| `persistence.mountPath` | `/data` | Mount path for the data volume. |
| `persistence.dbFile` | `nebari-frames.db` | SQLite filename under the mount path. |
| `persistence.size` | `1Gi` | PVC size. |
| `persistence.storageClass` | `""` | StorageClass for the PVC. Empty uses the cluster default; set explicitly in production. |
| `persistence.accessMode` | `ReadWriteOnce` | PVC access mode. |

## Branding

The app ships with built-in Nebari branding (title, logos, favicon, theme colors)
and needs no configuration. Operators can rebrand it **without rebuilding the
image**: the backend serves a runtime configuration document at `/config.json`, and the
SPA applies it before it mounts (title, favicon, theme CSS variables) and in the
header and sign-in screens (logo).

| Key | Default | Description |
| --- | --- | --- |
| `branding.title` | `""` | Browser-tab title, also used as the logo's alt text. Empty keeps `Nebari Frames`. |
| `branding.logoUrl` | `""` | Header and sign-in logo (light mode / default). Absolute `http(s)` URL, root-relative path, or base64 `data:` image URI. |
| `branding.logoUrlDark` | `""` | Dark-mode logo. Falls back to `logoUrl`, then the built-in dark wordmark. |
| `branding.faviconUrl` | `""` | Favicon URL. |
| `branding.theme.light` | `{}` | Theme token overrides for light mode (see below). |
| `branding.theme.dark` | `{}` | Theme token overrides for dark mode. |

Every field is optional, and each one falls back to its built-in default
independently, so an unbranded install renders exactly as it does without this
block - the chart doesn't even create the ConfigMap.

Supported theme tokens: `primary`, `primaryForeground`, `primaryHover`,
`background`, `foreground`, `card`, `cardForeground`, `secondary`,
`secondaryForeground`, `muted`, `mutedForeground`, `accent`, `accentForeground`,
`border`, `ring`, `radius`, `headerBackground`, `headerBorder`,
`headerForeground`, `headerActionHover`. Each is applied as the kebab-case CSS
custom property (`primaryForeground` → `--primary-foreground`).

`primaryHover` (button and badge hover/active) and `ring` (focus rings) are
**derived from `primary`** when you don't set them, so overriding `primary` alone
keeps hover and focus states on-brand instead of flashing Nebari magenta. Set
them explicitly only to pin a specific shade.

Token keys are written to CSS as-is - no allow-list is enforced when the chart
renders the document or when the SPA applies it - so any other token the SPA
defines can technically be set. Only the tokens listed above are supported.

### Kubernetes / Helm

Set `branding` in values. The chart renders the document into the
`<release>-nebari-frames-config` ConfigMap and mounts it into the pod at
`/etc/nebari-frames/config/config.json`:

```yaml
branding:
  title: "Acme Frames"
  logoUrl: "https://cdn.acme.example/logo.svg"
  logoUrlDark: "https://cdn.acme.example/logo-dark.svg"
  faviconUrl: "https://cdn.acme.example/favicon.svg"
  theme:
    light:
      primary: "oklch(55% 0.19 250)"
      primaryForeground: "#ffffff"
    dark:
      primary: "oklch(62% 0.21 250)"
```

A branding-only `helm upgrade` rolls the pod automatically: the pod template
carries a checksum of the rendered ConfigMap, and the app reads the document once
at startup.

### Outside Kubernetes

Running the image (or the binary) directly, branding resolves per field, highest
first:

1. `BRANDING_*` environment variables:

   | Env var | Field |
   | --- | --- |
   | `BRANDING_TITLE` | `title` |
   | `BRANDING_LOGO_URL` | `logoUrl` |
   | `BRANDING_LOGO_URL_DARK` | `logoUrlDark` |
   | `BRANDING_FAVICON_URL` | `faviconUrl` |
   | `BRANDING_THEME` | `theme` (raw JSON, e.g. `'{"light":{"primary":"#0066cc"},"dark":{}}'`) |

2. The JSON document at `BRANDING_CONFIG_FILE` (what the chart mounts).
3. Built-in Nebari defaults for anything still unset.

```bash
docker run -p 8080:8080 \
  -e FRAMES_DEV_MODE=true \
  -e BRANDING_TITLE="Acme Frames" \
  -e BRANDING_LOGO_URL=https://cdn.acme.example/logo.svg \
  ghcr.io/nebari-dev/nebari-frames
```

Branding never blocks startup: an unreadable config file or invalid
`BRANDING_THEME` JSON is logged as a warning and skipped, and every other field
still applies.

### Security

Theme values are validated in the browser before they are applied: any value
containing CSS-injection characters (`;`, `{`, `}`, `<`, `>`, quotes, backslash,
`url(`, `expression(`, `javascript:`) is dropped instead of injected into the
stylesheet. Logo and favicon URLs are restricted to `http(s)` URLs,
root-relative paths, and base64-encoded `data:` image URIs. A cross-origin logo
or favicon host is added to the app's `Content-Security-Policy` `img-src`
automatically - only the hosts that branding actually configures.

## Seed (first org and admin)

| Key | Default | Description |
| --- | --- | --- |
| `seed.orgSlug` | `""` | Slug for the seeded organization. |
| `seed.orgDisplayName` | `""` | Display name for the seeded organization. |
| `seed.adminEmail` | `""` | Email of the first admin, reconciled to their OIDC subject on first login. |

## Auth modes

Auth mode is selected fail-closed (mirrors the backend's own startup check):

```
nebariapp.enabled && nebariapp.auth.enabled -> OIDC env from the operator-provisioned secret
else auth.devMode == true                   -> FRAMES_DEV_MODE=true (no auth)
else                                         -> auth.oidc.* (self-managed OIDC)
```

| Key | Default | Description |
| --- | --- | --- |
| `auth.devMode` | `false` | When `true` (and NebariApp auth is off), disables auth and uses a fixed dev identity. Local use only. |
| `auth.oidc.issuerUrl` | `""` | OIDC issuer URL for self-managed auth. |
| `auth.oidc.clientId` | `""` | OIDC client id for the SPA. |
| `auth.oidc.deviceClientId` | `""` | OIDC client id for the device-code flow (CLI login). |

## NebariApp

| Key | Default | Description |
| --- | --- | --- |
| `nebariapp.enabled` | `false` | Create a `NebariApp` so the nebari-operator provisions routing, TLS, and OIDC. |
| `nebariapp.hostname` | `""` | Hostname the operator routes to the app. |
| `nebariapp.gateway` | `public` | Gateway the operator attaches the route to. |
| `nebariapp.routing.routes` | `[{pathPrefix: /, pathType: PathPrefix}]` | Routes the operator creates the HTTPRoute from. Without this the operator reports `RoutingNotConfigured` and the hostname is unreachable. |
| `nebariapp.routing.tls.enabled` | `true` | Whether the operator provisions a certificate for the hostname. Without this the operator reports `TLSDisabled`. |
| `nebariapp.landingPage.enabled` | `true` | Show a tile on the Nebari landing page. |
| `nebariapp.landingPage.displayName` | `Frames` | Tile display name. |
| `nebariapp.landingPage.description` | `Reusable context Frames for AI assistants, with a remote MCP endpoint.` | Tile description. |
| `nebariapp.landingPage.category` | `Platform` | Tile category. |
| `nebariapp.auth.enabled` | `true` | When `true`, the operator provisions OIDC clients and the app uses them (see Auth modes above). |
| `nebariapp.auth.scopes` | `[openid, profile, email]` | OIDC scopes requested. |

## MCP

| Key | Default | Description |
| --- | --- | --- |
| `mcp.enabled` | `true` | Mount the `/mcp` endpoint when a public URL is derivable. |
| `mcp.publicUrl` | `""` | Override the MCP public URL. Defaults to `https://<nebariapp.hostname>` when `nebariapp.enabled` is true. |
