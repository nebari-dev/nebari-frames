# 0001 - Frame templates are not Frames

- **Status:** accepted
- **Date:** 2026-09-07
- **Issue:** nebari-dev/nebari-frames#60

## Context

A new Frame starts blank, which is the hardest starting point: an author faces
ten empty slots (terminology, rules, skills, prompts, and the rest of
`frames.SlotTable` in `backend/internal/frames/slots.go`) and no cue about
which of them their organization actually cares about. In practice most Frames
fall into a small number of recognizable kinds - a style guide, a glossary, a
tool-usage brief - and an org that has settled on its own house standard for
one of those kinds wants every new Frame of that kind to start the same way.
Frame templates exist to give an author a running start and let an org express
that standard once instead of restating it in every new Frame.

The obvious modelling shortcut, and the one the originating issue suggested, is
to make a template a Frame marked as a template. That would reuse everything a
Frame already has for free: RBAC (`backend/internal/rbac/rbac.go`), versioning,
the publish path (`backend/internal/frames/service.go`), and both existing read
surfaces (the Connect API and the MCP resource list). This ADR records why that
shortcut was rejected, and what was built instead.

## Decision

A template is an authoring affordance local to this implementation. It is not a
Frame and not a Frame Spec concept: nothing in the wire format a `.frame.md`
document exchanges with another system knows templates exist. A template has
its own internal type (`Template` in `backend/internal/frames/templates.go`),
no version, no RBAC beyond org membership, and it is copied once at Frame
creation and then forgotten - editing or deleting a template afterward cannot
reach back into any Frame already made from it.

Built-ins are compiled into the binary: `backend/internal/frames/builtins.go`
embeds a directory of YAML files (`//go:embed builtins/*.yaml`) and parses them
at package `init`, so a malformed starter panics at startup rather than failing
at a user's first click on the picker. Org templates are rows in the
`frame_templates` table (`backend/internal/store/sqlite/migrations/006_frame_templates.sql`).
Both resolve into one list behind the same two read RPCs, `ListFrameTemplates`
and `GetFrameTemplate` (`backend/internal/frames/templates_service.go`), which
put built-ins first (Blank leading) and then an org's own rows. There is no
shadowing between the two: if an org template happens to share a built-in's
title, it sits beside it in the list rather than hiding or replacing it, and
each entry carries its own `builtin` flag so a client can tell which is which.

A template carries two things. First, prefill content: a `Prefill` (slots plus
suggested `extends` parents) that is deliberately a strict subset of a full
Frame document, because a template seeds content and must never dictate a
Frame's identity - `ParsePrefill` in `templates.go` refuses a prefill that sets
`name`, `description`, `version`, `visibility`, `scope`, `maintainer`, or
`excludes`. Second, a per-slot rule: a `FieldRule{Level, Note}` where `Level` is
one of `optional` / `recommended` / `required` and `Note` is the template
author's own guidance for that slot.

Requirements are checked once, at the create-from-template publish, and nowhere
else. `Template.Check` (`templates.go`) walks the required slots and reports the
ones a document leaves empty; `publish` (`service.go`) calls it only when the
request names a `template_id`, and merges its errors with the schema's own
violations so an author sees every problem in one response instead of
discovering the template's complaints after already fixing the schema's.
Nothing about the template - not its id, not which rules it had - is recorded
on the resulting Frame.

## Consequences

Positive: no cross-org change to `rbac.Can` was needed; there is no
reconciliation of seeded rows to run on upgrade when a built-in's rules change;
a template cannot be named in a Frame's `extends` list (there is no Frame to
name - see the rejected alternatives below), so the copy-once semantics cannot
be quietly violated by an author extending a "Frame" that is really a
template; and adding a slot to `SlotTable` touches neither the `frame_templates`
schema nor the proto, because prefill and field rules are stored as opaque
blobs (`prefill BLOB`, `field_rules BLOB` in migration 006) the same way
`frame_versions.content` is.

Accepted, and important, stated plainly: `required` is an authoring aid, not a
control. Because the check runs only on the publish that names `template_id`,
an author can empty a required section on their very next publish and nothing
objects - `PublishRequest.TemplateID`'s own doc comment in `service.go` says a
later publish "is never re-checked." A caller who omits `template_id` skips the
checks entirely, and both the CLI (`frames publish`, where `--template`
defaults to empty in `cli/cmd/publish.go`) and `create_frame` over MCP (whose
`writeFrameInput` in `backend/internal/mcp/write.go` carries no template field
at all) can publish without ever naming one. Real governance - a rule that
still holds on a Frame's tenth version - needs a persistent model that
survives past creation, which is a different design; the alternatives below
explain what that would have cost and why it was not built instead.

Also accepted: the CLI scaffold's guidance comments persist into the Frame's
stored content, because the Connect RPC stores the author's exact submitted
bytes rather than re-marshalling a parsed document (see `PublishFrame` in
`service.go`, and `scaffoldComment` in `cli/cmd/template.go`, which prints the
required and recommended slots and their notes as a leading YAML comment
block). They are stripped from composed and MCP output, which both read a
parsed `Doc` rather than the raw bytes, so the guidance never leaks into what
another Frame or an AI client sees. The alternative - printing the same
guidance to a terminal instead of writing it into the file - is worse: the
terminal scrolls away, and the author is left rereading a bare scaffold with no
memory of what each section was for.

For the future MCP templates surface (nebari-dev/nebari-frames#61): templates
must be exposed as tools only, never as MCP resources. The `nebari-frame://`
namespace (`backend/internal/mcp/resources.go`,
`docs/design/2026-05-21-mcp-endpoint-design.md`) addresses Frame content that a
client can read, cache, and - through `extends` - resolve into a composed
document. A template is not that: it is a one-shot authoring affordance, and
listing it in that namespace would suggest to a client that it is something a
Frame can inherit from or that resolves the way a Frame does. This is worth
recording now, before that surface exists, precisely because nothing else
written down would stop a future implementer from reaching for the shape
that's already there.

## Alternatives considered

### Templates as Frames in a shared system org

Rejected: it cannot work at all. `rbac.Can` (`backend/internal/rbac/rbac.go`)
opens with a cross-org deny - `if frameOrgID != caller.OrgID { return false,
nil }` - before it even reaches the admin check or the grant lookup, and
`ListFramesByOrg` is strictly single-org by construction. No caller could read
a Frame that lived in a system org distinct from their own, so every built-in
would be invisible to every real user unless `rbac.Can` itself changed.
Shipping a handful of starter documents is not a reason to modify the one
function every read and write path in the codebase depends on for its security
guarantee.

### Templates as Frames seeded into every org

Rejected: this would work, but at a cost that only grows. Each built-in would
need a row per org, so N built-ins times M orgs is N x M rows to keep in sync;
changing a built-in's wording would mean reconciling that row in every org on
every upgrade, a migration-shaped problem for what should be a static asset.
It would also pollute every list path that was never built to filter
templates out of Frames: `ListFramesByOrg`, the Connect `list_frames` RPC, and
the MCP resource listing would each need new filtering logic to hide entries
that are templates rather than real Frames. And because a Frame that happens
to be a template is still a Frame, it could be named in another Frame's
`extends` (`ExtendRef` in `backend/internal/frames/schema.go` is just a name
and an optional version) - which is exactly the coupling the copy-once
semantics of a template are meant to rule out.

### Custom templates as an operator config file, like branding

Rejected. This looked attractive by analogy to
`backend/internal/branding/branding.go`, which resolves an operator-supplied
JSON file mounted via `BRANDING_CONFIG_FILE` into runtime configuration - the
same shape of "per-deployment customization without a database row" that
custom templates might want. It looked especially apt because
`org_memberships` has `PRIMARY KEY (user_sub)`
(`backend/internal/store/sqlite/migrations/001_orgs_and_memberships.sql`): a
user belongs to at most one org, so a single mounted config file is, in
practice, already scoped to one org's population of users the way a
per-org table would be. But the actual requirement is that org members author
templates from inside the app and have them persist across restarts and
deploys - add one this afternoon, edit its wording tomorrow - and a file an
operator mounts at deploy time cannot become something an admin edits from the
web app. Branding answers a question only an operator has an opinion about;
templates answer a question a customer's own admin has an opinion about, and
needs a write path for.

### Persistent enforcement, by live pointer or by snapshot

Rejected in favor of checking at creation only, but this was the closest call.
A Frame could instead carry a live `template_id` and have every subsequent
publish re-run `Check` against whatever the template currently says. That
would give `required` real teeth, but a template edit would then retroactively
break every Frame that used to conform to it, and deleting a template could
make Frames that reference it unpublishable outright - a governance mechanism
that can reach back in time and break work an author already finished and
walked away from. A snapshot of the rules at creation time, stored on the
Frame, avoids that specific breakage and mirrors how `extends` already pins a
specific parent version rather than always following its latest - but it still
permanently couples a Frame to the template it started from, which contradicts
the same "authoring affordance, not an ongoing relationship" premise the rest
of this design rests on, and a Frame's schema and storage would carry that
coupling forever, for every Frame, whether or not its author ever cared about
the distinction after the first save. Creation-only checking keeps the promise
that a Frame made from a template is, from its second publish on, an entirely
ordinary Frame - at the accepted cost recorded above under Consequences.

### A bare Requirement instead of FieldRule{level, note}

Rejected in favor of pairing a level with a note from the start. A bare level
lets a template say "terminology is required" but not what this org actually
wants written there, which is the harder and more useful half of the
guidance an author needs. This was decided before shipping rather than patched
in afterward because the wire representation is a map from slot key to
`FieldRule` (`proto/frames/v1/frame.proto`): widening an existing map's value type from a
bare enum to a message is a breaking wire change for any client already
decoding the old shape, whereas shipping the message from the first version
costs nothing and leaves room to add fields later without breaking anyone.
