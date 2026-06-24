-- +goose Up
CREATE TABLE frames (
    id             TEXT PRIMARY KEY,
    org_id         TEXT NOT NULL REFERENCES orgs(id),
    name           TEXT NOT NULL,
    description    TEXT NOT NULL,
    owner_sub      TEXT NOT NULL,
    latest_version TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    UNIQUE (org_id, name)
);

CREATE TABLE frame_versions (
    frame_id     TEXT NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    version      TEXT NOT NULL,
    content      BLOB NOT NULL,
    digest       TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    published_by TEXT NOT NULL,
    published_at TEXT NOT NULL,
    changelog    TEXT NOT NULL DEFAULT '',
    -- forward-compatible review-gate hook (roadmap §11 item 13); unused in MVP
    reviewed_by  TEXT,
    reviewed_at  TEXT,
    status       TEXT NOT NULL DEFAULT 'published',
    PRIMARY KEY (frame_id, version)
);

CREATE TABLE frame_extends (
    frame_id        TEXT NOT NULL,
    version         TEXT NOT NULL,
    parent_frame_id TEXT NOT NULL REFERENCES frames(id),
    parent_version  TEXT NOT NULL,
    order_index     INTEGER NOT NULL,
    FOREIGN KEY (frame_id, version) REFERENCES frame_versions(frame_id, version) ON DELETE CASCADE,
    PRIMARY KEY (frame_id, version, parent_frame_id, parent_version)
);

CREATE TABLE frame_excludes (
    frame_id          TEXT NOT NULL,
    version           TEXT NOT NULL,
    excluded_frame_id TEXT NOT NULL,
    FOREIGN KEY (frame_id, version) REFERENCES frame_versions(frame_id, version) ON DELETE CASCADE,
    PRIMARY KEY (frame_id, version, excluded_frame_id)
);

-- +goose Down
DROP TABLE frame_excludes;
DROP TABLE frame_extends;
DROP TABLE frame_versions;
DROP TABLE frames;
