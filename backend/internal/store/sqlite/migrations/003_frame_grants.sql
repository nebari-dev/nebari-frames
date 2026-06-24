-- +goose Up
CREATE TABLE frame_grants (
    id           TEXT PRIMARY KEY,
    frame_id     TEXT NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    section_id   TEXT,
    subject_type TEXT NOT NULL,
    subject_id   TEXT NOT NULL,
    permission   TEXT NOT NULL,
    granted_by   TEXT NOT NULL,
    granted_at   TEXT NOT NULL,
    UNIQUE (frame_id, section_id, subject_type, subject_id, permission)
);

CREATE INDEX idx_frame_grants_lookup
    ON frame_grants (frame_id, subject_type, subject_id, permission);

-- +goose Down
DROP TABLE frame_grants;
