# Keycloak Setup for Nebari Frames MCP Endpoint

This document outlines the prerequisites for configuring Keycloak to support the Nebari Frames MCP (Model Context Protocol) endpoint. The MCP server acts as an OAuth 2.1 resource server, with Keycloak as the authorization server.

## Prerequisites

Before proceeding, ensure you have:
- Administrative access to your Keycloak realm
- The public URL of your Frames deployment (e.g., `https://frames.example.com`)
- Network connectivity from Keycloak to the MCP server

## Setup Checklist

### 1. Enable Dynamic Client Registration (RFC 7591)

Dynamic Client Registration (DCR) allows Claude.ai to automatically register itself as an OAuth client without manual intervention.

**Steps:**
1. In your Keycloak realm, navigate to **Realm Settings** > **General**.
2. Locate the **Dynamic Client Registration** settings.
3. Choose one of the following policies:
   - **Anonymous registration policy**: Allows any client to register without authentication (simpler, less secure).
   - **Initial Access Token (IAT) policy**: Requires an access token issued by the realm administrator (more secure, requires pre-issued tokens).

**Important note:** Claude registers a fresh OAuth client per connection via DCR. Over time, this can result in client record accumulation in your realm. Plan for periodic cleanup of unused DCR-registered clients if necessary.

### 2. Add an Audience Protocol Mapper (REQUIRED)

Keycloak does not natively honor the RFC 8707 `resource` parameter used by the MCP server to request a specific audience. Without this step, tokens will carry `aud: account` instead of the required audience, and the MCP server will reject authentication requests with a 401 error.

**Steps:**
1. Navigate to **Realm Settings** > **Client Scopes**.
2. Find or create a client scope named `roles` (or select an existing default scope).
3. Click on the scope and go to the **Mappers** tab.
4. Click **Create mapper** and select **Audience**.
5. Configure the mapper:
   - **Mapper name**: (e.g., `mcp-audience` or `frames-mcp-aud`)
   - **Included Client Audience**: Leave empty to apply globally.
   - **Add to access token**: ON
   - **Add to ID token**: OFF (ID tokens do not need the audience claim)
6. **Audience value**: Enter the canonical resource identifier: `<FRAMES_PUBLIC_URL>/mcp`
   - Example: `https://frames.example.com/mcp`

Alternatively, if you prefer to add this mapper to a **default client scope** (so it applies to all newly registered clients automatically):
1. Go to **Realm Settings** > **Client Scopes** > **Default Client Scopes**.
2. Ensure your audience-mapped scope is in the **Assigned Default Scopes** list.

Reference Keycloak's official documentation: https://www.keycloak.org/securing-apps/mcp-authz-server

### 3. Confirm Code Challenge Methods

The MCP server and Claude.ai use the OAuth 2.1 Authorization Code flow with PKCE (Proof Key for Code Exchange) for security.

**Steps:**
1. In your Keycloak realm, navigate to **Realm Settings** > **General**.
2. Scroll to **Endpoints**.
3. Click **OpenID Endpoint Configuration** to view the realm's metadata.
4. In the JSON response, search for `code_challenge_methods_supported`.
5. Confirm it includes `S256` (SHA-256, the standard PKCE method).

Keycloak includes `S256` by default; no action is typically required. If the list is absent or missing `S256`, contact your Keycloak administrator.

### 4. Redirect URI Configuration

DCR-registered clients declare their own redirect URIs during registration. Claude.ai (web, desktop, mobile) uses a unified redirect URI:

```
https://claude.ai/api/mcp/auth_callback
```

This URI is baked into the Claude.ai client; no manual configuration in Keycloak is needed. The redirect URI will appear automatically in the DCR-registered client record after the first OAuth flow.

### 5. WAF / Network Allowlist

If Keycloak sits behind a restrictive Web Application Firewall (WAF), OAuth discovery and registration traffic from Anthropic originates from the CIDR block:

```
160.79.104.0/21
```

If OAuth flows silently fail (no error message, just timeout or silent redirect), check your WAF logs and allowlist this CIDR range.

## Server Environment Variables

The Frames server requires the following environment variables to enable the MCP endpoint:

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `FRAMES_PUBLIC_URL` | Yes | Public scheme and hostname of your Frames deployment. The MCP endpoint is only mounted if this is set. | `https://frames.example.com` |
| `OIDC_ISSUER_URL` | Yes | The Keycloak realm URL (the OpenID Connect issuer). | `https://keycloak.example.com/realms/myrealm` |
| `OIDC_MCP_AUDIENCE` | No | The OAuth 2.0 resource audience required by tokens. Defaults to `<FRAMES_PUBLIC_URL>/mcp` if not set. | `https://frames.example.com/mcp` |
| `FRAMES_DEV_MODE` | No | If set to `true`, disables OAuth authentication for local development only. Do not use in production. | `true` (dev only) |

## Verification

Once configured, verify the setup by:

1. **Check metadata discovery**: Visit `<OIDC_ISSUER_URL>/.well-known/openid-configuration` and confirm it includes the realm's OAuth endpoints.
2. **Check resource metadata**: Visit `<FRAMES_PUBLIC_URL>/.well-known/oauth-protected-resource` and confirm the `resource` field equals `<FRAMES_PUBLIC_URL>/mcp` and `authorization_servers` lists your Keycloak realm.
3. **Test OAuth flow**: Use the Claude.ai test plan (see `docs/connect/claude-ai.md`) to verify the full authentication flow.

## Troubleshooting

**Issue: MCP endpoint not accessible**
- Verify `FRAMES_PUBLIC_URL` is set in the server configuration.
- Confirm it matches the hostname users will use to access the MCP endpoint.

**Issue: 401 Unauthorized on token requests**
- Verify the audience protocol mapper is configured and applied to the default client scope.
- Confirm tokens include `aud: <FRAMES_PUBLIC_URL>/mcp` (check with a JWT decoder).
- Verify `OIDC_MCP_AUDIENCE` matches the audience in tokens.

**Issue: OAuth redirect silently fails or times out**
- Check WAF logs for blocked requests from `160.79.104.0/21`.
- Allowlist the CIDR range in your WAF rules.

**Issue: DCR client registration fails**
- Verify Dynamic Client Registration is enabled on the realm.
- If using IAT policy, confirm the client has a valid initial access token.
- Check Keycloak logs for registration errors.

## References

- [Keycloak Securing Applications Guide - MCP/Resource Server](https://www.keycloak.org/securing-apps/mcp-authz-server)
- [RFC 7591: OAuth 2.0 Dynamic Client Registration Protocol](https://tools.ietf.org/html/rfc7591)
- [RFC 8707: OAuth 2.0 Authorization Framework Resource Indicators](https://tools.ietf.org/html/rfc8707)
- [RFC 7636: OAuth 2.0 Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
