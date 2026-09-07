# E2E Test Token Acquisition from Keycloak

This document describes how an automated e2e test can acquire a Keycloak token that the Nebari Frames backend actually accepts.

## Overview

The Nebari Frames backend validates incoming tokens against Keycloak's OIDC configuration. Automated tests must acquire a token that:
1. Is issued by the configured OIDC issuer (e.g., http://localhost:8081/realms/frames)
2. Has the correct audience claim (frames-web)
3. Is issued for an existing user in Keycloak

## Important: Build Freshness

The dev stack runs a prebuilt `./nebari-frames-server` binary at the repository root (gitignored) rather than building from the current source tree. This binary can become stale and serve code older than the working tree. If you are testing recently added server behavior, run `make build` before starting the stack, or you will test old code and misread the result as a missing feature.

This document verifies token acceptance against GetMe and ListFrames endpoints because they exist in any build; these tests isolate the question of whether the backend accepts the token from the question of which build is running. Newer template RPCs were confirmed working separately against a freshly built server.

## Test Environment Setup

### Start the dev stack:
```bash
make dev-auth
```

This starts:
- Keycloak on http://localhost:8081
- Backend API on http://localhost:5173

### Keycloak Configuration

The dev realm (`dev/keycloak/frames-realm.json`) includes:
- Realm name: `frames`
- Seeded users: dev / dev, alice / alice, bob / bob
- Default web client: `frames-web` with `directAccessGrantsEnabled: false`

The `frames-web` client has direct access grants disabled to support the PKCE flow used by the web app. For automated tests that need password grant, a purpose-provisioned test client is required.

## Working Token Acquisition Flow

### 1. Create a test client via Keycloak Admin API

Acquire an admin token:
```bash
ADMIN_TOKEN=$(curl -s -X POST "http://localhost:8081/realms/master/protocol/openid-connect/token" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=admin" \
  -d "password=admin" | jq -r '.access_token')
```

Create the test client `frames-test`:
```bash
curl -s -X POST "http://localhost:8081/admin/realms/frames/clients" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "frames-test",
    "enabled": true,
    "publicClient": true,
    "directAccessGrantsEnabled": true,
    "standardFlowEnabled": false,
    "protocolMappers": [
      {
        "name": "frames-web-audience",
        "protocol": "openid-connect",
        "protocolMapper": "oidc-audience-mapper",
        "config": {
          "included.client.audience": "frames-web",
          "id.token.claim": "false",
          "access.token.claim": "true"
        }
      }
    ]
  }'
```

The critical part is the audience mapper: it sets the `aud` claim in the token to `frames-web`, which matches what the backend's token validator expects.

### 2. Acquire a token via password grant

```bash
TOKEN_RESPONSE=$(curl -s -X POST "http://localhost:8081/realms/frames/protocol/openid-connect/token" \
  -d "grant_type=password" \
  -d "client_id=frames-test" \
  -d "username=dev" \
  -d "password=dev")

ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')
```

### 3. Use the token against the backend

```bash
curl -s -X POST \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{}' \
  http://localhost:5173/frames.v1.FrameService/GetMe
```

Expected response (HTTP 200):
```json
{
  "subject": "f18f8763-c9e6-4746-bf60-be547008fac3",
  "email": "dev@localhost",
  "org": {
    "id": "01KWH68FR03KBP25Q6ZYMKYAAV",
    "slug": "dev-org",
    "displayName": "Dev Org",
    "createdAt": "2026-07-02T10:33:42Z"
  },
  "role": "admin",
  "canCreate": true
}
```

### 4. Verify with additional endpoints

List user's frames:
```bash
curl -s -X POST \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{}' \
  http://localhost:5173/frames.v1.FrameService/ListFrames
```

## Key Takeaways

1. **Audience claim matters**: The backend validates that the token's `aud` claim equals the expected audience. The frames-web client via the audience mapper adds this.

2. **Purpose-provisioned test client**: The production web client (`frames-web`) has `directAccessGrantsEnabled: false` to enforce the PKCE flow. A separate test client (`frames-test`) with password grant enabled is required for automated tests.

3. **Local dev testing only**: This procedure uses the local dev Keycloak realm and was proven only against that setup. A cluster's operator-provisioned OIDC identity provider may have different configuration, issuer URLs, audience requirements, or claim mappings. Apply these principles but validate against your actual OIDC configuration.

## Troubleshooting

**Token rejected with 401 "invalid or expired token"**: Check that the token's `aud` claim matches the backend's expectation (frames-web). Decode the JWT payload to verify:
```bash
echo "${ACCESS_TOKEN}" | cut -d'.' -f2 | base64 -d | jq .
```

**Password grant returns "unauthorized_client"**: The client has `directAccessGrantsEnabled: false`. Either use a different flow (PKCE, client credentials) or create a new client with password grant enabled.

## Cleanup

Stop the dev stack:
```bash
make dev-clean
```

This stops Keycloak and cleans up the SQLite database lock.
