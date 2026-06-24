-- +goose Up
CREATE TABLE orgs (
    id           TEXT PRIMARY KEY,
    slug         TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

CREATE TABLE org_memberships (
    org_id   TEXT NOT NULL REFERENCES orgs(id),
    user_sub TEXT NOT NULL,
    role     TEXT NOT NULL,
    added_at TEXT NOT NULL,
    PRIMARY KEY (user_sub)
);

-- +goose Down
DROP TABLE org_memberships;
DROP TABLE orgs;
