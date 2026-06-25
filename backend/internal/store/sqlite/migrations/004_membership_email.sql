-- +goose Up
-- Rebuild org_memberships to add email and allow pending (empty user_sub) invites.
CREATE TABLE org_memberships_new (
    org_id   TEXT NOT NULL REFERENCES orgs(id),
    user_sub TEXT NOT NULL DEFAULT '',
    email    TEXT,
    role     TEXT NOT NULL,
    added_at TEXT NOT NULL
);
INSERT INTO org_memberships_new (org_id, user_sub, email, role, added_at)
    SELECT org_id, user_sub, NULL, role, added_at FROM org_memberships;
DROP TABLE org_memberships;
ALTER TABLE org_memberships_new RENAME TO org_memberships;
-- one active membership per real subject (pending rows have user_sub='')
CREATE UNIQUE INDEX idx_membership_sub ON org_memberships(user_sub) WHERE user_sub <> '';
-- one invite per email per org
CREATE UNIQUE INDEX idx_membership_email ON org_memberships(org_id, email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX idx_membership_email;
DROP INDEX idx_membership_sub;
CREATE TABLE org_memberships_old (
    org_id   TEXT NOT NULL REFERENCES orgs(id),
    user_sub TEXT NOT NULL,
    role     TEXT NOT NULL,
    added_at TEXT NOT NULL,
    PRIMARY KEY (user_sub)
);
INSERT INTO org_memberships_old (org_id, user_sub, role, added_at)
    SELECT org_id, user_sub, role, added_at FROM org_memberships WHERE user_sub <> '';
DROP TABLE org_memberships;
ALTER TABLE org_memberships_old RENAME TO org_memberships;
