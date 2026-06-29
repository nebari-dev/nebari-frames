# Claude.ai Manual Test Plan: Nebari Frames MCP Connector

This document is the manual test procedure for verifying that the Nebari Frames MCP endpoint integrates correctly with Claude.ai. It serves as the evidence template for **Journey 1: Claude.ai user connects to Frames via OAuth and retrieves Frame content**.

## Prerequisites

Before starting this test, ensure:
- Your Keycloak realm is configured per `docs/connect/keycloak-setup.md`.
- The Frames server is running with `FRAMES_PUBLIC_URL` set to your deployment URL (e.g., `https://frames.example.com`).
- You have a valid Frames organization and at least one readable Frame authored or shared with your user account.
- You have access to a Claude.ai account.

## Test Environment

Record the following before beginning:

| Item | Value |
|------|-------|
| **Frames Public URL** | (e.g., `https://frames.example.com`) |
| **Keycloak Realm URL** | (e.g., `https://keycloak.example.com/realms/myrealm`) |
| **Test User Email** | (e.g., `test@example.com`) |
| **Test Date** | (e.g., `2026-06-26`) |
| **Test Environment** | (e.g., `staging`, `production`, `local`) |

## Manual Test Steps

### Step 1: Add Custom Connector in Claude.ai

1. Log in to [claude.ai](https://claude.ai).
2. Navigate to **Settings** (bottom-left menu or account icon).
3. Click **Connectors**.
4. Click **Add custom connector** or **+ Add connector**.
5. In the dialog, enter your Frames MCP endpoint URL:
   ```
   <FRAMES_PUBLIC_URL>/mcp
   ```
   Example: `https://frames.example.com/mcp`
6. Click **Connect** or **Verify**.

**Expected result:**
- Claude probes the URL and detects it as an MCP resource server.
- A dialog appears prompting you to authorize Claude.ai to access Frames on your behalf.
- No errors appear (if an error occurs, note it in the Findings section).

**Capture screenshot:** Screenshot of the authorization dialog or any prompts from Claude.ai.

---

### Step 2: Redirect to Keycloak Login

The browser automatically redirects to your Keycloak realm's login page.

**Expected result:**
- The browser navigates to your Keycloak realm's login page (URL contains `/realms/` and `/protocol/openid-connect/auth`).
- The login form displays username/email and password fields.
- No error page or SSL/TLS warnings appear.

**Capture screenshot:** Screenshot of the Keycloak login page.

---

### Step 3: Authenticate and Grant Consent

1. Enter your Keycloak credentials (the user account that owns or has access to Frames in your organization).
2. Click **Sign In** or **Log In**.
3. If prompted, **Grant consent** to allow the newly registered Claude.ai client to access Frames on your behalf.

**Expected result:**
- Login succeeds without errors.
- If a consent screen appears, granting consent completes without error.
- The browser redirects back to Claude.ai.

**Capture screenshot:** Screenshot showing the redirect back to Claude.ai and the connector status.

---

### Step 4: Verify Connector Connection and Resource Discovery

After the redirect, you are back in Claude.ai.

1. Verify that the Frames connector now shows a status of **"Connected"** or **"Active"** (not "Disconnected" or "Error").
2. Open a new chat (or a chat that will use the connector).
3. In the message compose area, look for the connector's **resource picker** or **Frame selector**. This may appear as:
   - A button labeled **+ Attach** or **Attach connector**.
   - A list of available Frames in a sidebar or dropdown.
4. Click on the connector's resource picker to expand the list of available Frames.

**Expected result:**
- The resource picker loads without errors.
- Your organization's readable Frames are listed by name.
- Each Frame is clearly labeled with its name (e.g., "Engineering Handbook", "Product Guidelines").
- No Frames appear that you do not have access to.

**Capture screenshot:** Screenshot of the resource picker showing the list of available Frames.

---

### Step 5: Include a Frame and Request Content

1. Select one Frame from the resource picker and include it in your chat message.
2. Ask Claude to quote or summarize a specific section, or ask a general question about the Frame's content.
   - Example: "Quote the Terminology section from this Frame."
   - Example: "What are the main rules outlined in this document?"
3. Submit the message.

**Expected result:**
- Claude retrieves and displays the Frame content.
- Claude correctly reproduces sections of the Frame's composed markdown content.
- Section headings (e.g., "## Terminology", "## Rules") are preserved and visible.
- The response is accurate and coherent (no truncation or garbled content).
- No authentication errors or timeouts occur.

**Capture screenshot:** Screenshot of Claude's response displaying Frame content.

---

## Results

### Test Status

| Item | Result |
|------|--------|
| **Test Date** | (e.g., `2026-06-26`) |
| **Test Environment** | (e.g., `staging`) |
| **Tester** | (e.g., `test@example.com`) |
| **Overall Status** | **PASS** / **FAIL** |

### Step-by-Step Results

Record the result of each step:

| Step | Expected Outcome | Observed | Status |
|------|------------------|----------|--------|
| 1 | Authorization dialog appears | | PASS / FAIL |
| 2 | Redirect to Keycloak login | | PASS / FAIL |
| 3 | Redirect back to Claude.ai, connector shows "Connected" | | PASS / FAIL |
| 4 | Frames listed in resource picker | | PASS / FAIL |
| 5 | Claude retrieves and quotes Frame content correctly | | PASS / FAIL |

### Findings

If any step fails or behaves unexpectedly, document the issue here:

```
Issue 1:
- Step: [which step]
- Expected: [what was expected]
- Observed: [what actually happened]
- Error message: [if any]
- Action taken: [what you did to investigate or work around it]

Issue 2:
- (repeat as needed)
```

### Evidence Artifacts

Attach or reference the screenshots captured in each step:
- Step 1: Authorization dialog
- Step 2: Keycloak login page
- Step 3: Connector status and redirect
- Step 4: Resource picker with Frames listed
- Step 5: Claude's response with Frame content

---

## Troubleshooting

**Issue: Authorization dialog does not appear**
- Check that `FRAMES_PUBLIC_URL` is correctly set and publicly accessible.
- Verify that the MCP endpoint metadata is available at `<FRAMES_PUBLIC_URL>/.well-known/oauth-protected-resource`.
- Check browser console for errors (F12 > Console tab).

**Issue: Keycloak login fails or times out**
- Verify network connectivity to your Keycloak instance.
- Check that `OIDC_ISSUER_URL` is correctly configured on the Frames server.
- Confirm your Keycloak instance is running and accessible.

**Issue: After login, redirect does not return to Claude.ai**
- Verify that the redirect URI `https://claude.ai/api/mcp/auth_callback` is not blocked by a firewall or WAF.
- Check Keycloak logs for OAuth redirect errors.

**Issue: Connector shows "Error" or "Disconnected" after step 3**
- Verify the token includes the correct audience claim (`aud: <FRAMES_PUBLIC_URL>/mcp`).
- Check that the audience protocol mapper is configured in Keycloak (see `docs/connect/keycloak-setup.md`).
- Review the Frames server logs for auth validation errors.

**Issue: Frames do not appear in the resource picker**
- Verify that your user account has read access to at least one Frame in your organization.
- Check the Frames server logs for errors in the resource list endpoint.
- Confirm that `FRAMES_PUBLIC_URL` is set correctly on the server.

**Issue: Claude returns a 401 error or cannot access Frame content**
- Verify the token is valid and not expired.
- Check that the MCP server is running and reachable.
- Review the Frames server logs for access control errors.

---

## References

- [Frames Documentation](../README.md)
- [Keycloak Setup Guide](./keycloak-setup.md)
- [Nebari Frames MCP Endpoint Architecture](../architecture.md)
