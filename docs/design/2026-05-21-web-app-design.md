# Design Doc: Nebari Frames Web App

| | |
|---|---|
| **Status** | Draft - pending review |
| **Author** | Chuck McAndrew |
| **Created** | 2026-05-21 |
| **Last updated** | 2026-05-21 |
| **Reviewers** | @dharhas, @jbouder |
| **Depends on** | [Nebari Frames Migration](./2026-05-21-nebari-frames-migration.md) |
| **Companion doc** | [MCP Endpoint Design](./2026-05-21-mcp-endpoint-design.md) |

## TL;DR

A web application that lets non-technical users discover, **author**, edit, and connect Frames in their organization. Frames are surfaced through a polished browse UI; new Frames are created through either of two editors over the same document - a form with typed inputs per slot (terminology row editor, list editors for rules / skills / prompts, markdown textareas for prose slots, parent-Frame picker for `extends`) or a raw [Frame Spec v0.2](https://github.com/openteams-ai/frame-spec) `.frame.md` source editor that doubles as the import surface; and once published, users can wire their AI client of choice to the Hub's MCP endpoint through guided per-provider connector pages.

This is the primary user-facing surface for everyone who isn't a CLI user. The app is a Single-Page Application built with React + Vite + TypeScript + Tailwind, served as embedded static assets from the same Go binary as the backend. Auth is OIDC standard authorization code flow with PKCE. All RBAC enforcement is server-side; the web app is presentation only.

## 1. Background

### 1.1 Audience

The Frames product targets two distinct audiences:

- **Technical users** (engineers, ops, data scientists) - comfortable in a terminal, want fast iteration, will install a CLI and author Frames in their editor.
- **Non-technical users** (marketing, sales, legal, HR, executives, anyone in product or content roles) - won't install a CLI, but absolutely want to author Frames - "this is our brand voice," "this is how we handle compliance," "this is the playbook for Q4 sales." Their input is the most valuable Frame content in the whole system.

The CLI serves the first audience. The web app serves the second. The MCP endpoint serves both. Authoring is therefore in the web app for MVP - browsing only would treat non-technical users as second-class consumers of content authored by engineers, which inverts where the actual organizational value lives.

### 1.2 Why a web app, not a desktop app

The migration doc rejected desktop as the MVP user surface (D7). Web apps ship faster, install nowhere, and cover the MVP need. Desktop is a roadmap option for users who genuinely need offline / local-filesystem capability.

### 1.3 What the migration and MCP docs settled

- All RBAC enforcement is server-side.
- All auth flows reuse the existing OIDC stack with separate web-app OAuth client registration.
- The MCP endpoint serves Frames to AI clients; the web app provides connector setup instructions per provider.
- The CLI continues to serve technical users in parallel; authoring is available in both surfaces.

## 2. Goals and Non-Goals

### 2.1 Goals

| # | Goal |
|---|------|
| G1 | Let a non-technical user log in and see the Frames they're entitled to read |
| G2 | Let a Publisher author a new Frame through a typed form (no raw YAML) and publish it |
| G2a | Let a Publisher who prefers the spec format author, import, and export a Frame as a `.frame.md` document |
| G3 | Let an author edit and publish new versions of Frames they own |
| G4 | Provide step-by-step per-provider instructions for adding the Hub as an MCP connector (Claude.ai first) |
| G5 | Allow org admins to manage org membership and have admin-override access to all org Frames |
| G6 | Ship as a single Go binary alongside the backend (no separate frontend deploy) |
| G7 | Look and feel polished enough that a non-technical user trusts it |

### 2.2 Non-Goals (MVP)

| # | Non-Goal | Deferred to |
|---|----------|-------------|
| N1 | Rich WYSIWYG markdown editing | Roadmap; markdown textareas with optional preview is enough |
| N2 | Live multi-user collaboration (presence, simultaneous edits) | Roadmap; lock-on-edit at most for MVP |
| N3 | Draft autosave with conflict resolution | Roadmap; MVP is form -> Publish -> new version, no separate draft state |
| N4 | Active-stack curation UI (picking default Frames for MCP) | Roadmap; AI clients' resource pickers cover MVP |
| N5 | Sharing UI (granting cross-user or cross-org access) | Roadmap; MVP has no cross-org sharing at all |
| N6 | Frame feedback / scoring UI (whitepaper's -10..+10 + suggested-edits) | Roadmap |
| N7 | Multi-language / i18n | Roadmap |
| N8 | Mobile-optimized layouts | Best-effort responsive; not a primary target |
| N9 | Section-level visibility editing | Roadmap, blocked on backend section-level RBAC (migration doc D2) |
| N10 | Analytics dashboards / install-count visualizations | Roadmap |

## 3. Proposal

### 3.1 Tech stack

| Layer | Choice | Why |
|---|---|---|
| Framework | **React 19+** | Largest ecosystem, broadest familiarity, easy to staff |
| Build | **Vite** | Fast HMR, well-supported with Tailwind and TS |
| Language | **TypeScript** | Catches bugs at build; required for the ConnectRPC client to be usable |
| Styling | **Tailwind CSS** | Fast iteration, consistent design tokens |
| Component library | **shadcn/ui** (copy-in primitives) | Polished without runtime lock-in; we own components, can customize |
| Routing | **React Router** | Standard, sufficient |
| API client | **ConnectRPC TypeScript client** (generated from proto) | Type-safe end-to-end with the backend |
| Server state | **TanStack Query (React Query)** | Caching, retries, mutation flow |
| Form state | **react-hook-form + zod** | Typed validation; works well with shadcn |
| Auth | **OIDC PKCE in the browser** via `oidc-client-ts` | Standard SPA pattern |
| Markdown rendering | **react-markdown + remark/rehype + rehype-sanitize** | Frame slot content rendering |
| Markdown editing | **react-textarea + a small preview toggle** | YAGNI on a full editor; textarea+preview is plenty for MVP |
| Tests | **Vitest** + **RTL**; **Playwright** for E2E | Standard stack |

### 3.2 Deployment model

The compiled web app is embedded into the Go server binary via `embed.FS`. The same server binary that serves the ConnectRPC API and MCP endpoint also serves the SPA static assets at `/`. SPA-style routing (catch-all returning `index.html`) is handled by the Go server.

Build: `make build-web` runs `npm ci && npm run build` in `web/`, outputs to `web/dist/`, gets embedded at Go compile time. CI runs both Go and web tests; release binaries include the web app baked in.

One artifact, one deploy, one URL. No CORS. Trade-off: the web app cannot update independently of the backend. Acceptable for MVP.

### 3.3 Page / screen list

| Route | Page | Purpose |
|---|---|---|
| `/` | Catalog | List of Frames the user can read in their org. Search and filter. |
| `/frames/:org/:name` | Frame Detail | Full view of a single Frame: all slot content rendered, inheritance trail, version history, "Use this Frame in..." panel, Edit button (if entitled). |
| `/frames/new` | Frame Authoring (create) | Empty form for a brand-new Frame. |
| `/frames/:org/:name/edit` | Frame Authoring (edit) | Same form, pre-filled from the latest version of the existing Frame. |
| `/connect` | Connect Hub | Landing page for connector setup; tiles per provider. |
| `/connect/:provider` | Connect (per-provider) | Step-by-step instructions for adding the Hub to that provider. |
| `/admin` | Admin home | Org admin landing; only visible to admins. |
| `/admin/members` | Membership management | Add / remove users; set roles. |
| `/admin/frames` | Frame management | Admin override surface: see all org Frames; delete; transfer ownership (v1.1). |
| `/login` | Login | OIDC start. |
| `/auth/callback` | OAuth callback | Standard OIDC callback handler. |
| `/no-access` | Request-access landing | Shown to authenticated users with no org membership. |

### 3.4 Frame Authoring (the headline new surface)

The authoring page is a **document editor**: it keeps the shape of the page the reader will see,
because the spec's thesis is that a Frame *is* a Markdown document. View and edit share one
single-column, content-first layout; editing means editing that document in place.

- **Title and description** are quiet inline inputs styled as the document's heading - a visible
  border appears on hover/focus so they stay discoverable as fields.
- **Spec metadata** (`visibility` select, `scope`, `maintainer`) is one compact row under the
  description. `visibility` is declared intent that travels with the document - `frame_grants`
  remains the sole access-control authority.
- **Composition** (Inherits from / Excludes) sits between metadata and content, the way the
  frontmatter it maps to sits above a document body.
- **Sections are added on demand.** Only sections that carry content (or that the author added
  this session) are on the page; an **"+ Add section" menu** lists the rest, each with a one-line
  hint ("Rules - hard constraints the AI must follow"), so the schema documents itself at the
  moment it is needed and a new frame is never a wall of empty inputs. Every visible section has
  a hover-revealed "Remove section" control that clears it.
- **Version and changelog are publish-time decisions**, not document content: the Publish button
  opens a small dialog with the version (pre-filled - `1.0.0` for a first frame, patch-bumped by
  `suggestNextVersion` on edit) and the changelog. A version conflict (AlreadyExists) keeps the
  dialog open with the error on the version input; any other failure closes it so the inline
  field errors are visible.

```
┌──────────────────────────────────────────────────────┐
│  NEW FRAME                     [⋯] [Cancel] [Publish…]│
│                                                        │
│  frame-name                    (inline title input)    │
│  What is this frame for?       (inline description)    │
│  Visibility [internal▾]  Scope [    ]  Maintainer [  ] │
│  ┌──────────────────────────────────────────────────┐  │
│  │ INHERITS FROM  [+ Add parent Frame]              │  │
│  │ EXCLUDES       [+ Add exclusion]                 │  │
│  └──────────────────────────────────────────────────┘  │
│  ──────────────────────────────────────────────────    │
│  Rules                          (hover: Remove section)│
│   Hard constraints the AI must follow.                 │
│   • Never claim performance numbers…          [Remove] │
│   [+ Add rule]                                         │
│  ──────────────────────────────────────────────────    │
│  [+ Add section ▾]   (Terminology, Skills, Goals, …)   │
└──────────────────────────────────────────────────────┘
```

**Markdown is a secondary mode**, not a header toggle: "Edit as Markdown" lives in the ⋯ overflow
menu (alongside "Preview as resolved Frame"), swaps the page for the `.frame.md` source editor
with a "Back to editor" action, and is never persisted as a preference. Publishing from Markdown
takes the version from the document's own frontmatter, so the publish dialog offers only the
changelog there. Import (`/frames/new?import=1`, reached from the catalog's "Import .frame.md"
button) lands straight in the Markdown editor with paste/drop/Load file.

~~**Editor kinds per section** are unchanged: terminology is a two-column row editor, rules/skills/
prompts are single-column row editors, prose sections are markdown textareas with a preview
toggle. The section list (keys, labels, editor kind, hints) lives once in
`web/src/lib/slot-sections.ts`, mirroring the Go `SlotTable`; the read-only renderer
(`FrameSlots`) and the editor both consume it.~~

> **Superseded by [#59](https://github.com/nebari-dev/nebari-frames/issues/59):** there are no
> sections to have editor kinds for. A Frame's content is a single free-form markdown body,
> matching Frame Spec v0.2, so authoring is one markdown editor with starter templates and the
> `.frame.md` source mode described above. `web/src/lib/slot-sections.ts` and the Go `SlotTable`
> it mirrored are both gone; `web/src/lib/frame-templates.ts` supplies the starting points that
> the per-section hints used to. Versions published under the ten-slot schema are still readable:
> the backend folds a legacy `slots:` block into a body on read (`backend/internal/frames/legacy.go`).

**The view page mirrors the editor.** Frame Detail leads with identity (name, version badges,
visibility/scope badges, description, maintainer, inherits chips linking to parents) and then the
document itself, unboxed at readable width. Registry bookkeeping - owner, publisher, timestamps,
size, digest, changelog - is demoted to a collapsed "Details" disclosure in the sidebar, which
otherwise carries "Use this Frame" (MCP URI), version history, and the hierarchy link. Export is
a small menu (Download `.frame.md` / Copy as Markdown) available to anyone who can read the frame.

**Validation feedback.** Two phases, as before: zod client-side, then server `FieldViolations` on
publish. Every field renders its own message through the shared `FieldError` component
(`components/form/FieldError.tsx`), which also wires `aria-invalid` / `aria-describedby`. ~~Server
paths use bracket notation (`slots.rules[0]`) while inputs register dotted paths
(`slots.rules.0`); react-hook-form's `get` resolves both to the same node, so they meet on the
input that caused them.~~ (Superseded by [#59](https://github.com/nebari-dev/nebari-frames/issues/59):
with one body field there are no indexed slot paths to reconcile.) Cycle detection in `extends`
remains a form-level banner.

**The `.frame.md` codec.** Conversion lives only in Go (`backend/internal/frames/framemd.go`) and is
reached through one stateless `ConvertFrame` RPC, so the codec is not mirrored into TypeScript.
Round-tripping is covered by a golden corpus over `examples/`.

Parsing is strict about the **frontmatter** and never rejects the **body**:

- *Structural* (blocks conversion): unknown frontmatter key, missing or unterminated frontmatter,
  bad `type`. Errors name the line. The frontmatter delimiter is matched only at column 0, so an
  indented `---` inside a multi-line YAML value is content rather than a terminator.
- *Value* (does not block): an unpinned or unqualified `inherits`, an empty description. These
  convert successfully and land as fixable inline errors in the form.

The body itself is free-form under Frame Spec v0.2, so there are no headings to validate and no
"did you mean" suggestions to make - whatever is after the closing `---` is the content.

That split is what makes import usable: the spec's own `examples/complete/frame.md` uses
`inherits: editorial-style-guide` - bare name, no org, no pinned version - which a single strict
gate would reject outright. Because slot bodies are delimited by `##`, an author must use `###` or
deeper for headings inside a section; the editor says so inline.

**Save behavior.** No draft state: the document holds in memory until Publish, which inserts a new
`frame_versions` row. Publishing from Markdown converts first, so the server only ever stores
canonical slot YAML and there is exactly one publish path. Edit is the same page, prefilled, with
the name rendered as a fixed title.

### 3.5 Catalog and Frame Detail

The Catalog (`/`) is intentionally simple:

- Search box (filters by name and description; client-side fuzzy match for MVP, server-side once catalogs grow).
- Tile or row layout (responsive); each Frame shows name, description, owner, last-updated, version.
- "Create new Frame" button (visible to Publishers and Admins) prominently top-right.

Frame Detail (`/frames/:org/:name`) is the highest-value reading screen:

- Header: name, description, version, owner, "Edit" / "Delete" buttons (visible per server-returned permissions).
- **"Use this Frame" panel** (right rail on desktop, top section on mobile): one-click links to per-provider Connect pages; code block showing the MCP resource URI for users who know what to do with it.
- **Inheritance trail**: visual representation of the `extends` chain. Each parent is clickable and links to its detail page.
- ~~**Slot rendering**: all populated slots rendered in their typed form (terminology as a definition list; rules / skills / prompts as bullet lists; prose slots as rendered markdown). Empty slots hidden. Each section collapsible.~~ **Superseded by [#59](https://github.com/nebari-dev/nebari-frames/issues/59):** the body renders as markdown, whatever structure its author gave it. The detail page is the authoring form rendered read-only, so reading and editing are one surface rather than two renderers to keep in step.
- **Version history**: collapsed by default; expandable to see all published versions with timestamps and changelogs. Each version row links to its read-only detail page (no in-app diff in MVP; roadmap).

### 3.6 Per-provider Connect pages

Each `/connect/:provider` page is a short tutorial:

1. Where in the provider's UI to add a connector
2. The connector URL to paste (the Hub's `/mcp` endpoint)
3. OAuth flow walkthrough
4. How to verify it worked (try a Frame-aware prompt)

Claude.ai page at launch. ChatGPT and Gemini added when their docs are written. Cursor, Windsurf, Codex, Claude Code added on demand. Each page has a copy-button for the connector URL, a screenshot or two from the target provider, and a footer noting last-verified date.

### 3.7 Admin pages

`/admin/members` is a single-page CRUD over `org_memberships`:

- Table of current members: identifier (OIDC email or sub), role, added-at.
- "Add member" form: user sub or email + role.
- Inline role editor.
- Remove button (admin can revoke membership).

Server-side RBAC gates the route; UI gates display, server enforces.

`/admin/frames` lists every Frame in the admin's org with a delete action. Admin override is enforced server-side. Transfer-ownership is roadmap.

### 3.8 Auth flow

Standard OIDC authorization code with PKCE:

1. User clicks "Log in"; web app redirects to OIDC authorize endpoint with PKCE challenge.
2. User authenticates; OIDC redirects to `/auth/callback` with code.
3. Web app exchanges code + verifier for access + refresh token.
4. Tokens stored in `sessionStorage` (shorter lifetime; no localStorage XSS-persisted risk across tabs).
5. Web app calls `GetMe` to populate identity + role from the backend.
6. Subsequent API calls send `Authorization: Bearer <token>`.
7. Refresh on 401 (or proactively at half-lifetime).

If the authenticated user has no org membership, route to `/no-access` explaining that an org admin needs to add them. No auto-create of memberships in MVP.

### 3.9 RBAC in the UI

UI hides what the user shouldn't see; server enforces. Two layers, server is canonical.

- Catalog only lists Frames the API returns (server-filtered by read grants).
- Admin nav items hidden for non-admins.
- "Edit" / "Delete" / "Create" buttons shown only when the API tells the client the action is permitted. The Frame Detail response includes `permissions: { canEdit, canDelete }` alongside the Frame content; the catalog response includes `canCreate` on the user payload.

### 3.10 Accessibility

Target: WCAG 2.1 AA.

- Keyboard navigation works for all interactive elements (especially the form's row editors and pickers).
- Form labels associated with inputs.
- Color contrast for body text and interactive elements.
- ARIA roles on custom components (shadcn defaults; verify with axe-core).
- Focus management on route changes.
- Markdown rendering preserves heading hierarchy.

Not in MVP: full screen-reader audit, RTL support, high-contrast theme.

## 4. Open Questions

- **OQ1: Component library choice.** shadcn/ui is the working assumption. Alternative: MUI, Chakra, Radix Primitives raw. shadcn gives us ownership without runtime lock-in; the trade-off is maintaining the components ourselves. Confirm with whoever does the frontend work.
- **OQ2: Markdown editor library.** Plain `<textarea>` plus a Preview button is the MVP working assumption. Alternatives: TipTap, ProseMirror-based editors, Lexical. All are heavier and overkill for the slot-prose use case. Revisit if author feedback says textarea is too thin.
- **OQ3: Frame picker UX.** The `extends` picker is the most complex single component. Should it be a typeahead autocomplete (search as you type), a modal browser (browse the catalog in a popup), or both? Lean toward typeahead for MVP simplicity.
- **OQ4: Embedded vs separate static host.** MVP embeds the SPA in the Go binary. Kubernetes-with-CDN deployments would prefer the SPA on a CDN, API on the Go binary. Not blocking MVP; revisit when a deployment context demands it.
- **OQ5: OIDC web client registration steps.** Need to coordinate with whoever owns OIDC provider configuration in each Hub. Document registration steps in the Hub deployment guide (not yet written).
- **OQ6: Marketing-level branding.** Logo, color palette, typography. Not blocking design but blocking a polished-feeling product. Coordinate with whoever owns Nebari Frames branding.
- **OQ7: Request-access flow.** MVP shows a static "ask your admin" page. A self-serve "request access from admin X with optional message" flow with notifications is roadmap; confirm timing.
- **OQ8: Concurrent edit conflicts.** Two Publishers edit the same Frame simultaneously and both Publish. Both versions land in `frame_versions` (atomic per-version inserts), but the second clobbers the first's `latest_version` pointer. MVP: last-writer-wins on `latest_version` with no warning. Roadmap: optimistic concurrency check showing the conflict.

## 5. Alternatives Considered

### 5.1 Browse-only web app, authoring CLI-only

Rejected (reversed during design review). Browsing alone treats non-technical users as second-class consumers of content authored by engineers; the audience whose input most matters (marketing, legal, sales) would have no way to contribute Frames. Adding form-based authoring costs ~30-40% more work for substantially higher product value.

### 5.2 Raw YAML editor in browser

Rejected for the *primary* authoring surface: it defeats the purpose of a non-technical UI. Delivered instead as the secondary "Markdown" editor, in the spec's `.frame.md` form rather than raw slot YAML - the format authors outside this app already use, which makes the same surface serve import and export.

### 5.3 Server-rendered with HTMX

Rejected for MVP. HTMX produces small fast apps but the Frame authoring form benefits from rich client-side validation and dynamic row editors. Revisit if React proves overkill in practice.

### 5.4 Next.js or Remix

Rejected. SSR is valuable for SEO and first-paint; Frame Hub is behind auth; nothing is indexable; first-paint cost is acceptable. The added complexity of a Node-side rendering server isn't earned.

### 5.5 Separate frontend deployment (CDN + API)

Rejected for MVP. Adds CORS, two deploys, two URLs. Easy to migrate to later.

### 5.6 Native desktop (Tauri / Electron)

Rejected for MVP. Roadmap option for users with offline / local-filesystem needs.

### 5.7 Rich WYSIWYG markdown editor

Rejected for MVP. Textarea + preview is plenty for slot content. WYSIWYG is a bottomless pit of edge cases (paste handling, inline images, custom marks) and not what authors need. Roadmap if textareas prove insufficient.

## 6. Security Considerations

- **Tokens in `sessionStorage`** (not `localStorage`) to limit XSS exfiltration and cross-tab persistence.
- **No secrets in web app code.** OAuth client ID is public (by PKCE design); no client secret.
- **CSP headers** on the Go server serving the SPA: tight `default-src 'self'`; `connect-src` includes backend origin and OIDC discovery.
- **All API calls authenticated.** No anonymous endpoints exposed to the SPA.
- **RBAC mirrored in UI, enforced in server.** Either layer alone insufficient.
- **Markdown rendering uses `rehype-sanitize`** with an HTML element allowlist. No raw HTML injection.
- **Form input sanitization on the server.** The client form sends structured data; server validates against the schema. Never trust client validation.
- **OIDC redirect URI allowlist.** Only deployed Hub URLs registered.

## 7. Testing Strategy

- **Unit tests** (Vitest) for non-trivial components: catalog filter, inheritance-trail rendering, slot renderers, form row editors, Frame picker.
- **Integration tests** (RTL) for page flows: log in, browse catalog, open Frame detail, navigate inheritance trail, copy MCP URI, create Frame via form, publish, edit existing Frame.
- **E2E tests** (Playwright) for the auth flow end-to-end against Dex in CI, plus a happy-path authoring flow.
- **Accessibility test pass** with axe-core in CI on every page.
- **Visual regression** via Playwright screenshots for Frame Detail (highest-fidelity surface) and the authoring form.

## 8. Rollout

- **Alpha:** the web app is the primary alpha surface (with the MCP endpoint). Internal users log in, author the three example Frames *in the web app*, set up Claude.ai connector, verify Frames flow into Claude.
- **Closed beta:** Connect docs for Claude.ai polished; admin pages working for invited orgs; authoring UX refined based on alpha feedback.
- **Open beta:** self-serve org creation; ChatGPT and Gemini Connect pages live.
- **GA:** branding pass, polish, accessibility audit, real audit logging.

## 9. Roadmap (Post-MVP)

1. **Diff view** between Frame versions on the version-history panel.
2. **Pre-publish preview** showing the inheritance-resolved Frame.
3. ~~**"View as YAML" power-user toggle** in the authoring form.~~ - shipped as the `.frame.md` Markdown editor; see §3.4.
4. **Active-stack curation** (user picks default Frames for MCP).
5. **Sharing UI** for cross-user and cross-org grants.
6. **Feedback / scoring UI** (whitepaper's -10..+10 with suggested edits).
7. **Optimistic concurrency** on edits (warn when two authors are publishing the same Frame).
8. **Section-level visibility editing** (once backend section-level RBAC ships).
9. **Search improvements:** server-side full-text search; tag/scope filters.
10. **Per-provider Connect pages** beyond Claude/ChatGPT/Gemini.
11. **Native desktop wrapper** (Tauri) for users with offline or local-filesystem write needs.
12. **Self-serve "request access" flow** with admin notifications.
13. **Analytics dashboards:** install counts, popular Frames, org-scale inheritance graph visualization.
14. **Frame templating** - "start from this Frame as a template" workflow for common Frame patterns.
