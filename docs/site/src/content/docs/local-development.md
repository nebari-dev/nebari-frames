---
title: Local Development
---

Two loops cover local development, both documented in the project [Makefile](https://github.com/nebari-dev/nebari-frames/blob/main/Makefile) and [README](https://github.com/nebari-dev/nebari-frames#run-locally).

**Prerequisites:** Go 1.25.7+, Node 16+ (Docker also required for `make dev-auth`).

## Fast UI loop - `make dev`

```bash
make dev
```

Runs the backend in dev mode (no OIDC) on `:8080` and the Vite dev server on `:5173`, seeded with representative sample data: an org, members across roles, and Frames with full slot content, multi-level inheritance, and versions. Open **http://localhost:5173** - UI edits hot-reload. A single **Ctrl-C** stops both processes.

There is no login step in this loop. Dev mode disables OIDC and injects a fixed identity, so you land straight in the app as `dev-user`, an org admin, and never hit the "No organization access" screen (see [Troubleshooting](/troubleshooting/)).

## Real login loop - `make dev-auth`

```bash
make dev-auth
```

Builds the SPA into the binary, starts Keycloak in Docker (`:8081`) with an auto-imported realm, and runs the backend in OIDC mode serving the built SPA on `:5173`. Open **http://localhost:5173** and log in as `dev@localhost` / `dev` - that user is seeded as a pending org admin that activates on first login. Keycloak admin console: http://localhost:8081 (`admin` / `admin`).

Use this loop when you need to exercise the real OIDC path (login redirect, token refresh, the CLI's device-code flow) rather than the fixed dev identity `make dev` gives you.

## Tearing down - `make dev-clean`

```bash
make dev-clean
```

Stops any leftover dev backend still holding the SQLite lock (for example, an orphaned process from a `make dev` that was suspended with Ctrl-Z or killed with `kill -9` instead of stopped with a single Ctrl-C), removes the dev database and its `-wal`/`-shm` sidecar files, and stops/removes the Keycloak container and its volume. It kills by the TCP port owner rather than by process name, since dev binaries built with `go build` share a common truncated command name that a name-based kill would over-match. Run this whenever a dev loop was not shut down cleanly before starting a fresh one.

## Other useful targets

- `make build-web` - build the SPA (`web/`) and the server binary.
- `make test` - `go test ./... -race -coverprofile=coverage.out`.
- `make lint` - `golangci-lint run ./...`.
- `make proto` - lint and regenerate the protobuf/Connect code (`buf lint` + `buf generate`).
