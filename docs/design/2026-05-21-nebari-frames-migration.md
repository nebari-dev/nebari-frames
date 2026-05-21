# Design Doc: Migrating skillsctl to Nebari Frames

| | |
|---|---|
| **Status** | Draft - pending review |
| **Author** | Chuck McAndrew |
| **Created** | 2026-05-21 |
| **Last updated** | 2026-05-21 |
| **Reviewers** | @dharhas, @jbouder |

## TL;DR

The OpenTeams *Intelligence Hub Whitepaper - v4* (April 2026) describes a three-layer architecture for the distributed AI economy: Infrastructure (Nebari / Nebi / Intelligence Hubs), Execution (Frames, Cogs, Ops), and Economy (a marketplace for those artifacts). The existing `skillsctl` project is a working registry and CLI for Claude Code skills, and its core primitives - publish, version, install, OIDC-authenticated catalog - map cleanly onto what the whitepaper calls the **Frame** layer.

This document proposes evolving `skillsctl` into the registry for **Nebari Frames**: scoped, text-based artifacts that carry organizational context (terminology, style, goals, rules, etc.), composable through inheritance, governed through RBAC, and consumable by AI clients (Claude in the browser via MCP, Claude Code via file install, future providers via per-client connectors).

The work is scoped aggressively for an MVP that gets real users in front of working code. Cogs and Ops are deferred indefinitely; Nebi integration is deferred; section-level visibility, implicit org-tree inheritance, and grant management UI are roadmapped. The MVP delivers (1) a Frame data model with a clean RBAC foundation, (2) a remote MCP endpoint for Claude.ai, and (3) a web app for non-technical users to browse and connect Frames. The CLI remains mostly unchanged for technical users.

The work will land in a new repository: **https://github.com/nebari-dev/nebari-frames**. The existing `skillsctl` repo stays in maintenance mode for its current Claude Code skills audience. Reasoning for the fork (as opposed to evolving `skillsctl` in place) is in [the migration path section](#migration-path).

## 1. Background

### 1.1 The whitepaper in one paragraph

OpenTeams positions a three-layer architecture in which **Intelligence Hubs** (Nebari deployments) are owned, governed infrastructure for AI. On top of that infrastructure run three classes of artifact: **Frames** (organizational context: terminology, style, norms, rules), **Cogs** (AI workers that operate within Frames), and **Ops** (orchestrated workflows that combine Cogs and Frames into business outcomes). All three are versioned, shareable, and exchangeable through a marketplace. **Frames are the focus of this design**: they are the lowest-friction artifact to adopt, the highest-leverage for alignment, and the layer most directly served by what `skillsctl` already does.

### 1.2 Current state of skillsctl

`skillsctl` (https://github.com/nebari-dev/skillsctl) is a Go-based registry and CLI for distributing Claude Code skills. As of commit `4967bd9` (May 2026):

**Capabilities today:**

- Backend service (`backend/cmd/server`) exposing a ConnectRPC API for publish, list, get, and content-fetch of skills.
- SQLite storage (pure-Go `modernc.org/sqlite`) with content stored as a versioned BLOB per skill.
- Generic OIDC token validation (works against Keycloak, Okta, Dex) via an auth interceptor; CLI uses RFC 8628 device-code flow.
- Cobra-based CLI (`skillsctl`) supporting `init`, `explore`, `install`, `publish`, `auth login`, plus a few admin and config subcommands.
- Homebrew, install script, and `go install` distribution.
- Pack metadata for a software-pack dashboard (`pack-metadata.yaml`).
- Documentation site at `skillsctl.dev` (built from `docs/site/`).

**What it deliberately doesn't have:**

- Any concept of organizations, memberships, or roles. Authentication is a single OIDC validation; authorization is "valid token = can do anything."
- Any concept of inheritance, composition, or scope across artifacts. Skills are flat: each skill is independent.
- Any web UI. The CLI is the only client surface.
- Any non-Claude-Code consumer integration. Skills install into `~/.claude/skills/` (or equivalent); there is no remote MCP endpoint, no browser connector, no provider-agnostic content surface.
- Any visibility or sharing model beyond "all authenticated users see all skills."

**Data model (current):**

```sql
skills (name PK, description, owner, tags, latest_version, install_count, created_at, updated_at, source, marketplace_id, upstream_url)
skill_versions (skill_name FK, version, oci_ref, digest, size_bytes, published_by, changelog, draft, published_at, content BLOB, PK(skill_name, version))
```

That is the entire schema. Two tables. No org dimension, no permission dimension, no inheritance dimension.

### 1.3 Why skillsctl is a sensible starting point for Frames

The expensive parts of building a Frame registry are the parts skillsctl already has:

- Versioned content store with atomic publish semantics.
- ConnectRPC API surface generated from protobuf, with both gRPC and JSON/HTTP transports.
- OIDC integration with both interactive (device flow) and service (token-validation) sides.
- A CLI distribution story (Homebrew, install script, `go install`, source).
- A documentation site and basic governance.

The parts that are *missing* - structured artifact schemas with inheritance, organizations with RBAC, multi-client content delivery (MCP), and a non-technical UI - are the parts of the whitepaper Frame model that don't exist in any skill registry today. So we're not throwing away work; we're building the Frame-specific layer on a registry foundation that already exists.

## 2. Goals and Non-Goals

### 2.1 Goals

| # | Goal | Why |
|---|------|-----|
| G1 | Establish a Frame data model and registry capable of expressing the whitepaper's Frame concept | Foundation for everything else |
| G2 | Make Frames consumable by Claude.ai users via a remote MCP connector | Single highest-value distribution channel for non-technical users |
| G3 | Provide a web UI for non-technical users to browse Frames and obtain connector instructions | Non-technical users will not install a CLI |
| G4 | Preserve a CLI path for technical users | Engineers will want to author Frames in their editor and ship via CLI; this is how skillsctl works today |
| G5 | Build RBAC primitives that can grow into full per-frame and per-section grants without a future data-model migration | Retrofitting RBAC onto a flat data model is the single most expensive migration to defer |
| G6 | Keep the MVP small enough to put working code in front of real users in weeks, not quarters | Feedback signal from working software beats feedback on mockups |

### 2.2 Non-Goals (MVP)

| # | Non-Goal | Deferred to |
|---|----------|-------------|
| N1 | Cogs (AI workers) and Ops (orchestrated workflows) | Indefinitely; arguably never in this product |
| N2 | Nebi integration as the distribution mechanism | When a Nebi spec exists to integrate against |
| N3 | Section-level Frame visibility (selective field-level sharing) | Roadmap item; data model accommodates it |
| N4 | Implicit inheritance via organizational scope tree | Roadmap item; explicit inheritance covers MVP use cases |
| N5 | Grant management UI (per-user, per-group, per-section permission editing) | Roadmap item; MVP RBAC exposed as three roles |
| N6 | Multi-org membership for a single user | Roadmap item; one-org-per-user PK contains the migration cost |
| N7 | Cross-org Frame sharing | Roadmap item; MVP serves only intra-org reads |
| N8 | Non-Claude provider connectors (ChatGPT, Gemini, Codex) | Post-MVP; the MCP protocol gives us ~80% of this for free, but per-provider onboarding docs and testing is incremental work |
| N9 | File-install connectors for Claude Code, Cowork, Codex (write Frames to local disk) | Post-MVP; CLI users get a different path through the existing CLI |
| N10 | A "Frame protocol" formal specification document for external authors | Will follow once the schema has been stress-tested by real users |

## 3. Proposal

Build a new artifact type, **Frame**, into a Nebari Frames registry hosted in the new `nebari-frames` repo, seeded from `skillsctl`'s foundational packages. Ship three surfaces:

1. **Backend service** with a `FrameService` ConnectRPC API and a remote MCP endpoint.
2. **Web app** for browsing Frames and obtaining connector instructions.
3. **CLI** for technical users to author and publish Frames - a new binary in the `nebari-frames` repo, seeded from the existing `skillsctl` CLI codebase but distinct from it (separate binary, separate distribution, separate Homebrew formula). The original `skillsctl` CLI continues to ship from its own repo for Claude Code skill users.

The backend follows **Approach A** from the brainstorming session: Frames are a *sibling artifact type* alongside Skills, sharing infrastructure (auth, store, server registration) but with separate tables, separate proto service, and separate service package. This was chosen over (B) unifying skills and frames into one polymorphic artifact (the schemas barely overlap; polymorphism would leak into the API) and (C) a fully independent subsystem (premature; the same binary serves both fine for MVP).

### 3.1 High-level component diagram

```
                          ┌──────────────────────┐
                          │  Web app (browser)   │
                          │  - Browse Frames     │
                          │  - Connector setup   │
                          └──────────┬───────────┘
                                     │ ConnectRPC/HTTP
┌────────────────────┐               │
│  Claude.ai (etc)   │── MCP/OAuth ──┤
└────────────────────┘               │
                                     │
┌────────────────────┐               │
│  Frames CLI        │── ConnectRPC ─┤
│  (new binary in    │               │
│   nebari-frames)   │               │
└────────────────────┘               │
                          ┌──────────▼───────────┐
                          │   Backend server     │
                          │  - FrameService RPC  │
                          │  - MCP endpoint      │
                          │  - RegistryService   │
                          │    (skills, legacy)  │
                          │  - Auth interceptor  │
                          │  - rbac package      │
                          └──────────┬───────────┘
                                     │
                          ┌──────────▼───────────┐
                          │  SQLite (WAL mode)   │
                          └──────────────────────┘
```

### 3.2 Backend package layout

```
(in the new nebari-frames repo)

proto/frames/v1/
├── frame.proto             Frame, FrameVersion, Grant, Org messages
└── frame_service.proto     FrameService RPCs

backend/internal/
├── auth/                   seeded from skillsctl; gains org-resolver
├── frames/                 Frame CRUD, versioning, resolution
├── orgs/                   org + membership CRUD
├── rbac/                   single permission-decision point
├── mcp/                    remote MCP endpoint (spec #2)
└── store/sqlite/migrations/
    ├── 001_orgs_and_memberships.sql
    ├── 002_frames.sql
    └── 003_frame_grants.sql
```

`rbac` is the single source of truth for permission decisions. Every read or write path through `frames` consults it; **all enforcement is server-side**. The MCP endpoint is a server-side handler; the Claude client receives only what the server chooses to expose. The web app and CLI are likewise untrusted - the server enforces, clients display.

### 3.3 Data model

```sql
-- Organizations
orgs (
    id           TEXT PRIMARY KEY,        -- ULID
    slug         TEXT NOT NULL UNIQUE,    -- URL-safe, e.g. "openteams"
    display_name TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

-- Org membership + role (MVP: one org per user; PK enforces this)
org_memberships (
    org_id     TEXT NOT NULL REFERENCES orgs(id),
    user_sub   TEXT NOT NULL,             -- OIDC subject claim
    role       TEXT NOT NULL,             -- 'viewer' | 'publisher' | 'admin'
    added_at   TEXT NOT NULL,
    PRIMARY KEY (user_sub)
);

-- Frames
frames (
    id             TEXT PRIMARY KEY,      -- ULID
    org_id         TEXT NOT NULL REFERENCES orgs(id),
    name           TEXT NOT NULL,         -- unique within org
    description    TEXT NOT NULL,
    owner_sub      TEXT NOT NULL,
    latest_version TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    UNIQUE (org_id, name)
);

-- Frame versions; content is a YAML BLOB per version (atomic)
frame_versions (
    frame_id      TEXT NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    version       TEXT NOT NULL,
    content       BLOB NOT NULL,
    digest        TEXT NOT NULL,
    size_bytes    INTEGER NOT NULL,
    published_by  TEXT NOT NULL,
    published_at  TEXT NOT NULL,
    changelog     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (frame_id, version)
);

-- Inheritance edges; pinned per child version
frame_extends (
    frame_id        TEXT NOT NULL,
    version         TEXT NOT NULL,
    parent_frame_id TEXT NOT NULL REFERENCES frames(id),
    parent_version  TEXT NOT NULL,        -- pinned
    order_index     INTEGER NOT NULL,     -- precedence; later overrides earlier on conflict
    FOREIGN KEY (frame_id, version) REFERENCES frame_versions(frame_id, version) ON DELETE CASCADE,
    PRIMARY KEY (frame_id, version, parent_frame_id, parent_version)
);

-- Skip a specific ancestor during resolution
frame_excludes (
    frame_id          TEXT NOT NULL,
    version           TEXT NOT NULL,
    excluded_frame_id TEXT NOT NULL,
    FOREIGN KEY (frame_id, version) REFERENCES frame_versions(frame_id, version) ON DELETE CASCADE,
    PRIMARY KEY (frame_id, version, excluded_frame_id)
);

-- Grants: designed for full per-frame and per-section RBAC; MVP exercises a subset
frame_grants (
    id            TEXT PRIMARY KEY,
    frame_id      TEXT NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    section_id    TEXT,                    -- NULL = whole frame; MVP always NULL
    subject_type  TEXT NOT NULL,           -- 'user' | 'org' (MVP); 'group' later
    subject_id    TEXT NOT NULL,           -- user_sub or org_id depending on type
    permission    TEXT NOT NULL,           -- 'read' | 'edit' | 'delete' | 'share'
    granted_by    TEXT NOT NULL,
    granted_at    TEXT NOT NULL,
    UNIQUE (frame_id, section_id, subject_type, subject_id, permission)
);
```

Key data-model choices:

- **`org_memberships` PK = `user_sub`** enforces one-org-per-user. Migration to multi-org changes the PK to composite `(user_sub, org_id)` plus an "active org" mechanism; contained.
- **`frame_extends` is per-version**. Pinning is part of the version's identity. Republishing a parent does not affect children until they re-publish.
- **`frame_grants.section_id` is nullable from day one**. MVP only writes NULL (whole-frame grants), but the column shape is forward-compatible with section-level RBAC.
- **`frames.name` is unique per org**, not globally. Two orgs can both have a `brand-voice` frame.
- **Frame content is a single YAML BLOB per version**. Atomic, simple, human-inspectable. Per-slot rows were considered and deferred; small slot edits are explicit version events anyway.

### 3.4 Frame schema (content shape)

A Frame is a YAML document with metadata and a fixed set of 10 slots. The schema is **fixed**, not extensible. Extensibility is a future protocol decision, not an MVP property.

```yaml
name: brand-voice                     # [a-z0-9][a-z0-9-]{0,63}, unique within org
description: OpenTeams brand voice    # max 280 chars
version: 1.2.0                        # semver-ish

extends:                              # ordered list; later wins on slot conflict
  - ref: openteams/company-frame      # org_slug/frame_name; same-org may omit slug
    version: 3.1.0
  - ref: industry-consortia/healthcare-compliance
    version: 2024.4

excludes:
  - openteams/legacy-marketing-frame

slots:
  terminology:                        # typed list of {term, definition}
    - term: customer
      definition: An enterprise organization that has deployed an Intelligence Hub.

  rules:                              # typed list of strings
    - Never claim performance numbers without a benchmark citation.

  skills:                             # typed list of strings
    - technical-writing

  prompts:                            # typed list of strings (named structure deferred)
    - When summarizing a release, lead with the customer impact, not the feature list.

  tool_specs: |                       # freeform markdown (typed treatment deferred)
    The Frame expects access to `gh` for GitHub issue retrieval.

  goals: |                            # prose markdown
    ...

  style: |
    ...

  norms: |
    ...

  architecture: |
    ...

  business_process: |
    ...
```

**Slot typing rationale.** The whitepaper lists ten slots without prescribing how they should be structured. We commit to all ten in the schema from MVP (avoids ecosystem churn from adding slots later) but ship conservative typing on four of them:

| Slot | MVP shape | Rationale |
|---|---|---|
| terminology | typed `{term, definition}` | The most clearly list-structured; high leverage; cheap to design well |
| rules, skills, prompts | typed list of strings | Conservative; richer structures (named prompts, severity-tagged rules) are easy additive evolutions later |
| tool_specs | freeform markdown | Genuinely uncertain what shape this should take; freeform until real usage signals |
| goals, style, norms, architecture, business_process | prose markdown | Slots are inherently prose; typed forms would fight authors |

**Resolution algorithm** (executed at read time by `frames.Resolver`):

1. Walk `extends` graph depth-first from the requested Frame version. Abort with error on cycle.
2. Remove any node whose `frame_id` appears in the requested Frame's `excludes`.
3. Merge slots in `extends` order (later wins):
   - `terminology`: merge by `term`; later definition wins on collision.
   - Typed-list-of-strings (`rules`, `skills`, `prompts`): concatenate, dedupe preserving last occurrence.
   - Prose slots (and `tool_specs`): later parent's content replaces earlier entirely.
4. Apply the Frame's own slot values last; they override all parents.
5. Return the resolved Frame.

Resolution at read time (not publish time) keeps storage simple. Pinned parent refs mean read-time results are stable until the child re-publishes.

### 3.5 RBAC model

**Three layers - data model, enforcement, UX - ship independently.**

| Layer | MVP | Roadmap |
|---|---|---|
| Data model | Full grants table (`subject`, `frame_id`, `section_id` nullable, `permission`) | No changes |
| Enforcement | Hardcoded default grants on publish; admin role short-circuits as override | Section-level grants enforced; group subjects; cross-org grants |
| UX | Three-role surface (Viewer / Publisher / Admin) | Grant management UI; per-section visibility editing |

**MVP roles per org:**

- **Viewer** - reads any Frame in their org (via org-wide read grants).
- **Publisher** - reads + publishes new Frames; can edit/delete their own.
- **Admin** - full r/w/delete across all Frames in their org. Bypasses the grants table entirely in the `rbac.Can` check.

**Default grants auto-created on publish:**

```
(user, publisher_sub, frame, NULL, "edit")
(user, publisher_sub, frame, NULL, "delete")
(org,  owner_org_id,  frame, NULL, "read")
```

**`rbac.Can` decision logic:**

1. If `frame.org_id != caller.org_id`: deny. (MVP has no cross-org grants.)
2. If `caller.role == "admin"`: allow.
3. Look up grants matching `(subject_type, subject_id, frame_id, NULL section_id, permission)` for both the user and their org. Allow if any match.
4. Otherwise deny.

**Error response contract:**

| Outcome | Status |
|---|---|
| No / invalid OIDC token | 401 |
| Token valid, no org membership | 403 |
| Caller lacks edit / delete / share permission | 403 |
| Caller lacks read permission on a specific frame | **404** (do not leak existence) |
| Frame doesn't exist | 404 |
| Publish attempted without publisher / admin role | 403 |

The 403-vs-404 distinction for missing read permission is deliberate: returning 403 would leak that a Frame with the requested name exists in some org.

**Not in MVP:** no `AddGrant` / `RevokeGrant` RPCs; no `share` action exercised; no section-level checks; no group subjects. The schema accommodates these; the API does not yet expose them.

### 3.6 MCP endpoint

Detailed design is deferred to a follow-up spec (see [§5 Sub-specs](#sub-specs)). The shape:

- New `backend/internal/mcp/` package, server-side handler registered alongside the ConnectRPC service in `cmd/server`.
- Speaks the MCP protocol; authenticates via OAuth (reuses the existing OIDC stack). Claude.ai (and any other MCP-capable client) adds the Hub as a connector once and OAuths in.
- Exposes the authenticated user's resolved Frame stack as MCP resources. Calls into `frames.Service` like any other consumer; goes through `rbac.Can` for every read; serves only what the caller is entitled to.
- Returns each Frame as composed markdown - all populated slots concatenated under conventional headers. Claude reads it as context.

Because the protocol is MCP, the same endpoint serves any MCP-capable client (ChatGPT, Gemini, Cursor, Windsurf, Codex). Per-client documentation pages for the connector setup are incremental work; the protocol bet pays off on the second client.

### 3.7 Web app

Detailed design is deferred to a follow-up spec.

The MVP web app supports **browse + authoring + connect**. Non-technical users are the people whose Frame content (brand voice, compliance rules, sales playbooks) is most valuable; a browse-only web app would treat them as second-class consumers. Authoring is form-based with typed inputs per slot - no raw YAML in the browser. See [web app design](./2026-05-21-web-app-design.md) §3.4 for the detailed authoring shape. The CLI continues to serve technical authors in parallel.

Web app MVP screens:

- **Catalog.** List of Frames the user can read (filtered server-side by RBAC).
- **Frame detail.** Slot-by-slot rendering of a single Frame, with inheritance trail showing which slots came from which parent.
- **Connect.** Per-provider instructions for adding the Hub as a connector - starting with Claude.ai, expanding to ChatGPT, Gemini, etc. as docs are written.
- **Auth.** SSO via the same OIDC stack as the CLI.

Build target: cross-platform web served from the same backend binary (Go server with embedded static assets). Local-first / offline support is deferred indefinitely; "install a Frame locally" maps to "add it to your active stack on the Hub," and the MCP endpoint serves it on demand.

### 3.8 CLI

A **new CLI binary** ships from the `nebari-frames` repo for technical Frame authors. Binary name is an open question (see OQ9); using the placeholder `frames` here:

```
frames publish --dir ./brand-voice
frames list
frames show openteams/brand-voice
frames resolve openteams/brand-voice@1.2.0     # print resolved (inheritance-merged) Frame
frames extends openteams/brand-voice           # print inheritance tree
frames auth login                              # OIDC device flow, same shape as skillsctl
```

Seeded from the existing `skillsctl` CLI codebase (OIDC device flow, config management, Cobra command structure, API client patterns) but a separate binary with separate distribution. The original `skillsctl` CLI continues to ship from the `skillsctl` repo for Claude Code skill users; nothing about it changes.

## 4. Deviations from the Whitepaper

The whitepaper describes a coherent vision. The MVP described here deviates from it deliberately, in service of shipping working code. Each deviation is listed with rationale and the path back to whitepaper-parity.

| # | Whitepaper says | MVP does | Why | Path to parity |
|---|-----------------|----------|-----|----------------|
| D1 | Frames, Cogs, and Ops are the three execution-layer artifacts | Frames only; Cogs and Ops out of scope | Cogs require a Frame-aware AI worker runtime; Ops require an orchestration layer. Both are full products of their own. Frames alone are the highest-leverage layer and the one a registry can ship now. | Add Cog and Op artifact types as additional sibling tables / services when the products exist to back them. The grants table and org model port over directly. |
| D2 | Selective field-level (section-level) sharing of Frames | Frame-level visibility only | Section-level filtering touches every read path (API, MCP, UI) and complicates authoring UX. The data model accommodates it (`frame_grants.section_id` is nullable from day one). | Implement read-path filtering against `section_id != NULL` grants; add authoring UI for marking sections. No schema migration needed. |
| D3 | Frames inherit across an organizational scope tree (org → dept → team → project → user) | Explicit `extends` only; no scope tree, no implicit inheritance | An org-tree data model plus management UI plus scope-aware RBAC is its own subsystem. Explicit inheritance expresses every whitepaper use case if the author wires it up. | Add an `org_scopes` table and a resolver step that constructs implicit `extends` edges from scope position. The explicit `extends` list remains canonical; implicit edges merge in. |
| D4 | Distribution via Nebi (the packaging and reproducibility layer) | Distribution via direct API / MCP / CLI; no Nebi | Nebi is a separate component whose spec isn't ready to integrate against. | Add a Nebi-compatible package format as an additional publish output; the registry stays the source of truth. |
| D5 | Frames live inside an Intelligence Hub (sovereign, per-org deployment) | This is true in MVP too (the Hub is the skillsctl-derived server) but the doc doesn't yet describe per-org Hub provisioning | Multi-tenant single-Hub serves MVP. Per-org sovereign Hubs is a deployment concern, not a data-model concern. | Document deployment patterns; provide an OpenTofu module similar to other Nebari components. |
| D6 | Cross-org sharing via the marketplace | No cross-org sharing in MVP | Cross-org grants change the RBAC threat model substantially. Defer until intra-org is solid. | Extend `frame_grants` to accept org subjects from other orgs; add a sharing UI and acceptance flow. |
| D7 | Desktop application as the primary user surface | Web app for non-technical users, CLI for technical users; no native desktop in MVP | Desktop apps require build/release infrastructure per OS. A web app covers the non-technical-user goal and ships faster. | Build a Tauri or Electron wrapper around the web app if real users need offline / local-filesystem capability. |
| D8 | Frames feed Cogs running inside the Hub | Frames feed Claude.ai (and other MCP clients) via remote MCP, plus Claude Code via file install (existing CLI path) | We have no Cogs to feed. The audiences we *do* have are existing AI clients. | Backwards compatible. When Cogs exist, they consume Frames through the same `frames.Service` API. |
| D9 | Marketplace flywheel: Frames discovered, installed, exchanged at scale | No public marketplace in MVP; Frames live inside the deploying org's Hub | Marketplace dynamics presuppose a critical mass of orgs running Hubs. We'll have one or two during MVP. | Federate Hub registries when there are multiple to federate. The existing `SkillSource` enum (`INTERNAL`, `FEDERATED`) already anticipates this. |
| D10 | "Frame protocol" as an open specification | No formal protocol spec document in MVP | Premature standardization. We need to see what real authors do with the schema before locking the protocol. | Publish the protocol spec once 3-5 real Frames have shaken out the schema and the data has informed any changes. |

## 5. Migration Path

This is where the "fork or evolve" question gets answered.

### 5.1 The forking decision

`skillsctl` exists. It has users (however few), a public install story, a documentation site, a brand, a governance model, and a stable contract: "publish and install Claude Code skills." The Frame work we're proposing:

- Introduces an entirely new artifact type (Frames) that vastly outsizes the existing one (Skills).
- Adds a new product surface (web app for non-technical users) that has no relationship to the existing CLI-only audience.
- Adds a new distribution channel (MCP for Claude.ai) that has no relationship to skill install.
- Re-frames the project's identity from "skill registry" to "Nebari Frames registry."

If we evolve `skillsctl` in place, the repo's identity becomes muddled. Existing skill users see a project that's now 70% about something else. New Frame users see a project named after the *minor* feature.

**Decision: fork into https://github.com/nebari-dev/nebari-frames.**

Copy the foundational packages from `skillsctl` - auth, store, server skeleton, proto generation toolchain - then build the Frame data model, web app, and MCP endpoint as net-new in the new repo.

`skillsctl` enters **maintenance mode**: no new features, security and dependency updates only. When `nebari-frames` reaches a public release, `skillsctl`'s README gains a prominent supersede notice pointing to it (Frames is the strategic successor; existing skill-registry functionality remains available for Claude Code users who depend on it, but new investment goes into `nebari-frames`).

#### Reasons to fork

- **Identity.** A Nebari product probably wants its own name, its own README, its own documentation site, its own roadmap. Evolving `skillsctl` to be that product would either confuse existing skill users (the repo no longer behaves like the one they installed) or paper over the change with awkward naming (`skillsctl frame ...`).
- **Backwards compatibility.** Existing `skillsctl` users have install scripts, CI references, and docs pointing at the current binary. A fork preserves that contract permanently.
- **Code reuse is small.** The Approach A plan was already to add Frames as a separate package alongside Skills. Forking just makes that separation a repo boundary instead of a package boundary. The shared code (auth, store, proto plumbing) copies cleanly.
- **Release independence.** Frames have a different cadence, audience, and risk profile than Skills. Coupling their releases is friction.
- **Clean break from "skill" terminology.** The proto package, CLI binary name, doc site URL, and Homebrew tap all reflect "skill" today. A new repo lets us name everything for what it actually is.

#### Reasons not to fork

- **One repo is easier to maintain than two.** True, but the maintenance cost is in *people*, not in *repos*. Two small repos by the same maintainers is not meaningfully harder than one large one.
- **Code duplication.** The duplicated code is auth + store + server bootstrapping - a few thousand lines. Worth it for the identity clarity.
- **`skillsctl` users might benefit from the new features.** They wouldn't. The new features (orgs, RBAC, Frames, MCP, web UI) don't apply to skill workflows.

#### What "fork" means concretely

The fork is **not** `git clone && rebrand`. It's:

1. Create a new repo `nebari-frames` (name TBD).
2. Cherry-pick the foundational packages: `backend/internal/auth`, `backend/internal/store` (the SQLite layer and migration tooling), the ConnectRPC server skeleton, the CLI's OIDC device-flow code, the proto generation Makefile targets.
3. Replace `proto/skillsctl/v1/` with `proto/frames/v1/` (no skill messages; only Frame, Org, Grant).
4. Build the Frame service, web app, and MCP endpoint as new work in the new repo.
5. Leave `skillsctl` repo as-is; it continues to ship skill registry binaries for Claude Code users.

The `skillsctl` repo and the new `nebari-frames` repo are not connected via a fork in the GitHub sense (no upstream-merge relationship). They are siblings sharing a heritage.

### 5.2 If we DO evolve in place instead

Documented for completeness. If the fork recommendation is rejected:

1. Add `proto/skillsctl/v1/frame.proto` and `frame_service.proto` alongside the existing skill proto files.
2. Add `backend/internal/frames/`, `backend/internal/orgs/`, `backend/internal/rbac/`, `backend/internal/mcp/` packages.
3. Add migrations `003_orgs_and_memberships.sql`, `004_frames.sql`, `005_frame_grants.sql`.
4. Add `cli/cmd/frame_*` subcommand files.
5. Add `web/` directory for the web app.
6. Rename `skillsctl.dev` documentation to cover both artifact types, or shard it.
7. Decide whether the binary stays named `skillsctl` or gets a new name; the answer is awkward either way.

Step 7 is where the in-place option keeps producing friction every time we touch user-facing surfaces.

### 5.3 Sequencing

Regardless of fork-or-evolve, the work sequences:

1. **Spec #1 - this document - data model + RBAC foundation** (in progress).
2. **Spec #2 - MCP endpoint design** (depends on #1).
3. **Spec #3 - web app design** (depends on #1; can run in parallel with #2).
4. **Implementation of #1**: protos, migrations, `frames`, `orgs`, `rbac` packages, basic CLI subcommands, default-grant logic. No MCP, no web app yet.
5. **Implementation of #2 (MCP endpoint)** and **#3 (web app)** can proceed in parallel.
6. **Three hand-authored example Frames** (Brand Voice, Healthcare Compliance, Q4 Sales Playbook from the whitepaper) created during implementation of #1 to pressure-test the schema before locking it.

## 6. Open Questions

- ~~**OQ1: Fork or evolve?**~~ **Resolved 2026-05-21:** fork into https://github.com/nebari-dev/nebari-frames. `skillsctl` enters maintenance mode and gets a supersede notice in its README pointing to the new repo when `nebari-frames` is public.
- ~~**OQ2: Repository name for the fork.**~~ **Resolved 2026-05-21:** `nebari-frames` under the `nebari-dev` org.
- **OQ3: Frame ID and slug conventions.** The doc proposes ULIDs as internal IDs and `org_slug/frame_name` for human refs. Worth confirming with the people who'll write the example Frames.
- **OQ4: Prompts slot shape.** MVP ships prompts as typed-list-of-strings. The whitepaper's framing ("reusable prompt fragments") suggests `{name, body}` pairs might fit better. Defer to first real Frame; revisit if authors want named prompts.
- **OQ5: Org provisioning UX.** How does a new org get created and its first admin assigned? Manual via SQL, CLI admin command, or web-based onboarding? MVP probably wants a CLI admin command. Spec #3 (web app) might add a self-serve flow later.
- **OQ6: Where do MVP example Frames live?** Inside the fork's `examples/` directory, or in a separate `nebari-frames-examples` repo? Lean toward in-repo for MVP; spin out when the catalog grows.
- **OQ7: How are Hub deployments shipped?** OpenTofu module like other Nebari components? Helm chart? Docker Compose for local development? Probably all three eventually; OpenTofu first to align with the rest of Nebari.
- **OQ8: What happens to `skillsctl`'s existing `marketplace_id` and `upstream_url` fields on `skills`?** They suggest a federation concept that was sketched but not built out. Frames may want a similar concept eventually. Not blocking MVP.
- **OQ9: Name of the new CLI binary.** Candidates: `frames`, `nebari-frames`, `nebari frames` (subcommand of a future `nebari` CLI). Placeholder `frames` is used throughout this doc. Decision blocks the Homebrew formula and install script.

## 7. Alternatives Considered

### 7.1 Build a new Frame registry from scratch (no reuse of skillsctl)

Rejected. The expensive parts of a registry (versioned content store, OIDC auth with device flow, ConnectRPC API surface, CLI distribution) are exactly what `skillsctl` already has. Throwing that away to start fresh trades months of work for cosmetic cleanliness.

### 7.2 Unify Skills and Frames into one polymorphic artifact type

Rejected. The schemas barely overlap. A `skills` row has tags and an OCI ref; a `frames` row has org membership, inheritance edges, and section-level grants. Forcing them into one table either bloats the row with nulls or hides differences behind a JSON sidecar that fights any typed access. The polymorphism would leak into the API and confuse both audiences.

### 7.3 Full RBAC from day one (per-frame and per-section grant management UI in MVP)

Rejected on YAGNI grounds. The data model is built to support full granular RBAC (the `frame_grants` table accepts section-level grants out of the box), but the *UX* and *enforcement matrix* for granular management is large enough to be its own product. Three roles cover the credible-enterprise floor; full RBAC is additive.

### 7.4 Section-level visibility from day one

Rejected for MVP, accepted for roadmap. Genuinely valuable but touches every read path. Data model accommodates it.

### 7.5 Implicit inheritance via an organizational scope tree

Rejected for MVP, accepted for roadmap. Explicit `extends` expresses every whitepaper use case if the author is willing to wire it up; implicit inheritance is a convenience layer that can be added later non-breakingly.

### 7.6 Native desktop app instead of web app for non-technical users

Rejected for MVP. Real product value (offline, local-filesystem write for non-Claude tools) but doubles the shipping work for an MVP that's trying to validate the Frame concept itself. Web app covers the validation goal; desktop is additive.

### 7.7 Build provider-specific connectors (Claude, ChatGPT, Gemini) instead of betting on MCP

Rejected. MCP is the protocol every major provider supports as of 2026. One MCP endpoint serves N clients with ~80% effort savings versus N native integrations. Per-provider docs and edge-case testing are the remaining 20%, incurred only when each provider is brought online.

## 8. Security and Privacy Considerations

- **All RBAC enforcement is server-side.** The `rbac/` package is the single decision point for every read and write path through `frames`. Clients (web UI, CLI, MCP) are untrusted.
- **404 instead of 403 on missing read permission** prevents frame-existence leaks across org boundaries.
- **OIDC tokens validated on every request** by the existing auth interceptor; identity is the OIDC `sub` claim.
- **OAuth for MCP connector** is the same OIDC stack; no new auth code path.
- **Frame content can contain sensitive organizational information** (brand strategy, compliance rules, internal terminology). Even with frame-level visibility, the right model is "don't put cross-org-sensitive material in a Frame intended for cross-org sharing." Section-level visibility (roadmap) lets authors mark genuinely-internal sections.
- **Audit logging** is not in MVP. Hub admins cannot retrospectively see "who read what Frame when." Roadmapped: an `audit_log` table populated by the `rbac` decision points.
- **Rate limits and abuse prevention** are not in MVP. The same OIDC auth that authenticates legitimate users limits abuse adequately for early-access scale; production rollout to broader audiences will need rate-limiting.

## 9. Testing Strategy

Drawn from the existing `skillsctl` testing patterns (`go test ./...` with race detector; table-driven tests per `CLAUDE.md`).

- **Unit tests** for `rbac` decision logic (allow/deny matrix for every combination of role, action, and frame ownership).
- **Unit tests** for the `frames.Resolver` (cycles, excludes, slot-merging semantics for each slot type, deep extends graphs).
- **Integration tests** against a real SQLite database (migration apply, default-grant insertion on publish, query plans for the grant-lookup hot path).
- **Schema validation tests** with fixture Frames that exercise every slot type, including invalid Frames that should be rejected with specific errors.
- **End-to-end tests** for CLI publish → backend list → backend get → CLI fetch.
- **Hand-authored example Frames** for Brand Voice, Healthcare Compliance, Q4 Sales Playbook serve as the canonical schema pressure test; included in the test fixture set.

MCP endpoint and web app testing strategies live in their respective specs (#2, #3).

## 10. Rollout

- **Alpha (internal):** deploy to OpenTeams-internal Hub; three real Frames authored by internal teams to validate the schema and authoring ergonomics.
- **Closed beta:** invite 3-5 friendly orgs to publish their own Frames; collect feedback on slot coverage, inheritance ergonomics, and RBAC adequacy.
- **Open beta:** public docs site and self-serve org creation; CLI and web app generally available. At this point, add the supersede notice to `skillsctl`'s README pointing to `nebari-frames`.
- **GA gate:** real audit logging, rate limits, deployment hardening, OpenTofu Nebari module published.

## 11. Roadmap (Post-MVP)

In rough priority order, drawn from the deferred items above:

1. Section-level Frame visibility (data model already supports it).
2. Grant management UI (per-user, per-group, per-section read/edit/share editing).
3. Cross-org Frame sharing (extend grants to accept other-org subjects; add acceptance flow).
4. Multi-org membership for individual users (consultants, contractors).
5. Implicit inheritance via organizational scope tree.
6. ~~Authoring of Frames in the web app~~ — in MVP scope as of 2026-05-21 design review; see web app design doc §3.4.
7. Per-provider connector pages (ChatGPT, Gemini, Codex, Cursor, Windsurf, etc.).
8. File-install connector path for Frames that need to land on local disk (Claude Code, Cowork, Codex CLI).
9. Nebi packaging integration when Nebi specs are ready.
10. Federation between Hubs.
11. Cogs and Ops, if and when they exist as products.
12. Formal "Frame protocol" specification document.

## 12. Sub-specs

This design doc covers the foundation. Two follow-up specs are required before full implementation can proceed:

- **Spec #2: MCP endpoint design.** Resource shape, OAuth flow, content composition, per-provider client compatibility notes. Depends on this doc landing.
- **Spec #3: Web app design.** Screens, component breakdown, build/deploy story, accessibility considerations. Depends on this doc landing; can run in parallel with #2.

This design doc was produced through a brainstorming session that began in the `skillsctl` repo; the WIP brainstorming notes from that session stayed in `skillsctl` and are not reproduced here. This document is the canonical design artifact.

---

## Appendix A: Glossary

- **Frame.** A scoped, text-based artifact carrying organizational context (terminology, style, goals, rules, etc.). The unit of distribution in this product.
- **Hub / Intelligence Hub.** A deployed instance of the Nebari Frames server inside an organization's perimeter. The whitepaper's term for "the place a Frame lives."
- **MCP.** Model Context Protocol; the open standard for connecting AI clients to external resource and tool servers. Anthropic-originated, now supported by all major model providers.
- **Org.** Organization. A tenant boundary in the data model. Each Frame belongs to exactly one org.
- **Slot.** A named, fixed portion of a Frame's content (e.g., `terminology`, `style`). The Frame schema is a fixed set of slots with mixed typed/prose treatment.
- **Resolver.** The component that produces a final, merged Frame from a base Frame plus its `extends` chain minus its `excludes` list.
- **Grant.** A row in `frame_grants` granting a subject (user or org) a permission (read / edit / delete / share) on a frame (whole or section).
- **YAGNI.** "You Aren't Gonna Need It." A bias against speculative engineering. This document leans on it heavily.
