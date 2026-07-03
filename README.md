# Nebari Frames

> **Status:** beta ([software pack maturity](https://github.com/nebari-dev/software-pack-template/blob/main/docs/release-readiness-checklist.md)). Backend, CLI, web app, and MCP endpoint are implemented. See [Run locally](#run-locally) to try it. Design docs in [`docs/design/`](docs/design/).

Nebari Frames is the registry and exchange for **Frames**: scoped, text-based artifacts that carry organizational context (terminology, style, goals, rules, business processes, and more) into AI conversations. A Frame composes through inheritance, is governed through role-based access control, and is consumable by any MCP-capable AI client (Claude, ChatGPT, Gemini, and others) or by Claude Code through file install.

This project is the successor to [`skillsctl`](https://github.com/nebari-dev/skillsctl), which it borrows registry foundations from. `skillsctl` continues to ship as the Claude Code skill registry; new investment goes here.

## Why Frames

Enterprise AI adoption is gated less by model capability and more by the organizational context that turns a generic model into a specialized worker: the brand voice, the compliance constraints, the named concepts, the team norms. Today that context lives in style guides, wikis, Slack history, and the heads of senior employees. Frames make it explicit, portable, inheritable, and governable - a first-class artifact that an organization owns and shares on its own terms.

See [Background §1.1 in the migration design doc](docs/design/2026-05-21-nebari-frames-migration.md#11-the-whitepaper-in-one-paragraph) for the broader framing from the OpenTeams *Intelligence Hub Whitepaper - v4*.

## Run locally

**Prerequisites:** Go 1.25.7+, Node 16+ (Docker also required for `make dev-auth`).

### Fast UI loop - `make dev`

```bash
make dev
```

Runs the backend (dev mode, no OIDC) on `:8080` and the Vite dev server on `:5173`, seeded with representative sample data (an org, members across roles, and frames with full slot content, multi-level inheritance, and versions). Open **http://localhost:5173**; UI edits hot-reload. Ctrl-C stops both.

There is no login step in this loop: dev mode disables OIDC and injects a fixed identity, so you land straight in the app as `dev-user`, an org admin - and never hit the "No organization access" screen (see [Troubleshooting](#troubleshooting)).

### Real login loop - `make dev-auth`

```bash
make dev-auth
```

Starts Keycloak in Docker (`:8081`) with an auto-imported realm and runs the backend in OIDC mode on `:5173`, serving the built SPA. Open **http://localhost:5173** (the same URL as `make dev`), log in as **`dev@localhost`** / **`dev`**. That user is seeded as an org admin. Keycloak admin console: http://localhost:8081 (admin / admin). Run `make dev-clean` to tear everything down.

### Troubleshooting

**"No organization access" after login.** This is intentional fail-closed behavior: a signed-in user who is not a member of any org is denied. Locally, `make dev` seeds you (`dev-user`) as an admin, and `make dev-auth` seeds `dev@localhost` as a pending admin that activates on first login - so neither should show this page. If you see it against a real deployment, ask an org admin to add your email.

**`disk I/O error` / `database is locked` on startup.** A previous dev backend was left running (e.g. `make dev` was suspended with Ctrl-Z or killed with `kill -9` instead of stopped with a single Ctrl-C) and still holds the SQLite lock. Run `make dev-clean` to stop the orphan (it frees ports `:5173`/`:8080`), clear the dev DB and its `-wal`/`-shm` files, and reset Keycloak, then start again. Always stop a dev loop with a single **Ctrl-C** so both processes shut down cleanly.

## Deploy on a Nebari Cluster

Nebari Frames ships as a Helm chart (`chart/`) meant to run on a Nebari cluster, where the nebari-operator provisions routing, TLS, and OIDC for it. See [`chart/README.md`](chart/README.md) for the full install walkthrough and [`chart/values.yaml`](chart/values.yaml) for every value; this section covers the parts most likely to trip up a first deploy.

### Prerequisites

- A Nebari cluster provisioned by nebari-infrastructure-core (NIC), with the nebari-operator, Envoy Gateway, cert-manager, and Keycloak foundational stack already running.
- The target namespace must carry the label `nebari.dev/managed: "true"`. Without it, the nebari-operator refuses to manage the app's NebariApp resource: routing, TLS, and the OIDC client are never provisioned, and the deployment fails closed (see [Troubleshooting a Nebari deployment](#troubleshooting-a-nebari-deployment)). Set it directly with `kubectl label namespace <ns> nebari.dev/managed=true --overwrite`, or, if ArgoCD owns the namespace, through `syncPolicy.managedNamespaceMetadata` on the Application so ArgoCD applies the label itself instead of it being clobbered on the next sync.
- Sizing is modest by default: a single replica (`replicaCount: 1`, required because SQLite is single-writer) and a 1Gi PVC for the SQLite database (`persistence.size`). See [`chart/values.yaml`](chart/values.yaml) for the full set of knobs.

### Authentication setup

The chart selects one of three auth modes, in this order (from `chart/values.yaml`):

1. **`nebariapp.enabled: true` and `nebariapp.auth.enabled: true`** - the nebari-operator provisions a Keycloak OIDC client and writes the result to a `<release>-nebari-frames-oidc-client` secret. The Deployment reads its issuer URL, SPA client id, and device client id from that secret; see [`chart/README.md`](chart/README.md#install-on-nebari).
2. **`auth.devMode: true`** - authentication is disabled entirely. The backend sets `FRAMES_DEV_MODE=true` and serves a fixed `dev-user` identity. Local development and demos only, never a real deployment.
3. **Otherwise, `auth.oidc.*`** - self-managed OIDC. Set `auth.oidc.issuerUrl`, `auth.oidc.clientId`, and `auth.oidc.deviceClientId` yourself when you're pointing at an OIDC provider the operator doesn't manage.

For a standalone binary or a non-chart deployment, the equivalent environment variables (see `backend/cmd/server/main.go`) are:

| Env var | Purpose |
| --- | --- |
| `OIDC_ISSUER_URL` | OIDC issuer URL. |
| `OIDC_CLIENT_ID` | OIDC client id for the SPA. |
| `OIDC_DEVICE_CLIENT_ID` | OIDC client id for the device-flow login used by `frames auth login`. |
| `OIDC_GROUPS_CLAIM` | Claim to read group membership from. Defaults to `groups`. |
| `FRAMES_DEV_MODE` | Set to exactly `true` to disable auth (dev only). |

`FRAMES_DEV_MODE=true` short-circuits everything else. Otherwise both `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` are required, or the server fails fast on startup with a message naming the missing variable.

### Chart values you will likely customize

Beyond the auth block above, the values most people end up touching are:

- `nebariapp.hostname` - the hostname the operator routes to the app.
- `seed.orgSlug`, `seed.orgDisplayName`, `seed.adminEmail` - the organization created on first boot, and the email that is reconciled to the first real admin on their first login.
- `persistence.size`, `persistence.storageClass` - PVC size and storage class for the SQLite database.
- `mcp.enabled`, `mcp.publicUrl` - whether the `/mcp` endpoint is mounted, and an override for its public URL when it can't be derived from `nebariapp.hostname`.

See [`chart/values.yaml`](chart/values.yaml) for the complete reference. Releases publish the chart to the Nebari Helm registry, so the simplest install is straight from there:

```bash
# The operator only manages namespaces that opt in, so create and label it
# first (see Prerequisites above).
kubectl create namespace nebari-frames
kubectl label namespace nebari-frames nebari.dev/managed=true

helm install nebari-frames oci://quay.io/nebari/charts/nebari-frames --version 0.1.5 \
  --namespace nebari-frames \
  --set nebariapp.enabled=true \
  --set nebariapp.hostname=frames.example.com \
  --set seed.orgSlug=my-org --set seed.adminEmail=admin@example.com
```

Installing from a git checkout also works, as shown in [`chart/README.md`](chart/README.md#install-on-nebari).

## Known Limitations

- **SQLite is single-writer.** `replicaCount` must stay `1`; the chart defaults to it and documents why in [`chart/README.md`](chart/README.md#values-reference). There is no highly-available mode yet.
- **OIDC discovery happens from inside the pod.** The backend resolves and validates the issuer URL itself at startup, so the pod must be able to resolve the issuer's hostname and trust its TLS certificate. This fails on clusters where the external Keycloak hostname isn't resolvable in-cluster, or where Keycloak serves a certificate the pod doesn't already trust.
- **One organization in the MVP.** `seed.orgSlug` seeds a single organization; there's no cross-org sharing or multi-org UI yet.
- **Role assignment is per-membership.** Each org membership carries its own role today. Keycloak group-to-role mapping is tracked in [#21](https://github.com/nebari-dev/nebari-frames/issues/21) and a default role for new members in [#22](https://github.com/nebari-dev/nebari-frames/issues/22).

## Troubleshooting a Nebari Deployment

| Symptom | Cause | Fix |
| --- | --- | --- |
| Pod stuck in `CreateContainerConfigError`, event names secret `<release>-nebari-frames-oidc-client` as not found | The namespace is missing the `nebari.dev/managed: "true"` label, so the nebari-operator won't manage the NebariApp and never provisions the OIDC client secret - or the operator can reach the namespace but can't reach Keycloak to create the client. | Check the NebariApp's conditions with `kubectl get nebariapps -o yaml`, then check the nebari-operator logs in the `nebari-operator-system` namespace. |
| Readiness probe (`/readyz`) returns 503 and stays there | OIDC discovery can't reach or doesn't trust the issuer from inside the pod. | Confirm in-cluster DNS resolves the issuer URL and that the pod trusts Keycloak's certificate; cross-check the `nebari-operator-system` logs and the Keycloak deployment on the same cluster. |
| `frames` CLI returns `401` on calls that used to work | The cached login token expired. | Run `frames auth login` again. |

## Design Documents

| Doc | Purpose |
|---|---|
| [Migration: skillsctl → Nebari Frames](docs/design/2026-05-21-nebari-frames-migration.md) | Foundation: data model, RBAC, schema, current-state of skillsctl, deviations from the whitepaper, fork rationale |
| [MCP Endpoint Design](docs/design/2026-05-21-mcp-endpoint-design.md) | Remote MCP server for AI clients (Claude.ai, ChatGPT, Gemini, ...) |
| [Web App Design](docs/design/2026-05-21-web-app-design.md) | Browse, author, and connect surface for non-technical users |

Reviewers: @dharhas, @jbouder.

## Status

The data-model + RBAC foundation, backend service, CLI, web app, and MCP endpoint are implemented. See [Run locally](#run-locally).

## License

Apache 2.0. See [LICENSE](LICENSE).
