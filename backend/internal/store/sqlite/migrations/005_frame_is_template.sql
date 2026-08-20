-- +goose Up
-- Frames flagged as templates are offered as starting points by the authoring
-- UI's "start from a template" picker. Denormalized from the latest version's
-- `template` doc field at publish time so listing never parses content blobs.
ALTER TABLE frames ADD COLUMN is_template INTEGER NOT NULL DEFAULT 0;
