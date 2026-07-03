---
title: Installation
---

Nebari Frames ships as a single Helm chart that deploys one pod (Go backend + embedded SPA) backed by a SQLite database on a PVC.

## Prerequisites

- A Nebari cluster with the foundational stack running: the nebari-operator, Envoy Gateway, cert-manager, and Keycloak.
- The target namespace labeled `nebari.dev/managed=true`. The nebari-operator only reconciles `NebariApp` resources in namespaces carrying this label - without it, routing, TLS, and OIDC client provisioning silently do not happen (see [Troubleshooting](/troubleshooting/)).

## Install

Install from the published chart:

:::note
Every tagged release publishes the chart to `oci://quay.io/nebari/charts/nebari-frames` (see [CI/CD and Releasing](/ci-cd-releasing/)). Installing from a checkout also works: `helm install frames chart/ -n nebari-frames -f chart/examples/nebari-values.yaml`, replacing values as needed.
:::

```bash
kubectl create namespace nebari-frames
kubectl label namespace nebari-frames nebari.dev/managed=true --overwrite

helm install frames oci://quay.io/nebari/charts/nebari-frames \
  --version <chart-version> \
  -n nebari-frames \
  --set nebariapp.enabled=true \
  --set nebariapp.hostname=frames.example.com \
  --set seed.orgSlug=my-org \
  --set seed.orgDisplayName="My Org" \
  --set seed.adminEmail=admin@example.com
```

Expect the pod to report `NotReady` for a short window after install: the readiness probe (`/readyz`) returns `503` until the backend's in-pod OIDC discovery against the issuer succeeds, which depends on the operator finishing the OIDC client it provisions. The pod flips to `Ready` once discovery completes.

## NebariApp integration

Setting `nebariapp.enabled: true` creates a `NebariApp` resource. The nebari-operator reads it and provisions, on your behalf:

- **Routing** - an HTTPRoute attaching the app to the named gateway (`nebariapp.gateway`, default `public`) at `nebariapp.hostname`.
- **TLS** - a certificate for the hostname (`nebariapp.routing.tls`).
- **Landing page tile** - an entry on the Nebari landing page (`nebariapp.landingPage`), gated by `nebariapp.auth`.
- **OIDC**, when `nebariapp.auth.enabled: true` (see below) - a Keycloak client and a secret the Deployment reads its OIDC settings from.

## Auth modes

The chart selects exactly one auth mode, fail-closed, in this order (mirrored in the backend at startup):

1. **NebariApp-managed OIDC** - `nebariapp.enabled: true` and `nebariapp.auth.enabled: true`. The operator provisions a Keycloak client, and the app reads `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, and `OIDC_DEVICE_CLIENT_ID` from the operator-written secret.
2. **Dev mode** - `auth.devMode: true` (only takes effect when NebariApp auth is off). Disables authentication entirely and injects a fixed `dev-user` identity. Local/demo use only - never enable this on a real deployment.
3. **Self-managed OIDC** - neither of the above. You supply `auth.oidc.issuerUrl`, `auth.oidc.clientId`, and optionally `auth.oidc.deviceClientId` for your own OIDC provider.

If none of the three is satisfiable - auth required but incomplete - the backend refuses to start rather than come up unauthenticated. See [Configuration](/configuration/#auth-modes) for the full values reference.

## First admin

Set `seed.adminEmail` to your first administrator's email. On startup Frames creates a pending admin invite for that email; the first person who logs in through OIDC with a matching email is reconciled to that invite and becomes the org admin. You do not need to look up an OIDC subject ahead of time. In dev mode, the identity is fixed to `dev-user` / `dev@localhost` and `seed.adminEmail` has no effect.

## Deploying via ArgoCD

Nebari clusters typically install applications through GitOps rather than a direct `helm install`. The repository ships a ready-to-adapt example at [`chart/examples/argocd-app.yaml`](https://github.com/nebari-dev/nebari-frames/blob/main/chart/examples/argocd-app.yaml): an ArgoCD `Application` pointed at the `chart/` directory, with the same `nebariapp`, `seed`, and `mcp` values shown above. Once the chart is published (see [CI/CD and Releasing](/ci-cd-releasing/)), the same `Application` can instead point at `oci://quay.io/nebari/charts` with `chart: nebari-frames` and a `targetRevision` set to the chart version, matching the `helm install` command above.

Either way, prefer ArgoCD's `syncPolicy.managedNamespaceMetadata.labels` to apply `nebari.dev/managed: "true"` as part of namespace creation, so you do not need a separate `kubectl label` step.

## Storage and scaling

SQLite is single-writer, so `replicaCount` must stay `1`; the Deployment uses the `Recreate` strategy so the old pod releases the volume before the new one mounts it. Postgres support for `replicaCount > 1` is planned but not in this release - do not raise it.
