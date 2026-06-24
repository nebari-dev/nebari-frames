# CLI Final Fixes Report

## Summary

Three fixes applied as part of the final review pass on the `feat/frames-cli` branch.

---

## Fix 1: Re-login prompt on CodeUnauthenticated (spec §4)

**Files changed:**
- `cli/cmd/show.go` - added `authAware` helper; `show` error path updated
- `cli/cmd/extends.go` - `extends` error path updated
- `cli/cmd/resolve.go` - `resolve` error path updated
- `cli/cmd/list.go` - `list` error path updated
- `cli/cmd/publish.go` - `publish` error path updated (after CodeInvalidArgument check)
- `cli/cmd/auth.go` - `auth status` error path updated

**What changed:** Added `authAware(err error) error` to `show.go` (same package as all cmd files). It checks `connect.CodeOf(err) == connect.CodeUnauthenticated` and returns a friendly `"not authenticated; run 'frames auth login'"` error. All RPC error returns now pass through this wrapper; `show`, `extends`, and `resolve` chain it as `authAware(notFoundAware(err))`.

---

## Fix 2: /auth/config GET guard (T1)

**Files changed:**
- `backend/internal/server/server.go` - method guard added
- `backend/internal/server/server_test.go` - assertions extended; new 405 table test added

**What changed:** `handleAuthConfig` now checks `r.Method != http.MethodGet` and returns `405 Method Not Allowed` before any config logic. The existing GET path is unchanged. `TestServer_AuthConfig` now decodes and asserts `client_id` and `device_client_id` in addition to `enabled` and `issuer_url`. A new `TestServer_AuthConfig_MethodNotAllowed` table-driven test covers POST, PUT, and DELETE.

---

## Fix 3: Stale godoc in device_flow.go (T3)

**File changed:** `cli/internal/auth/device_flow.go`

**What changed:** The `StartDeviceFlow` godoc comment said "skillsctl server"; updated to "frames server". Test variable names were not touched.

---

## Tests Added

`cli/cmd/show_test.go` - `TestAuthAware` (table-driven, keyed literals). Covers `show`, `extends`, `resolve`, and `list` returning `CodeUnauthenticated` from the stub, asserting the error message contains `"run 'frames auth login'"`.

---

## Verification

- `go vet ./...` - clean
- `go test ./... -race` - all 14 packages pass (11 with tests, 5 no test files)
- `go build -o /tmp/frames ./cli` - success
