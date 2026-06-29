# Nebari Frames - Manual QA Plan (MCP Endpoint)

Covers running the MCP endpoint locally and exercising all 8 definition-of-done
journeys by hand, from a fast no-auth local pass through to a full Keycloak +
Claude.ai end-to-end. Pairs with the automated tests in
`backend/internal/mcp/` (every journey except #1 has automated coverage; this
plan is the human confirmation and the journey-1 gate).

Journey numbers match the spec at
`docs/superpowers/specs/2026-06-26-mcp-endpoint.md` section 4.

## 0. Architecture recap (what talks to what)

- **MCP endpoint**: part of the same Go binary as the FrameService. It is an
  OAuth 2.1 **resource server only**. It exposes:
  - `POST/GET /mcp` - the MCP protocol over **Streamable HTTP** (official
    `modelcontextprotocol/go-sdk`). Bearer-protected in auth mode; open in dev mode.
  - `GET /.well-known/oauth-protected-resource` - public RFC 9728 metadata
    pointing clients at the authorization server (Keycloak).
- **Authorization server**: Keycloak (NOT the Frames server). Claude.ai runs
  OAuth 2.1 + PKCE directly against Keycloak, registering via Dynamic Client
  Registration (DCR). The Frames server never sees the password and never issues
  tokens; it only validates them.
- **Audience binding (RFC 8707)**: the MCP endpoint validates that a token's
  `aud` is the canonical `<FRAMES_PUBLIC_URL>/mcp`. A token minted for the web or
  CLI client (different `aud`) is rejected with 401. Keycloak needs an audience
  protocol-mapper to stamp this `aud` (see `docs/connect/keycloak-setup.md`).
- **Resources**: each Frame the caller can read is one MCP resource, URI
  `nebari-frame://<org>/<name>@<version>`, served as composed markdown.
- **Env that controls it**:
  - `FRAMES_PUBLIC_URL` - public scheme+host. REQUIRED in production to mount
    `/mcp`. Sets the canonical resource URL and default audience.
  - `OIDC_MCP_AUDIENCE` - optional override; defaults to `<FRAMES_PUBLIC_URL>/mcp`.
  - `OIDC_ISSUER_URL` - the Keycloak realm (the `authorization_servers` entry).
  - `FRAMES_DEV_MODE=true` - disables bearer auth on `/mcp` (local only); injects
    the fixed `dev-user` identity. The component still mounts and metadata is
    still served.

## Tools you will use

- **MCP Inspector** (recommended for the protocol journeys):
  `npx @modelcontextprotocol/inspector`. A browser UI that speaks MCP; point it
  at the `/mcp` URL, then use the Resources tab to list and read.
- **Claude Code as a local MCP client** (alternative to Inspector, dev mode):
  `claude mcp add --transport http frames-dev http://localhost:8080/mcp`, then in
  a session the resources appear under the connector.
- **curl** - for the plain-HTTP journeys (metadata document, the 401 challenge)
  and for seeding frames via the FrameService in dev mode.
- **jq** - pretty-print JSON.
- (Tier 3) **Keycloak** and a **Claude.ai** account.

---

## Journeys at a glance

| # | Journey | Where to test |
|---|---------|---------------|
| 1 | Claude.ai connect via OAuth/DCR | Tier 3 (manual; see `docs/connect/claude-ai.md`) |
| 2 | RBAC-filtered resource list | Tier 1 (positive) + Tier 3 (negative: viewer cannot see a restricted frame) |
| 3 | Composed-markdown read | Tier 1 |
| 4 | Unauthenticated `/mcp` -> 401 challenge | Tier 2 |
| 5 | Protected-resource metadata discoverable | Tier 1 |
| 6 | Wrong-audience token rejected | Tier 3 |
| 7 | Denied/missing read both -> not-found (no leak) | Tier 1 (missing) + Tier 3 (denied) |
| 8 | Dev-mode `/mcp` works without Keycloak | Tier 1 |

---

## Tier 1 - Local dev mode (no OIDC), ~10 minutes

No external services. Exercises the protocol, resources, composition, metadata,
and dev-mode access (journeys 2-positive, 3, 5, 7-missing, 8).

### 1.1 Build and run in dev mode with the MCP endpoint enabled

```bash
cd /home/chuck/devel/nebari-frames
make build                          # builds ./nebari-frames-server
rm -f qa-mcp.db                     # start clean
FRAMES_DEV_MODE=true \
FRAMES_PUBLIC_URL=http://localhost:8080 \
OIDC_ISSUER_URL=http://localhost:8081/realms/frames \
SEED_ORG_SLUG=openteams \
SEED_ORG_DISPLAY_NAME="OpenTeams" \
SEED_ADMIN_SUB=dev-user \
DB_PATH=qa-mcp.db \
./nebari-frames-server
# logs: "WARNING: FRAMES_DEV_MODE=true - authentication DISABLED ..."
#       "starting server on :8080"
# NOTE: dev mode prints NO "MCP endpoint enabled" line (that log is emitted only
#       in auth mode). The endpoint is still mounted - confirm via the metadata
#       check in 1.3.
```

Notes:
- `FRAMES_DEV_MODE=true` disables auth on `/mcp` (no token needed) and makes the
  injected `dev-user` the seeded org admin.
- `OIDC_ISSUER_URL` is set here only so the metadata document (journey 5) shows a
  realistic authorization server. Keycloak does NOT need to be running for Tier 1.
- `FRAMES_PUBLIC_URL=http://localhost:8080` makes the canonical resource URL and
  metadata sensible for local clients.

### 1.2 Seed two readable frames (dev mode needs no token)

The FrameService takes the Frame YAML as base64 bytes over Connect JSON. Seed a
content-rich frame (so the read shows multiple markdown sections) and a second
plain one (so the list shows more than one):

```bash
# Frame 1: brand-voice (terminology + rules + goals)
YAML1=$(cat <<'EOF'
name: brand-voice
description: How we speak to customers.
version: 1.0.0
slots:
  terminology:
    - term: Frame
      definition: A scoped context artifact.
  rules:
    - Be concise and concrete.
    - Prefer active voice.
  goals: Sound human, not corporate.
EOF
)
curl -s localhost:8080/frames.v1.FrameService/PublishFrame \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"$(printf '%s' "$YAML1" | base64 -w0)\",\"changelog\":\"seed\"}" | jq '.frame.name, .version.version'
# expect: "brand-voice", "1.0.0"

# Frame 2: support-tone (plain)
YAML2=$(cat <<'EOF'
name: support-tone
description: Tone for support replies.
version: 1.0.0
slots:
  rules:
    - Acknowledge the issue first.
EOF
)
curl -s localhost:8080/frames.v1.FrameService/PublishFrame \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"$(printf '%s' "$YAML2" | base64 -w0)\",\"changelog\":\"seed\"}" | jq '.frame.name'
# expect: "support-tone"
```

### 1.3 Journey 5 - protected-resource metadata is discoverable and correct

```bash
curl -s localhost:8080/.well-known/oauth-protected-resource | jq
```

**Expected:**
- `resource` == `http://localhost:8080/mcp` (the canonical `/mcp` URL).
- `authorization_servers` == `["http://localhost:8081/realms/frames"]` (the issuer).
- `scopes_supported` is a non-empty array (`["openid","email","profile"]`).
- `bearer_methods_supported` contains `"header"`.

### 1.4 Journeys 8, 3, 2 (positive) - connect, list, read via MCP Inspector

```bash
npx @modelcontextprotocol/inspector
# opens a browser UI (default http://localhost:6274)
```

In the Inspector UI:
1. Set **Transport Type** = `Streamable HTTP`.
2. Set **URL** = `http://localhost:8080/mcp`.
3. Click **Connect**.

   **Journey 8 (dev-mode access) - Expected:** connection succeeds with no token /
   no OAuth prompt. The session initializes.

4. Open the **Resources** tab and click **List Resources**.

   **Journey 2 (positive list) - Expected:** both seeded frames appear:
   - `Brand Voice (OpenTeams)` -> `nebari-frame://openteams/brand-voice@1.0.0`
   - name shows `<frame name> (OpenTeams)`; URI is the `nebari-frame://` scheme.
   (Hold the negative half - "a frame you cannot read is absent" - for Tier 3,
   since dev-user is an admin and can read everything.)

5. Click the `brand-voice` resource and **Read** it.

   **Journey 3 (composed markdown) - Expected:** the content is `text/markdown`
   and renders as:
   ```
   # Frame: brand-voice

   How we speak to customers.

   > Resolved at: <ISO timestamp>

   ## Terminology

   - **Frame**: A scoped context artifact.

   ## Rules

   - Be concise and concrete.
   - Prefer active voice.

   ## Goals

   Sound human, not corporate.
   ```
   Confirm: named sections in fixed order; EMPTY slots (Skills, Style, Norms,
   etc.) are OMITTED entirely (no empty headers); no `> Inherits from:` line for
   this non-inheriting frame.

> Alternative without Inspector (Claude Code as the client):
> `claude mcp add --transport http frames-dev http://localhost:8080/mcp`
> then start `claude`, and the two frames are available as connector resources.
> Remove afterward: `claude mcp remove frames-dev`.

### 1.5 Journey 7 (missing) - reading a nonexistent frame returns not-found

In the Inspector Resources tab, use **Read Resource** with a hand-typed URI for a
frame that does not exist:

```
nebari-frame://openteams/does-not-exist@1.0.0
```

**Expected:** a resource-not-found error (the same error a real-but-unreadable
frame returns - see Tier 3 journey 7 for the denied half). Also try a malformed
URI like `nebari-frame://openteams/../secret@1.0.0` - same not-found, no
distinguishing detail.

### Tier 1 coverage
Journeys 5, 8, 3, 2 (positive), 7 (missing). The RBAC-negative halves of 2 and 7
require a non-admin user and a restricted frame, which dev mode (single admin
identity) cannot express - do those in Tier 3.

---

## Tier 2 - Unauthenticated challenge (auth mode, no Keycloak needed), ~3 minutes

Journey 4 only. The bearer middleware rejects a tokenless request BEFORE
contacting the authorization server, so Keycloak does not need to be reachable -
the OIDC env values just need to be set so the server starts in auth mode.

### 2.1 Run in auth mode

Stop the Tier 1 server. Then:

```bash
cd /home/chuck/devel/nebari-frames
rm -f qa-mcp.db
FRAMES_PUBLIC_URL=http://localhost:8080 \
OIDC_ISSUER_URL=http://localhost:8081/realms/frames \
OIDC_CLIENT_ID=frames-web \
SEED_ORG_SLUG=openteams \
SEED_ORG_DISPLAY_NAME="OpenTeams" \
SEED_ADMIN_SUB=placeholder-sub \
DB_PATH=qa-mcp.db \
./nebari-frames-server
# logs: "auth enabled (issuer: ...)" and
#       "MCP endpoint enabled (resource: http://localhost:8080/mcp)"
# (the OIDC provider being unreachable is fine for this test)
```

### 2.2 Journey 4 - unauthenticated `/mcp` returns a 401 challenge

```bash
curl -i -s -X POST localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | head -20
```

**Expected:**
- Status line `HTTP/1.1 401 Unauthorized`.
- A `WWW-Authenticate: Bearer ...` header whose value contains
  `resource_metadata="http://localhost:8080/.well-known/oauth-protected-resource"`.

Confirm the metadata pointer is present (this is what lets a client discover the
authorization server). The metadata document itself is still public:

```bash
curl -s localhost:8080/.well-known/oauth-protected-resource | jq .resource
# expect: "http://localhost:8080/mcp" (served without a token)
```

### Tier 2 coverage
Journey 4.

---

## Tier 3 - Full Keycloak + Claude.ai, ~30-45 minutes first-time

Journeys 1, 6, and the RBAC-negative halves of 2 and 7. Requires a Keycloak realm
configured per `docs/connect/keycloak-setup.md` (DCR enabled + the audience
protocol-mapper that stamps `aud = <FRAMES_PUBLIC_URL>/mcp`) and a publicly
reachable `FRAMES_PUBLIC_URL` over HTTPS (Claude.ai must reach it; use a tunnel
such as `cloudflared`/`ngrok` if testing from a laptop).

### 3.1 Prerequisites
1. Keycloak realm configured per `docs/connect/keycloak-setup.md`. Verify the
   audience mapper is a realm DEFAULT client scope so DCR-registered clients
   inherit it.
2. Frames server running in auth mode with `FRAMES_PUBLIC_URL` = your public
   HTTPS URL, `OIDC_ISSUER_URL` = the realm, `OIDC_CLIENT_ID` set, and at least
   one published frame.
3. At least two Keycloak users: one org admin (or publisher) and one **viewer**
   who is a member of the org. Seed the admin via `SEED_ADMIN_SUB`/`SEED_ADMIN_EMAIL`.

### 3.2 Journey 1 - Claude.ai connect via OAuth/DCR

Run the full manual walkthrough in **`docs/connect/claude-ai.md`** (5 steps:
add connector -> Keycloak login -> consent -> resource picker lists frames ->
include a frame and confirm composed content). Record PASS/FAIL and screenshots
in that document's evidence template. This is the journey-1 gate.

### 3.3 Journey 6 - a token minted for a different audience is rejected

Goal: prove the MCP endpoint accepts only tokens whose `aud` is the canonical
`/mcp` URL.

1. Obtain a token whose `aud` is a DIFFERENT client (e.g. the web/CLI client
   `frames-web`, which does NOT carry the MCP audience mapper). For example, via
   the Keycloak token endpoint with a direct-grant test client:
   ```bash
   WRONG=$(curl -s -X POST \
     "$OIDC_ISSUER_URL/protocol/openid-connect/token" \
     -d grant_type=password -d client_id=frames-web \
     -d username=<viewer-user> -d password=<pass> -d scope=openid \
     | jq -r .access_token)
   ```
2. Present it to `/mcp`:
   ```bash
   curl -i -s -X POST "$FRAMES_PUBLIC_URL/mcp" \
     -H "Authorization: Bearer $WRONG" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | head -5
   ```
   **Expected:** `HTTP/1.1 401 Unauthorized` (wrong audience rejected).

3. Now obtain a token that DOES carry `aud = <FRAMES_PUBLIC_URL>/mcp` (a client
   or scope with the audience mapper applied - the same path Claude uses) and
   present it the same way.
   **Expected:** NOT 401 (a 200/400/JSON-RPC response - any non-401 proves the
   audience check passed and the request reached the MCP protocol layer). Decode
   both tokens at jwt.io and confirm the `aud` claim differs as described.

### 3.4 Journey 2 (negative) + Journey 7 (denied) - no cross-user leakage

Set up a frame the viewer cannot read: publish a frame whose grants do NOT
include org-read for the viewer (e.g. owned by the admin with default grants
removed, or in practice a frame in a different org). Then, connected to `/mcp`
as the **viewer** (via Claude.ai or MCP Inspector with the viewer's token):

- **Journey 2 (negative):** the restricted frame does NOT appear in the resource
  list.
- **Journey 7 (denied):** attempting to read its URI directly returns the SAME
  not-found error as a nonexistent frame (no "exists but forbidden" signal, no
  different status or message). Compare against the Tier 1 missing-frame result -
  they must be indistinguishable.

### Tier 3 coverage
Journeys 1, 6, 2 (negative), 7 (denied).

---

## Smoke checklist (quick pass/fail)

- [ ] (T1) Server starts in dev mode; logs "MCP endpoint enabled at /mcp".
- [ ] (T1, J5) `/.well-known/oauth-protected-resource` returns resource=`<url>/mcp`, the issuer, and non-empty scopes.
- [ ] (T1, J8) MCP Inspector connects to `/mcp` with no token.
- [ ] (T1, J2+) List shows the seeded frames with `name (Org)` labels and `nebari-frame://` URIs.
- [ ] (T1, J3) Read returns `text/markdown` with named sections, empty slots omitted.
- [ ] (T1, J7) Reading a nonexistent / malformed URI returns not-found.
- [ ] (T2, J4) Tokenless POST `/mcp` -> 401 with `WWW-Authenticate` containing `resource_metadata`.
- [ ] (T3, J1) Claude.ai connector completes OAuth and lists + reads frames (`docs/connect/claude-ai.md`).
- [ ] (T3, J6) Wrong-`aud` token -> 401; correct-`aud` token -> not 401.
- [ ] (T3, J2-/J7) Viewer does not see a restricted frame; denied read == missing read.

## Teardown

```bash
# stop the server (Ctrl-C)
rm -f qa-mcp.db
claude mcp remove frames-dev 2>/dev/null || true   # if added
docker rm -f kc 2>/dev/null || true                # if Keycloak was used
```
