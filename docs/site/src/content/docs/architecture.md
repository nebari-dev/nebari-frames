---
title: Architecture
---

## Components

- **Backend** (`backend/`) - a single Go binary that serves both the Connect RPC API and the built SPA. It owns startup wiring: opens the SQLite database, runs migrations, seeds the first org/admin, selects an auth mode, and optionally mounts the MCP endpoint.
- **Web app** (`web/`) - a Vite-built SPA, embedded into the backend binary at build time (`make build-web`) so the shipped artifact is one binary and one container image.
- **Store** (`backend/internal/store/sqlite`) - SQLite via `modernc.org/sqlite` (pure Go, no cgo), on a PVC in Kubernetes. Single-writer by design: `replicaCount` is pinned to `1` and the Deployment uses the `Recreate` strategy so the previous pod releases the volume before the next one mounts it.
- **CLI** (`cli/`) - the `frames` binary (built on `github.com/spf13/cobra`), talking to the backend over Connect RPC. See the [CLI Reference](/reference/cli/frames/).
- **MCP endpoint** (`backend/internal/mcp`) - a remote MCP server mounted at `/mcp`, letting any MCP-capable AI client (Claude, ChatGPT, Gemini, and others) read Frames the authenticated caller can access.
- **NebariApp / operator integration** (`chart/templates/nebariapp.yaml`) - on a Nebari cluster, the chart creates a `NebariApp` custom resource; the nebari-operator reconciles it into routing, TLS, a landing-page tile, and (optionally) an OIDC client.

## Request flow

1. A request arrives at the Envoy Gateway route the operator created from the `NebariApp`'s `routing` spec (or, in a self-managed/standalone deployment, at whatever ingress you put in front of the Service).
2. The backend's HTTP server (`backend/internal/server`) dispatches to either the static SPA handler, a Connect RPC handler, or the MCP handler for `/mcp` requests.
3. RPC handlers authenticate the caller (see Auth flow below), then apply RBAC (`backend/internal/rbac`) before touching the store.
4. Reads that involve inheritance - `resolve`, and the CLI's `resolve`/`extends` - walk the `extends` graph in `backend/internal/frames` and compose parent content into the resolved form.

## Auth flow

Authentication is fail-closed: the backend will not start serving authenticated traffic with incomplete configuration, and a Frame read either succeeds against a caller with real access or is rejected outright - there is no degraded "read-only anonymous" mode.

At startup the backend chooses exactly one auth mode (see [Configuration](/configuration/#auth-modes) for the precedence), then either:

- runs with authentication **disabled** and every request treated as a fixed `dev-user` identity (dev mode, local/demo only), or
- validates every request's OIDC token against an issuer, using either the operator-provisioned client (`OIDC_ISSUER_URL` / `OIDC_CLIENT_ID` / `OIDC_DEVICE_CLIENT_ID` from the NebariApp-managed secret) or a self-managed OIDC client you configure directly.

When NebariApp-managed, the nebari-operator is the piece that actually talks to Keycloak: it registers the SPA and device-flow clients and writes their issuer/client IDs into a Kubernetes secret the Deployment mounts as environment variables. The backend performs OIDC discovery against that issuer in the background at startup and only reports `/readyz` healthy once discovery succeeds - see [Troubleshooting](/troubleshooting/) for what it looks like when that does not happen.

The CLI authenticates separately from the web app, through an OIDC device-code flow (`frames auth login`) against the same issuer, using the device-flow client id.

The MCP endpoint reuses the same OIDC issuer but validates tokens against its own audience (`OIDC_MCP_AUDIENCE`, resolved from `mcp.publicUrl` or the NebariApp hostname), so a token issued for the MCP endpoint cannot be replayed against the main API, or vice versa.
