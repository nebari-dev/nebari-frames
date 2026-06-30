# nebari-frames

## Overview

Nebari Frames is a single-binary server that ships the web UI (a static SPA) and the API in one image. It stores its data in SQLite on a persistent volume, so a single pod holds the whole application and its database. On a Nebari cluster it integrates through a NebariApp resource, which lets the Nebari operator provision routing, TLS, and OIDC for it. Postgres support (for high availability and more than one replica) is a planned follow-on; for now the chart is single-replica by design.

## Build the image

Build and tag the image, then push it to a registry your cluster can pull from:

```bash
make image IMAGE_TAG=<tag>
docker push ghcr.io/nebari-dev/nebari-frames:<tag>
```

Point the chart at that tag with `image.tag` in your values file (the example values use `tag: dev`).

## Install on Nebari

Label the target namespace so the Nebari operator manages it, then install with the Nebari example values:

```bash
kubectl create namespace nebari-frames
kubectl label namespace nebari-frames nebari.dev/managed=true --overwrite
helm install frames . -n nebari-frames -f examples/nebari-values.yaml
```

With `nebariapp.enabled: true`, the operator provisions the route, TLS certificate, and OIDC clients for you, and writes a secret named `<release>-nebari-frames-oidc-client`. The Deployment reads its OIDC settings (issuer URL, SPA client id, device client id) from that secret.

Expect the pod to report NotReady at first. The readiness probe (`/readyz`) returns 503 until OIDC discovery against the issuer succeeds. This is normal during startup while the operator finishes wiring up the OIDC client; the pod goes Ready once discovery works.

## Prerequisites

- A current-release NIC dev cluster with the nebari-operator, Envoy Gateway, cert-manager, and Keycloak.
- The target namespace labeled `nebari.dev/managed=true`.
- For the MCP endpoint with Claude: the realm must have Dynamic Client Registration enabled and an audience mapper on a default client scope stamping `aud=https://<host>/mcp`. See `docs/connect/keycloak-setup.md`.

## First admin

Set `seed.adminEmail` to the email of your first administrator:

```yaml
seed:
  adminEmail: admin@example.com
```

On startup this creates a pending admin invite keyed to that email. The first time someone logs in through OIDC with a matching email, the invite is reconciled to their real OIDC subject and they become the admin. You do not need to look up a Keycloak `sub` ahead of time.

## Standalone install

For local or demo use without a Nebari cluster, install with the standalone values:

```bash
helm install frames . -n nebari-frames -f examples/standalone-values.yaml
```

This runs in dev mode with authentication disabled and is meant for local use only. In dev mode the identity is fixed to `dev-user` / `dev@localhost`, so `seed.adminEmail` has no effect here. It only takes effect once real OIDC is configured (either through NebariApp or a self-managed `auth.oidc.*` block).

## Known Limitations (Alpha)

- MCP/Claude requires manual Keycloak realm config (DCR + default-scope audience mapper); operator-native support is pending.
- Wrong-audience rejection and RBAC-negative read isolation are verified in automated/local tests but are not gated in the live Alpha demo.
- No CI yet; the container image is built and pushed manually.
- The chart is not yet published to the Nebari helm-repository; ArgoCD syncs it from the git repository.
- SQLite is single-writer: `replicaCount` must stay 1.

## Values reference

| Key | Description |
| --- | --- |
| `image.repository` | Image repository. Defaults to `ghcr.io/nebari-dev/nebari-frames`. |
| `image.tag` | Image tag. Empty defaults to the chart `appVersion`. |
| `image.pullPolicy` | Image pull policy. Defaults to `IfNotPresent`. |
| `replicaCount` | Pod replicas. Must stay `1`: SQLite is single-writer. |
| `persistence.enabled` | Use a PersistentVolumeClaim for the database. If `false`, storage is ephemeral and data is lost when the pod restarts. |
| `persistence.mountPath` | Mount path for the data volume. Defaults to `/data`. |
| `persistence.dbFile` | SQLite filename under the mount path. Defaults to `nebari-frames.db`. |
| `persistence.size` | PVC size. Defaults to `1Gi`. |
| `persistence.storageClass` | StorageClass for the PVC. Defaults to `""`, which uses the cluster default class. Set this explicitly in production so you control which class backs the volume. |
| `persistence.accessMode` | PVC access mode. Defaults to `ReadWriteOnce`. |
| `seed.orgSlug` | Slug for the seeded organization. |
| `seed.orgDisplayName` | Display name for the seeded organization. |
| `seed.adminEmail` | Email of the first admin (reconciled to the OIDC subject on first login). |
| `auth.devMode` | When `true` (and NebariApp auth is off), disables auth and uses a fixed dev identity. Local use only. |
| `auth.oidc.issuerUrl` | OIDC issuer URL for self-managed auth (used when NebariApp is off and dev mode is off). |
| `auth.oidc.clientId` | OIDC client id for the SPA. |
| `auth.oidc.deviceClientId` | OIDC client id for the device-code flow (CLI login). |
| `nebariapp.enabled` | When `true`, create a NebariApp so the operator provisions routing, TLS, and OIDC. |
| `nebariapp.hostname` | Hostname the operator routes to the app. |
| `nebariapp.gateway` | Gateway the operator attaches the route to. Defaults to `public`. |
| `nebariapp.auth.enabled` | When `true`, the operator provisions OIDC clients and the app uses them. |
| `nebariapp.auth.scopes` | OIDC scopes requested. Defaults to `openid`, `profile`, `email`. |
| `mcp.enabled` | Mount the `/mcp` endpoint when a public URL is derivable | `true` |
| `mcp.publicUrl` | Override the MCP public URL (else `https://<nebariapp.hostname>`) | `""` |

Auth mode is chosen fail-closed: if `nebariapp.enabled` and `nebariapp.auth.enabled` are both true, the app uses the operator-provided OIDC secret. Otherwise, if `auth.devMode` is true, it runs with no auth. Otherwise it uses the self-managed `auth.oidc.*` values.

## Local e2e

The `dev/Makefile` runs an end-to-end loop against a local kind cluster. Run the targets from the `dev/` directory:

- `make up` - create the kind cluster (bring up the full Nebari stack via the operator dev scripts, then run `make image-load install`).
- `make up-standalone` - create the cluster and install in standalone mode in one step.
- `make image-load` - build the image and load it into the kind cluster.
- `make install` - install the chart with the Nebari values (labels the namespace `nebari.dev/managed=true`).
- `make install-standalone` - install the chart with the standalone values.
- `make down` - delete the kind cluster.

## Storage and scaling

The database is SQLite, which has a single writer. The chart pins `replicaCount: 1` and uses the `Recreate` deployment strategy so the old pod releases the volume before the new one mounts it. Do not raise the replica count on this slice; a second replica would corrupt the database.

Running more than one replica needs a shared database. Postgres support (high availability with `replicas > 1`) is the next slice of work and is not in this release.
