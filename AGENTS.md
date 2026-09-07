# AGENTS.md

Guidance for AI coding agents working in this repository. Tool-agnostic: any agent that reads
`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, or a similar convention should follow this file.

## What this is

Nebari Frames is a registry and exchange for **Frames**: scoped, text-based artifacts carrying
organizational context (terminology, rules, style, goals, business process, ...) into AI
conversations. A Frame composes through inheritance, is governed by role-based access control, and
is consumed either over MCP by any MCP-capable client or as files via the `frames` CLI.

Single Go module (`github.com/nebari-dev/nebari-frames`) containing a backend server, a Cobra CLI, a
React SPA embedded into the server binary, a Helm chart, and an Astro/Starlight docs site.

## Commands

```bash
make test          # go test ./... -race -coverprofile=coverage.out
make lint          # golangci-lint run ./...
make proto         # buf lint + buf generate (regenerates gen/go and gen/ts)
make build         # CGO_ENABLED=0 go build -o nebari-frames-server ./backend/cmd/server
make build-web     # npm ci && vite build into web/dist, then make build
make dev           # backend :8080 (dev mode, fixture-seeded) + Vite :5173 with HMR
make dev-auth      # Keycloak in Docker :8081 + backend on :5173 serving the built SPA
make dev-clean     # kill orphan dev servers, drop dev DB + WAL/SHM, tear down Keycloak
make image         # docker build (linux/amd64)
make e2e           # blackbox RPC suite against a deployed frames (needs FRAMES_E2E_BASE_URL + FRAMES_E2E_TOKEN)

# One Go test / package
go test ./backend/internal/frames -run TestResolve -race

# Web (from web/)
npm run lint && npm run typecheck && npm test    # eslint, tsc --noEmit, vitest run
npx vitest run src/pages/CatalogPage.test.tsx    # single test file

# Chart
helm lint chart/
helm template frames chart/ --set auth.devMode=true

# Docs CLI reference (generated from cli/cmd)
go run ./tools/docs-gen
```

CI (`.github/workflows/ci.yml`) gates on five jobs: `proto` (buf lint plus a stale-codegen check on
`gen/`), `go` (golangci-lint pinned to v2.12 plus race tests), `web` (lint, typecheck, vitest),
`chart` (helm lint, template renders, kubeconform pinned to v0.7.0), and `e2e-sandbox` (deploys the
built image onto a kind Nebari sandbox via ArgoCD and exercises real Keycloak auth through the
gateway). `e2e-sandbox` also provisions a dedicated Keycloak client and user, then runs the
build-tagged blackbox RPC suite in `e2e/` (`make e2e`) against the deployed pod through the real
gateway; that suite skips locally whenever `FRAMES_E2E_BASE_URL` and `FRAMES_E2E_TOKEN` are unset, so
`make test` is unaffected. `docs.yml` also fails if the generated CLI reference is stale. Run the
local equivalents before pushing.

**Generated code is checked in.** After touching `proto/frames/v1/*.proto`, run `make proto` and
commit `gen/`. After touching `cli/cmd/*`, run `go run ./tools/docs-gen` and commit
`docs/site/src/content/docs/reference/cli/`.

## Architecture

Request path: **client -> Connect RPC (or MCP) -> auth interceptor -> frames.Service -> rbac.Can ->
store.Repository**.

- `proto/frames/v1/` is the API source of truth. One service, `FrameService`, generated to
  `gen/go/frames/v1` (Connect Go handlers/clients) and `gen/ts/frames/v1` (consumed by the SPA via
  the `@gen` alias). All three surfaces (web, CLI, MCP) speak the same RPCs.
- `backend/internal/server` wires the mux: unauthenticated `/healthz`, `/readyz`, `/auth/config`,
  `/config.json` (runtime branding); the FrameService at its generated path behind
  `auth.NewInterceptor`; optionally the MCP component; and the embedded SPA at `/`. The
  FrameService is injected rather than constructed here, so the Connect and MCP endpoints share one
  instance and cannot be configured differently. Request bodies are capped at
  `server.MaxRequestBytes`, because a body is read in full before RBAC is consulted.
- `backend/internal/auth` resolves identity. `FRAMES_DEV_MODE=true` short-circuits everything with a
  fixed `dev-user` identity and a permanently-ready `/readyz`. Otherwise a lazy OIDC validator
  performs discovery from inside the pod, and `/readyz` stays 503 until it succeeds (fail closed).
- `backend/internal/orgs` turns claims into an `rbac.Caller`. Precedence is fixed: a stored
  membership, then a pending invite matched by email, then the baseline role from
  `FRAMES_DEFAULT_ROLE` (empty means deny, which is the fail-closed default). The baseline
  membership is persisted, so those users appear in the members list and are promotable - and it is
  written insert-only, because the "no membership" read that leads there is not atomic with the
  write and an update would overwrite a role another request had just established.
- `backend/internal/rbac` is the single authorization decision point. `Can` evaluates in order:
  cross-org deny, admin allow, then per-frame grants for the user or the org. Roles are
  `viewer | publisher | admin`, permissions `read | edit | delete`. A missing read permission is
  surfaced as 404, never 403, so frame existence does not leak.
- `backend/internal/frames` owns the Frame content model:
  - `schema.go` - the `Doc`/`Slots` YAML types. Parsing uses `KnownFields(true)`: the slot schema is
    fixed and unknown keys are an error.
  - `slots.go` - `SlotTable` is the single source of truth for slot keys, markdown headings, and
    content shape (terms / list / prose). Add or rename a slot here only; the `.frame.md` codec and
    the MCP composer both read it.
  - `framemd.go` - the Frame Spec v0.2 `.frame.md` codec (YAML frontmatter plus one `##` section per
    slot). Round-trip fidelity matters: `examples/*.yaml` and `examples/*.frame.md` are checked-in
    conformance fixtures asserted by `examples_test.go`.
  - `resolver.go` - inheritance merge over `extends`/`excludes`. Later parents win, the child's own
    slots win last, cycles produce `CycleError`, and an unreadable ancestor propagates
    `ErrParentUnreadable` rather than silently dropping content.
  - `service.go` - `publish` is the single write path behind both front doors. It authorizes before
    parsing caller-supplied content, enforces `MaxContentBytes` on the stored document, refuses a
    version that does not advance `latest_version`, and refuses a write whose declared base version
    is no longer current. The Connect RPC stores the author's exact bytes; `PublishDoc` marshals the
    document instead, so comments and formatting survive a CLI or web publish.
- `backend/internal/store` defines `Repository` plus an in-memory implementation used by tests;
  `store/sqlite` is the real one, with goose migrations in `store/sqlite/migrations`. Emails are
  canonical at rest (`store.CanonicalEmail`) and unique per org case-insensitively, so an invite
  cannot be shadowed by a case variant. The in-memory fake enforces the same unique constraints;
  where it cannot, tests reach for real SQLite and say why. Publishes go
  through `CreateFrameVersion`, which inserts the frame row, version, inheritance edges, and grants
  atomically. **SQLite is single-writer, so the deployment is pinned to one replica.**
- `backend/internal/mcp` is a thin protocol adapter over `frames.Service`, exposing frames as MCP
  resources under `nebari-frame://<org>/<name>[@<version>]` plus RFC 9728 metadata at
  `/.well-known/oauth-protected-resource`. It is also a write surface: `create_frame` and
  `update_frame` go through `frames.Service.PublishDocFrom`, the same RBAC-enforcing path the
  Connect API uses, so the adapter itself performs no permission or validation logic. Two rules
  matter when changing it. `update_frame` merges onto the frame's own document from `SourceDoc` and
  never onto the composed form `get_frame` returns by default - merging onto a resolved document
  would copy every parent's slots into the child and drop its `extends` edges. And the base version
  it asserts against comes from the caller (`base_version`, read via `get_frame source=true`), never
  from a fresh server-side read, which would always match and make the check inert. Request bodies
  are capped at `mcp.MaxRequestBytes`; the cap must wrap the outermost handler, since the bearer
  middleware is only installed when auth is on. URI parsing rejects anything structurally off (extra path
  segments, `.`/`..`) instead of misrouting it.
- `web/` is the SPA. `web/embed.go` embeds `web/dist` into the Go binary and serves it with an
  index.html fallback and a CSP assembled from the OIDC issuer origin and branded image origins.
  `web/dist/index.html` is deliberately kept in git (everything else under `dist/` is ignored) so
  `//go:embed` still compiles on a clean checkout. Auth guards live in `web/src/app/`
  (`RequireAuth`, `RequireMembership`, `RequireAdmin`); pages in `web/src/pages/`; RPC transport and
  domain helpers in `web/src/lib/`.
- `cli/` is the `frames` binary (Cobra plus Viper; config at `~/.config/frames/config.yaml`, env
  prefix `FRAMES_`, device-flow login).
- `chart/` deploys onto a Nebari cluster; the nebari-operator provisions routing, TLS, and the OIDC
  client from the `NebariApp` resource.

## Conventions

- Table-driven Go tests. Tests sit beside the code they cover, including in `web/` (`*.test.tsx`).
- Migrations are only ever applied to a fresh database by the normal suite, which hides anything that
  breaks on existing rows. `store/sqlite/migrations/migrate_legacy_test.go` builds an older schema
  with the data a migration has to repair and migrates forward; extend it when a migration touches
  existing rows.
- Never bump `chart/Chart.yaml`'s `version`/`appVersion` by hand: the release job stamps them from
  the git tag (`version` = tag without `v`, `appVersion` = the literal tag).
- The SPA ships inside the image, not the chart. A frontend change reaches a cluster only through a
  new image tag.
- A test that asserts only "this was rejected" usually proves nothing: an unrelated 401, or a parse
  failure, satisfies it just as well. Assert the specific code or message, and pair a rejection with
  a control that must succeed. Reflective guards in `backend/internal/mcp/resources_test.go` walk
  `frames.SlotTable` and `frames.Doc`, so adding a slot without wiring it through the MCP input
  fails rather than silently dropping data.
- Comments in this repo explain *why* a constraint exists (pinned CI versions, fail-closed
  readiness, the vite `@bufbuild/protobuf` aliases). Preserve that rationale when editing near it,
  and keep new comments in the same register.
- Always stop a dev loop with a single Ctrl-C. Killing it leaves the SQLite lock held and the next
  start fails with `disk I/O error` / `database is locked`; `make dev-clean` recovers.
- Design docs in `docs/design/`, client connection guides in `docs/connect/`, manual QA scripts in
  `docs/qa/`.
