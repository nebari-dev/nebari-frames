# Final Review Fixes Report

Date: 2026-06-24
Branch: feat/backend-core (worktree commit)

---

## Fix 1 - Cross-org parent read enforcement (security)

**Files changed:** `backend/internal/frames/service.go`

`resolveEdges` and `resolveExcludes` previously resolved a parent frame ref via
`s.repo.GetFrameBySlugName` without checking whether the caller can read that
frame. An org-A publisher could enumerate org-B frame names by observing
success/failure differences during publish - a classic oracle vulnerability.

**Changes:**
- Added `caller rbac.Caller` parameter to both `resolveEdges` and
  `resolveExcludes`.
- After resolving each parent frame, calls `rbac.Can(ctx, s.lookup, caller,
  pf.OrgId, pf.Id, rbac.PermRead)`.
- If the RBAC lookup errors, returns `connect.CodeInternal`.
- If `canRead` is false, returns the same `connect.CodeInvalidArgument` error
  message as when the frame is absent (`extends[i]: parent <ref> not found` /
  `excludes: <ref> not found`) so denied-read and absent are indistinguishable.
- Both call sites in `PublishFrame` updated to pass `caller`.
- Initialized `out` to non-nil empty slices in both helpers (also covers Fix 2
  for these functions).

**Test added:** `TestService_CrossOrgParentReadEnforcement` (table-driven,
2 subtests):
- `cross-org existing frame is denied (same code as absent)` - org-A publisher
  extends `acme/secret` which exists in org B; verifies `CodeInvalidArgument`.
- `cross-org nonexistent frame` - org-A publisher extends `acme/does-not-exist`;
  verifies same `CodeInvalidArgument`.
Both subtests confirm the error codes are identical, preventing oracle leakage.

---

## Fix 2 - Empty slices, not nil (wire contract)

**Files changed:** `backend/internal/store/memory.go`,
`backend/internal/store/sqlite/sqlite.go`, `backend/internal/frames/service.go`

Functions that previously returned `var out []T` (nil on zero rows) were changed
to `out := []T{}` so JSON serializes `[]` instead of `null`.

Specific changes:
- `memory.go` `ListFramesByOrg`: `var out` -> `out := []*framesv1.Frame{}`
- `memory.go` `FrameGrants`: map lookup now returns `[]Grant{}` on miss instead
  of nil (the nil map value).
- `sqlite.go` `ListFramesByOrg`: `var out` -> `out := []*framesv1.Frame{}`
- `sqlite.go` `FrameGrants`: `var out` -> `out := []store.Grant{}`
- `service.go` `ListFrames`: `resp.Frames` initialized to
  `[]*framesv1.FrameSummary{}` instead of implicit nil.
- `service.go` `resolveEdges` and `resolveExcludes`: also use `[]T{}` (covered
  as part of Fix 1 changes).

---

## Fix 3 - Atomic rollback negative test

**Files changed:** `backend/internal/store/sqlite/sqlite_test.go`

Added `TestSQLite_PublishAtomicRollback` (table-driven, 1 subtest):

- Calls `repo.CreateFrameVersion` with `IsNewFrame: true` and
  `Extends: []store.ParentEdge{{ParentFrameID: "does-not-exist", ...}}`.
- The `frame_extends` INSERT fails with a FK violation because no `frames` row
  has id `"does-not-exist"` (foreign_keys=ON is active via `sqlite.Open`'s
  pragmas).

Assertions:
- (a) `CreateFrameVersion` returns a non-nil error.
- (b) `GetFrameBySlugName` for the new frame returns `store.ErrNotFound`
  (frame row was rolled back).
- (c) `FrameGrants` for the frame id returns 0 grants (grants rows were rolled
  back too).

---

## Fix 4 - Rune count for description limit

**Files changed:** `backend/internal/frames/validate.go`

The description length check used `len(doc.Description) > 280` which counts
bytes, not Unicode characters. A description with multi-byte characters
(e.g. CJK text) would incorrectly fail validation.

**Changes:**
- Added `"unicode/utf8"` to imports.
- Changed `len(doc.Description) > 280` to
  `utf8.RuneCountInString(doc.Description) > 280`.
- Error message already read "must be at most 280 characters" - unchanged.
- The empty-description check (`doc.Description == ""`) is unaffected.

---

## Test Results

```
go vet ./...   - clean (no output)
go test ./... -race

ok  github.com/nebari-dev/nebari-frames/backend/internal/frames           1.023s
ok  github.com/nebari-dev/nebari-frames/backend/internal/orgs             1.014s
ok  github.com/nebari-dev/nebari-frames/backend/internal/rbac             1.015s
ok  github.com/nebari-dev/nebari-frames/backend/internal/seed             1.014s
ok  github.com/nebari-dev/nebari-frames/backend/internal/server           1.016s
ok  github.com/nebari-dev/nebari-frames/backend/internal/store            1.016s
ok  github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite     1.419s
ok  github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations  1.089s

go build ./backend/cmd/server  - clean (no output)
```

All 8 test packages pass. No race conditions detected. Build is clean.

---

## Issues Encountered

The agent worktree was branched from `main` (skeleton only, commit 92f167c),
not from `feat/backend-core`. The worktree was reset to `feat/backend-core`
(`git reset --hard feat/backend-core`) before applying fixes to ensure all
changes land on the correct branch history.
