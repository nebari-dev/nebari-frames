# Nebari Frames

> **Status:** early design phase. No working code yet. Design docs in [`docs/design/`](docs/design/).

Nebari Frames is the registry and exchange for **Frames**: scoped, text-based artifacts that carry organizational context (terminology, style, goals, rules, business processes, and more) into AI conversations. A Frame composes through inheritance, is governed through role-based access control, and is consumable by any MCP-capable AI client (Claude, ChatGPT, Gemini, and others) or by Claude Code through file install.

This project is the successor to [`skillsctl`](https://github.com/nebari-dev/skillsctl), which it borrows registry foundations from. `skillsctl` continues to ship as the Claude Code skill registry; new investment goes here.

## Why Frames

Enterprise AI adoption is gated less by model capability and more by the organizational context that turns a generic model into a specialized worker: the brand voice, the compliance constraints, the named concepts, the team norms. Today that context lives in style guides, wikis, Slack history, and the heads of senior employees. Frames make it explicit, portable, inheritable, and governable - a first-class artifact that an organization owns and shares on its own terms.

See [Background §1.1 in the migration design doc](docs/design/2026-05-21-nebari-frames-migration.md#11-the-whitepaper-in-one-paragraph) for the broader framing from the OpenTeams *Intelligence Hub Whitepaper - v4*.

## Design Documents

| Doc | Purpose |
|---|---|
| [Migration: skillsctl → Nebari Frames](docs/design/2026-05-21-nebari-frames-migration.md) | Foundation: data model, RBAC, schema, current-state of skillsctl, deviations from the whitepaper, fork rationale |
| [MCP Endpoint Design](docs/design/2026-05-21-mcp-endpoint-design.md) | Remote MCP server for AI clients (Claude.ai, ChatGPT, Gemini, ...) |
| [Web App Design](docs/design/2026-05-21-web-app-design.md) | Browse, author, and connect surface for non-technical users |

Reviewers: @dharhas, @jbouder.

## Status

Design docs are drafted and pending review. No implementation has begun. The intended next step (after design approval) is to produce an implementation plan for the data-model + RBAC foundation (the migration doc) and begin building the backend service.

## License

Apache 2.0. See [LICENSE](LICENSE).
