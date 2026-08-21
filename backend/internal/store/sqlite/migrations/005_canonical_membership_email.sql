-- +goose Up
-- Canonicalize membership emails and make their uniqueness case-insensitive.
--
-- Identity providers do not guarantee the case of the email claim and invites
-- are typed by hand, so the two routinely disagree. With a case-sensitive
-- unique index, two invites for the same person could coexist and which one a
-- login activated was arbitrary.
--
-- Statement order matters. The old index is case-sensitive, so folding
-- addresses while it is still in place fails the moment two rows in one org
-- differ only in case - which is precisely the data this migration exists to
-- repair. Drop the index first, collapse the duplicates on the *folded* key,
-- and only then fold the surviving rows.

DROP INDEX idx_membership_email;

-- Collapse rows that are about to become identical.
--
-- An activated membership always outranks a pending invite, so a real member is
-- never discarded in favour of an unclaimed invitation. Among activated rows the
-- oldest wins: it is the longer-standing membership. Among pending invites the
-- newest wins, because a second invite for the same person is the admin saying
-- what they want now - keeping the older one would silently reinstate a role
-- they had already replaced.
DELETE FROM org_memberships
 WHERE email IS NOT NULL
   AND rowid NOT IN (
       SELECT rowid FROM (
           SELECT rowid,
                  ROW_NUMBER() OVER (
                      PARTITION BY org_id, lower(trim(email))
                      ORDER BY
                          CASE WHEN user_sub <> '' THEN 0 ELSE 1 END,
                          CASE WHEN user_sub <> '' THEN rowid ELSE -rowid END
                  ) AS rn
             FROM org_memberships
            WHERE email IS NOT NULL
       )
      WHERE rn = 1
   );

UPDATE org_memberships
   SET email = lower(trim(email))
 WHERE email IS NOT NULL AND email <> lower(trim(email));

CREATE UNIQUE INDEX idx_membership_email
    ON org_memberships(org_id, email COLLATE NOCASE) WHERE email IS NOT NULL;

-- +goose Down
-- Folded addresses cannot be restored; only the index shape is reversible.
DROP INDEX idx_membership_email;
CREATE UNIQUE INDEX idx_membership_email
    ON org_memberships(org_id, email) WHERE email IS NOT NULL;
