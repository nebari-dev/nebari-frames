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
