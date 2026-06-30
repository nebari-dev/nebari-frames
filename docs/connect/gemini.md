# Connect Gemini to Nebari Frames (MCP)

As of mid-2026, Google's consumer **Gemini app** does not offer a general
"add a custom remote MCP server with OAuth" flow the way Claude.ai and ChatGPT
do. The supported paths for a remote, OAuth-protected MCP server like Nebari
Frames are:

- **Gemini CLI** (recommended for a developer demo), and
- **Gemini Enterprise** (admin-registered OAuth client; heavier setup).

This doc covers the Gemini CLI path.

## Prerequisites

- [Gemini CLI](https://geminicli.com) installed.
- A Keycloak account in the cluster's realm (e.g. `nebari`).
- The realm allows DCR from the Gemini CLI's loopback redirect, **or** a Keycloak
  client is pre-registered for the CLI (see note below). The MCP audience mapper
  must be in place (see [keycloak-setup.md](./keycloak-setup.md)).

## Steps (Gemini CLI)

1. Add the server to `~/.gemini/settings.json`:

   ```json
   {
     "mcpServers": {
       "nebari-frames": {
         "httpUrl": "https://<frames-host>/mcp"
       }
     }
   }
   ```

   (e.g. `https://frames.dcmcand-llm.openteams.dev/mcp`)

2. Start `gemini`, then authenticate to the server:

   ```
   /mcp auth nebari-frames
   ```

   This opens a browser OAuth flow. **Sign in with your Nebari (Keycloak)
   account** and approve. The CLI exchanges the code and stores the token.

3. Confirm the connection and tools:

   ```
   /mcp list
   ```

   You should see `nebari-frames` connected with the `list_frames` and
   `get_frame` tools.

## Using it

Prompt, e.g.:

> Using the nebari-frames tools, write an elevator pitch for Nebari.

Gemini calls `list_frames` then `get_frame nebari-platform` and writes from the
Frame.

## Note on OAuth registration

Unlike Claude.ai/ChatGPT, the Gemini CLI's MCP OAuth may not use Dynamic Client
Registration. If `/mcp auth` fails to register a client, register a public
client in the Keycloak realm with the CLI's loopback redirect
(`http://localhost:<port>/oauth2callback` or as the CLI reports) and reference
its `client_id` in the server's OAuth config per the Gemini CLI docs. Verify the
exact redirect URI against the CLI's output.

## References

- [MCP servers with Gemini CLI](https://geminicli.com/docs/tools/mcp-server/)
- [Set up your custom MCP server data store - Gemini Enterprise](https://docs.cloud.google.com/gemini/enterprise/docs/connectors/custom-mcp-server/set-up-custom-mcp-server)
