# Design Doc: Nebari Frames MCP Endpoint

| | |
|---|---|
| **Status** | Draft - pending review |
| **Author** | Chuck McAndrew |
| **Created** | 2026-05-21 |
| **Last updated** | 2026-05-21 |
| **Reviewers** | @dharhas, @jbouder |
| **Depends on** | [Nebari Frames Migration](./2026-05-21-nebari-frames-migration.md) |
| **Companion doc** | [Web App Design](./2026-05-21-web-app-design.md) |

## TL;DR

A remote Model Context Protocol (MCP) endpoint on the Nebari Frames server, accessible to any MCP-capable AI client (Claude.ai, ChatGPT, Gemini, Cursor, Windsurf, Codex, others). Users add the Hub as a connector once, OAuth in, and from that point can include their organization's Frames as context in any conversation. The endpoint is **the primary distribution mechanism for Frames to browser-based AI** and the fastest path to getting Frames in front of non-technical users.

The MCP endpoint is a server-side component. All RBAC enforcement happens server-side; the client receives only what the caller is entitled to read.

## 1. Background

### 1.1 What MCP is

Model Context Protocol is an open protocol originated by Anthropic in 2024 and adopted broadly through 2025-2026. As of 2026, all major AI providers support remote MCP connectors with OAuth: Anthropic (Claude.ai, Claude Desktop, Claude Code), OpenAI (ChatGPT, Codex CLI), Google (Gemini, AI Studio), plus IDE-embedded clients like Cursor, Windsurf, and Zed.

MCP defines three primitives:

- **Resources** - read-only addressable content. Clients enumerate, then fetch on demand or include in context.
- **Tools** - callable functions with typed parameters. The model invokes them mid-conversation.
- **Prompts** - reusable prompt templates the user can invoke explicitly.

Resources are the primary primitive for serving content like Frames. Tools are useful when the model needs to query something dynamically.

### 1.2 Why MCP is the right bet for Frames

A single MCP endpoint serves every major AI client without per-provider integration work. The protocol itself is provider-agnostic. Per-provider work is limited to documentation pages (how to add the connector in claude.ai vs chatgpt vs gemini); the server code is one implementation. That's roughly an 80% effort savings over building N native connectors.

Caveats worth naming: feature support varies across clients (some treat resources as second-class to tools), size and rate limits differ, and admin approval flows for connectors differ. The protocol is the same; the integration UX is per-provider. Acceptable cost for the leverage.

### 1.3 What the migration doc settled

Per the [migration design doc](./2026-05-21-nebari-frames-migration.md), the MCP endpoint is one of three MVP surfaces (alongside web app and CLI). Frames are the only artifact type served. Visibility is frame-level only in MVP. RBAC is enforced server-side through the `rbac/` package. The MCP endpoint lives in `backend/internal/mcp/` and consumes `frames.Service` like any other client of the backend.

## 2. Goals and Non-Goals

### 2.1 Goals

| # | Goal |
|---|------|
| G1 | Expose user-readable Frames as MCP resources, fetchable by any MCP-capable client |
| G2 | Authenticate via the existing OIDC stack (OAuth on the MCP side; same provider as the CLI device flow uses) |
| G3 | Enforce all RBAC server-side; never return content the caller isn't entitled to |
| G4 | Compose resolved Frame content (inheritance applied) into a single readable resource per Frame |
| G5 | Ship documentation for at least Claude.ai connector setup at launch; add docs for ChatGPT and Gemini soon after |

### 2.2 Non-Goals (MVP)

| # | Non-Goal | Deferred to |
|---|----------|-------------|
| N1 | "Active stack" / Frame composition management via MCP | Users pick which resources to include per-conversation via the client UI; explicit active-stack curation is a web app feature, roadmapped |
| N2 | MCP tools for searching, querying, or filtering Frames programmatically | Resources are enough for MVP; tools are additive |
| N3 | MCP prompts (reusable templates served from Frames' `prompts` slot) | Roadmap; the slot is conservatively typed (list of strings) and not yet shaped for MCP-prompt semantics |
| N4 | Cross-provider feature-parity testing matrix | We test Claude.ai end-to-end; other providers are docs-only on day one |
| N5 | Rate limiting and abuse prevention specific to MCP | Reuse whatever the broader backend ships for rate limiting |
| N6 | Audit logging of MCP-served reads | Roadmap; same audit-log story as the rest of the system |

## 3. Proposal

### 3.1 Architecture

```
                ┌──────────────────────────┐
                │  AI client (claude.ai,   │
                │  chatgpt, gemini, ...)   │
                └────────────┬─────────────┘
                             │
                       MCP over HTTP/SSE (or stdio for local clients)
                             │
                             │  OAuth bearer token
                             │
                ┌────────────▼─────────────┐
                │  /mcp endpoint           │
                │  (backend/internal/mcp)  │
                │                          │
                │  - OAuth token validate  │
                │  - resolve user + org    │
                │  - serve MCP protocol    │
                └────────────┬─────────────┘
                             │
                ┌────────────▼─────────────┐
                │  frames.Service          │
                │  (already RBAC-gated)    │
                └────────────┬─────────────┘
                             │
                ┌────────────▼─────────────┐
                │  rbac.Authorizer         │
                └──────────────────────────┘
```

The `/mcp` endpoint is a thin protocol adapter. It does not implement any business logic of its own beyond MCP-protocol mechanics; all content decisions go through `frames.Service`, which goes through `rbac`.

### 3.2 Authentication

**OAuth 2.1 with PKCE** (the auth flow MCP remote connectors use). Same OIDC provider as the CLI device flow - just a different OAuth client registration:

| Client | Flow | OIDC client ID |
|---|---|---|
| CLI | RFC 8628 device code | `nebari-frames-cli` |
| MCP | OAuth 2.1 + PKCE | `nebari-frames-mcp` |
| Web app | Standard authorization code | `nebari-frames-web` |

OIDC token validation reuses `backend/internal/auth` (the same package serving the ConnectRPC API today). The MCP handler attaches the validated identity to the request context, then calls a shared org-resolver to populate `Caller{UserSub, OrgID, Role}` for downstream RBAC.

The MCP spec defines OAuth discovery via `/.well-known/oauth-protected-resource`; we serve this from the `/mcp` endpoint along with the existing OIDC discovery the CLI uses.

### 3.3 Resource model

**One MCP resource per resolved Frame** the caller can read.

```
URI scheme:  nebari-frame://<org_slug>/<frame_name>@<version>
Example:     nebari-frame://openteams/brand-voice@1.2.0
```

Each resource has:

| Field | Value |
|---|---|
| `uri` | `nebari-frame://<org>/<name>@<version>` (latest version unless explicitly versioned) |
| `name` | Human-readable name: e.g. `Brand Voice (OpenTeams)` |
| `description` | The Frame's `description` field |
| `mimeType` | `text/markdown` |

Listing resources (`resources/list`) returns the user's full set of readable Frames. The client decides which to include in any given conversation - Claude.ai surfaces this as a picker; ChatGPT and Gemini have similar selection UIs.

**MVP behaviour:** the resource list is the user's full readable set in their org (filtered server-side by RBAC). No curation/active-stack mechanism beyond what each client UI provides. This is intentional - "active stack" management is a roadmap feature deferred to the web app.

### 3.4 Resource content

When a client reads a resource (`resources/read`), the server:

1. Parses the URI to extract org / name / version (or "latest").
2. Calls `frames.Service.GetResolved(ctx, frameID, version)` - returns the inheritance-merged Frame.
3. `frames.Service` consults `rbac.Can(caller, Read, frame)` first; 404 if denied.
4. Server composes resolved Frame slots into markdown.

**Composition format** (deterministic; same shape across all readers):

```markdown
# Frame: <name>

<description>

> Inherits from: <parent1>@<v>, <parent2>@<v>
> Resolved at: <ISO timestamp>

## Terminology

- **<term>**: <definition>
- ...

## Rules

- <rule 1>
- <rule 2>

## Skills

- <skill 1>
- ...

## Prompts

- <prompt fragment 1>
- ...

## Tool Specifications

<freeform markdown>

## Goals

<prose>

## Style

<prose>

## Norms

<prose>

## Architecture

<prose>

## Business Process

<prose>
```

Empty slots are **omitted from the rendered markdown** (no empty headers). The format is stable across requests so the AI can rely on consistent structure.

### 3.5 Per-provider compatibility notes

| Client | Connector type | Notes |
|---|---|---|
| Claude.ai | Remote MCP connector | Primary target; full end-to-end testing |
| Claude Desktop | Remote MCP connector | Works; same docs apply |
| Claude Code | Local MCP server config | Possible but not the primary path - Claude Code users have the CLI |
| ChatGPT | Remote MCP / Custom Connector | Docs only at MVP; resource semantics confirmed to work but enterprise admins may need to allow-list the connector |
| Gemini / AI Studio | Remote MCP | Docs only at MVP; same enterprise allow-list caveat |
| Codex CLI | Local MCP server | Likely fine since CLI users have their own auth, but tested only when demand appears |
| Cursor / Windsurf / Zed | Local MCP server | Same as Codex |

Per-provider docs land in the web app's "Connect" pages (see [web app design](./2026-05-21-web-app-design.md), §3.4). One page per client; each page is a small step-by-step with screenshots.

### 3.6 Endpoint surface

The MCP server speaks a single endpoint `/mcp` over HTTPS with Server-Sent Events (SSE) transport, matching the remote-connector convention used by Claude.ai. Stdio transport (for local clients) is not in MVP; remote clients are the audience that matters.

Supported MCP methods (MVP):

| Method | Behavior |
|---|---|
| `initialize` | Standard MCP handshake; advertises `resources` capability only |
| `resources/list` | Returns user's readable Frames, paginated |
| `resources/read` | Returns composed markdown for a specific Frame |
| `resources/subscribe` | Out of scope for MVP (no push updates yet) |
| Anything else | Returns `method not found` |

### 3.7 Pagination

`resources/list` paginates at 50 frames per page. The cursor is a frame ULID. Clients that don't paginate get the first 50; this is acceptable for MVP because no early-stage org will have more than 50 Frames published.

### 3.8 Caching

No server-side response caching in MVP. Frame content is small (sub-100KB typical), Frame counts per org are small, and read latency is dominated by the resolver walking the inheritance graph (which is itself bounded by the small graph sizes expected in early orgs). When this becomes a real cost, the natural cache is on `frames.Service.GetResolved` (memoize by `(frame_id, version)` since pinned-version refs make resolved content immutable until republish).

## 4. Open Questions

- **OQ1: Active stack vs full set.** MVP serves the user's full readable Frame set as resources, letting the client UI pick. Some clients (notably ChatGPT) may not surface that picker well, leading to either everything included by default (token-wasteful) or nothing (useless). Worth real-world testing once we have multiple clients connected.
- **OQ2: Inheritance trail metadata.** The composed markdown includes a single-line "Inherits from" header. Should we expose the inheritance trail as a more structured separate resource (so an AI can introspect provenance), or is the inline comment enough? Defer to first real user feedback.
- **OQ3: Resource refresh signaling.** MCP supports `notifications/resources/list_changed` to tell clients to re-list. We don't implement this in MVP, so a newly published Frame won't appear in an existing client session until reconnect. Acceptable for MVP; revisit if users complain.
- **OQ4: Tool surface for Frame search.** ChatGPT in particular seems to leverage tools more naturally than resources. A `search_frames(query)` tool might give a better UX in clients that don't surface resources well. Pure additive; can ship in v1.1 if the data says it's worth it.
- **OQ5: MCP prompts integration.** Frames have a `prompts` slot. MCP defines a `prompts` primitive. Connecting them is intuitive but the current `prompts` slot shape (list of strings) doesn't carry names, which MCP prompts require. Defer; revisit when we upgrade the prompts slot.

## 5. Alternatives Considered

### 5.1 Build native connectors per provider (Claude, ChatGPT, Gemini)

Rejected. Each provider's native connector API differs significantly; building N implementations costs roughly N times more than one MCP implementation. MCP exists exactly to solve this fragmentation and is supported by all major providers.

### 5.2 Use only MCP tools, no resources

Rejected. Tools require explicit model invocation. Resources are picked once by the user at conversation setup and silently flow into context. For "make my org's Frames available as context" - the actual user need - resources match the workflow. Tools are an additive option for v1.1+.

### 5.3 Serve raw Frame YAML instead of composed markdown

Rejected. The AI consuming the resource has to parse it. Composed markdown with named sections is the format AI models read natively well; raw YAML would force the AI to re-derive the structure that we already know. Raw YAML is a fine debug surface (CLI `frame show --raw`); not the MCP serving format.

### 5.4 Stdio MCP (run a local server per user)

Rejected for remote clients (Claude.ai, ChatGPT, Gemini) - they require remote connectors over HTTP. Local MCP via stdio is fine for CLI-adjacent clients (Claude Code, Cursor, etc.) but those users have the CLI itself; less leverage in supporting it.

### 5.5 Build a one-off Claude.ai connector first, defer MCP-protocol abstraction

Rejected. Claude.ai's connector mechanism IS MCP. There is no shorter path. "Build MCP" and "build a Claude.ai connector" are the same project.

## 6. Security Considerations

- **All RBAC server-side.** The `/mcp` endpoint never returns frame content without `rbac.Can(caller, Read, frame)` returning allow.
- **OAuth scopes.** MCP client gets read-only scope (`frames:read`). No publish or admin capability via MCP - those flow through CLI / web app. Reduces blast radius if an MCP token is compromised.
- **Token TTL.** MCP OAuth tokens follow standard OAuth refresh semantics (short access token + refresh token). The user can revoke at the OIDC provider level.
- **Same-origin and CSRF.** Not applicable; MCP is API-to-API after OAuth. Token in bearer header.
- **Content size caps.** Server enforces the same 512KB per-frame content cap as the rest of the system. A Frame with 100MB of inherited content is rejected at publish time, not at MCP-read time.
- **Information disclosure via list.** `resources/list` returns Frame names and descriptions even before content is fetched. Names and descriptions are intentionally shareable within an org (that's the point of a registry); cross-org names are not listed because `rbac` filters by org.
- **Connector trust prompts.** Enterprise admins at the consumer side (claude.ai org admin etc.) often gate third-party connectors. We document the trust prompts in the per-provider Connect pages.

## 7. Testing Strategy

- **Unit tests** for the MCP protocol adapter: list and read responses, pagination cursors, error mapping (404 from `rbac` denial maps to a clean MCP error, not a 500).
- **Integration tests** against a real OIDC provider (Dex in test) with a real OAuth flow: token issuance, validation, refresh.
- **End-to-end test against Claude.ai** as part of the alpha rollout - add the connector manually, verify resource list and content render correctly in a chat. Automated only via screenshot diffing if it becomes a regression-prone area.
- **Cross-provider smoke tests** (ChatGPT, Gemini) once docs exist for those providers; manual at first, automated later if there's churn.
- **Fuzz / negative tests** on the URI scheme (`nebari-frame://...`) to catch parser injection edge cases.

## 8. Rollout

- **Alpha:** MCP endpoint live on the OpenTeams-internal Hub; connect Claude.ai manually; verify the three example Frames render correctly.
- **Closed beta:** invited orgs add their own Claude.ai connectors. Connect docs for Claude.ai go on the web app.
- **Open beta:** ChatGPT and Gemini Connect docs added. The MCP endpoint itself doesn't change; per-provider docs are the new shipping work.
- **GA:** rate limits, audit logging, Cursor/Windsurf/Codex docs.

## 9. Roadmap (Post-MVP)

1. `search_frames` tool for clients that surface tools better than resources.
2. MCP `prompts` primitive backed by the Frame `prompts` slot (requires upgrading that slot to `{name, body}` shape).
3. `notifications/resources/list_changed` for live updates when Frames are published.
4. Active-stack curation - users choose a default subset; MCP serves only those by default.
5. Audit logging of MCP-served reads.
6. Section-level filtering via MCP once section-level visibility ships (`docs/design/...-migration.md` D2).
