# Nebari Frames

> **Status:** backend, CLI, web app, and MCP endpoint are implemented. See [Run locally](#run-locally) to try it. Design docs in [`docs/design/`](docs/design/).

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

Runs the backend (dev mode, no OIDC) on `:8080` and the Vite dev server on `:5173`, seeded with representative sample data (an org, members across roles, and frames with inheritance and versions). Open **http://localhost:5173**; UI edits hot-reload. Ctrl-C stops both.

In dev mode you are automatically `dev-user`, an org admin - so you never hit the "No organization access" screen (see [Troubleshooting](#troubleshooting)).

### Real login loop - `make dev-auth`

```bash
make dev-auth
```

Starts Keycloak in Docker (`:8081`) with an auto-imported realm and runs the backend in OIDC mode on `:8080`. Open **http://localhost:8080**, log in as **`dev@localhost`** / **`dev`**. That user is seeded as an org admin. Keycloak admin console: http://localhost:8081 (admin / admin). Run `make dev-clean` to tear everything down.

### Troubleshooting

**"No organization access" after login.** This is intentional fail-closed behavior: a signed-in user who is not a member of any org is denied. Locally, `make dev` seeds you (`dev-user`) as an admin, and `make dev-auth` seeds `dev@localhost` as a pending admin that activates on first login - so neither should show this page. If you see it against a real deployment, ask an org admin to add your email.

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
