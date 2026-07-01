# Connect ChatGPT to Nebari Frames (MCP)

ChatGPT can use an organization's Frames as a remote MCP connector. It registers
itself via Dynamic Client Registration (DCR) against Keycloak, signs the user in
with their Nebari account, and then exposes the `list_frames` and `get_frame`
tools in the conversation.

## Prerequisites

- **ChatGPT with Developer mode:** custom MCP apps run through Developer mode.
  ChatGPT Apps are available on **all plans** (as of 2025-11-13); your workspace
  admin must permit developer mode.
- **A Keycloak account in the cluster's realm** (e.g. `nebari` at
  `https://keycloak.<domain>`).
- **The realm must allow DCR from ChatGPT's host.** ChatGPT registers from
  OpenAI's servers with a `chatgpt.com` redirect URI, so the realm's anonymous
  "Trusted Hosts" client-registration policy must permit it (either allow all
  hosts, or add `chatgpt.com` to the trusted hosts). See
  [keycloak-setup.md](./keycloak-setup.md).
- The MCP audience mapper must be in place so ChatGPT's token carries
  `aud=https://<frames-host>/mcp` (see keycloak-setup.md).

## Steps

1. In ChatGPT, open **Settings → Apps → Advanced settings** and turn on
   **Developer mode**.
2. Back on **Apps**, click **Create** to open the **New App** dialog.
3. Fill in:
   - **Name:** `Nebari Frames`
   - **Connection:** leave on **Server URL** (not **Tunnel**), and paste
     `https://<frames-host>/mcp`
     (e.g. `https://frames.dcmcand-llm.openteams.dev/mcp`)
   - **Authentication:** `OAuth`
4. Tick **"I understand and want to continue"**, then click **Create**. ChatGPT
   reads the protected-resource metadata and registers a client via **DCR**
   (CIMD is skipped because the server doesn't advertise it), then opens the
   OAuth login. **Sign in with your Nebari (Keycloak) account** and approve.
5. The app shows **connected** with two tools: `list_frames` and `get_frame`.

## Using it

Ask, e.g.:

> Using Nebari Frames, write an elevator pitch for Nebari.

ChatGPT calls `list_frames` to discover Frames, `get_frame nebari-platform` to
load the content, and writes grounded in that Frame (respecting its rules).

## Troubleshooting

- **"Couldn't register" / registration error:** the realm's Trusted Hosts policy
  is rejecting ChatGPT's host. Allow `chatgpt.com` (or all hosts) for anonymous
  DCR.
- **Registration fails on scopes:** ChatGPT requests `openid email profile` by
  default. Keycloak's DCR "Allowed Client Scopes" policy must permit each of
  them; if only `openid` is allowed, add `email` and `profile` (see
  keycloak-setup.md).
- **Connects but tool calls 401:** the token lacks the `/mcp` audience. Confirm
  the audience mapper is on a scope every client gets (see keycloak-setup.md).
- **No tools shown:** confirm the server is the tool-bearing build (it exposes
  `list_frames`/`get_frame`), not resources-only.

## References

- [Developer mode and MCP apps in ChatGPT - OpenAI Help Center](https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt)
- [Building MCP servers for ChatGPT - OpenAI developers](https://developers.openai.com/api/docs/mcp)
