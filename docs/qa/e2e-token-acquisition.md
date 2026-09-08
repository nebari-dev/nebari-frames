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

3. **Also proven on a cluster**: this procedure began as local-dev-only, but the same shape now works against an operator-provisioned realm; see the section below for the two differences that matter. Issuer URLs, audience requirements, and claim mappings still vary between deployments, so validate against your actual OIDC configuration.

## Against a Real Cluster

Verified 2026-09-08 on a kind cluster running nebari-infrastructure-core, and mirrored by the `e2e-sandbox` CI job. Two things differ from local dev: which identity the backend expects, and which certificate the gateway serves.

**The audience is the operator's SPA client, not a name you can predict.** The chart wires `OIDC_CLIENT_ID` to the `spa-client-id` key of the secret the operator writes (`<nebariapp>-oidc-client`), so that is what the token's `aud` must contain. Read it from the secret rather than re-deriving the `<ns>-<app>-spa` naming convention, so a convention change fails loudly at the read instead of silently at token validation:

```bash
kubectl -n nebari-frames get secret nebari-frames-nebari-frames-oidc-client \
  -o jsonpath='{.data.spa-client-id}' | base64 -d
```

The operator's SPA client has `directAccessGrantsEnabled: false`, so tests still need their own client whose audience mapper points at that id.

Keycloak's declarative user profile is enabled by default on the realm and marks `firstName`/`lastName` required. A user created without them is left incomplete and the password grant refuses with `Account is not fully set up`, which names no specific unmet condition. Set both, and set `requiredActions: []` explicitly so a realm default such as `UPDATE_PASSWORD` cannot block the grant the same way.

**TLS: pin the app's own certificate, not the gateway's.** The nebari-operator gives each NebariApp its own Gateway listener with its own certificate whose only SAN is that app's hostname, and Envoy serves it for that SNI. `nebari-gateway-tls` is only the catch-all listener's certificate (SANs: apex, keycloak, argocd). Pinning it for an app hostname fails with a presented-versus-pinned mismatch. Resolve the certificate from the listener under test:

```bash
GW_NS=$(kubectl get gateway -A \
  -o jsonpath='{.items[?(@.metadata.name=="nebari-gateway")].metadata.namespace}')
CERT_SECRET=$(kubectl -n "${GW_NS}" get gateway nebari-gateway \
  -o jsonpath='{.spec.listeners[?(@.hostname=="frames.example.com")].tls.certificateRefs[0].name}')
kubectl -n "${GW_NS}" get secret "${CERT_SECRET}" \
  -o jsonpath='{.data.tls\.crt}' | base64 -d > gateway-tls.crt
```

These certificates are self-signed leaves (`CA:FALSE`), issued by a self-signed cert-manager ClusterIssuer, with no CA anywhere to chain to. Go does accept such a certificate as its own trust anchor - `crypto/x509` short-circuits on `opts.Roots.contains(c)` and returns a one-element chain without consulting basic constraints - so "unknown authority" from a bundle built this way means the certificate is the *wrong* one, not that Go rejected a non-CA. `e2e/harness.go` pins instead of chaining for this case; see its comment.

Note that the pod's own trust bundle (`trustBundle.configMapName`, surfaced as `SSL_CERT_FILE`) is a separate concern, and `SSL_CERT_FILE` *replaces* Go's root pool rather than extending it. It must therefore carry public roots plus the private CA, or TLS to anything publicly signed breaks.

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
