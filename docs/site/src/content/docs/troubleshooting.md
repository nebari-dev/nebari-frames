---
title: Troubleshooting
---

## Deployment (Kubernetes)

### Pod stuck in `CreateContainerConfigError` - OIDC secret not provisioned

**Symptom:** with `nebariapp.enabled: true` and `nebariapp.auth.enabled: true`, the pod never starts; `kubectl describe pod` shows `CreateContainerConfigError` referencing a `secretKeyRef` to the operator-provisioned OIDC client secret.

**Cause:** the nebari-operator only reconciles `NebariApp` resources - and provisions the OIDC client secret they depend on - in namespaces labeled `nebari.dev/managed=true`. Without the label, the operator never picks up the app, so the secret the Deployment expects is never created.

**Fix:**

```bash
kubectl label namespace <namespace> nebari.dev/managed=true --overwrite
```

The operator picks up the labeled namespace on its next reconcile and creates the secret; the pod then starts normally.

### Readiness stuck at `503` - OIDC discovery cannot reach or trust the issuer

**Symptom:** the pod is `Running` but never `Ready`; `/readyz` keeps returning `503`.

**Cause:** the backend performs OIDC discovery against `OIDC_ISSUER_URL` in the background and only reports ready once it succeeds. If the issuer is unreachable from inside the pod (DNS, network policy) or its certificate is not trusted (for example a self-signed CA the pod does not trust), discovery never completes and the pod stays `NotReady` indefinitely. This is expected and by design: routing and TLS can be fully provisioned and the app should still correctly report not-ready if auth cannot actually be validated.

**Where to look:**

```bash
# The operator's view of what it has (and has not) provisioned for this app.
kubectl -n <namespace> get nebariapps.nebari.dev -o yaml
# Check conditions (routing/TLS/auth) and the issuer/client fields it recorded.

# The operator's own logs, for reconcile errors against Keycloak.
kubectl -n nebari-operator-system logs -l control-plane=controller-manager --tail=200

# The frames pod's own startup log line reports the issuer it is validating against.
kubectl -n <namespace> logs deploy/<release>-nebari-frames --tail=100
```

## Local development

### "No organization access" after login

This is intentional fail-closed behavior: a signed-in user who is not a member of any org is denied. `make dev` seeds you (`dev-user`) as an admin, and `make dev-auth` seeds `dev@localhost` as a pending admin that activates on first login, so neither loop should show this page. Against a real deployment, ask an org admin to add your email (via `seed.adminEmail` or the admin UI).

### `disk I/O error` / `database is locked` on startup

A previous dev backend was left running - for example `make dev` was suspended with Ctrl-Z or killed with `kill -9` instead of stopped with a single Ctrl-C - and still holds the SQLite lock. Run `make dev-clean` to stop the orphan (it frees ports `:5173`/`:8080`), clear the dev database and its `-wal`/`-shm` files, and reset Keycloak, then start again. Always stop a dev loop with a single **Ctrl-C** so both processes shut down cleanly.
