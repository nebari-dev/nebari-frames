-- +goose Up
-- Frame templates are an authoring affordance, not Frames: they are copied once
-- at creation and nothing is recorded on the Frame that came from one, so there
-- is no foreign key back to frames and no version history here. See
-- docs/adr/0001-frame-templates-are-not-frames.md.
--
-- Built-in templates are NOT rows. They are compiled into the binary, because
-- seeding them per org would mean reconciling N rows on every upgrade.
CREATE TABLE frame_templates (
    id           TEXT PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES orgs(id),
    title        TEXT NOT NULL,
    description  TEXT NOT NULL,
    -- Opaque blobs so that adding a slot to frames.SlotTable never touches this
    -- schema, the same reasoning as frame_versions.content.
    prefill      BLOB NOT NULL,   -- canonical YAML: slots + extends
    field_rules  BLOB NOT NULL,   -- JSON: slot key -> {level, note}
    created_by   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    UNIQUE (org_id, title)
);

-- +goose Down
DROP TABLE frame_templates;
