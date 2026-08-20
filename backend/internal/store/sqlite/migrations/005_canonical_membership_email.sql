-- +goose Up
-- Canonicalize membership emails and make their uniqueness case-insensitive.
--
-- Identity providers do not guarantee the case of the email claim and invites
-- are typed by hand, so the two routinely disagree. With a case-sensitive
-- unique index, two invites for the same person could coexist and which one a
-- login activated was arbitrary.

-- Fold existing addresses to the canonical form first, so the new index can be
-- built. Rows written before this migration may carry stray case or whitespace.
UPDATE org_memberships
   SET email = lower(trim(email))
 WHERE email IS NOT NULL AND email <> lower(trim(email));

-- Folding can collide: two rows in one org whose addresses differed only in
-- case are now identical. Keep the oldest (lowest rowid) and drop the rest,
-- preferring an activated membership over a pending invite so a real member is
-- never discarded in favour of an unclaimed invitation.
DELETE FROM org_memberships
 WHERE email IS NOT NULL
   AND rowid NOT IN (
       SELECT rowid FROM (
           SELECT rowid,
                  ROW_NUMBER() OVER (
                      PARTITION BY org_id, email
                      ORDER BY CASE WHEN user_sub <> '' THEN 0 ELSE 1 END, rowid
                  ) AS rn
             FROM org_memberships
            WHERE email IS NOT NULL
       )
      WHERE rn = 1
   );

DROP INDEX idx_membership_email;
CREATE UNIQUE INDEX idx_membership_email
    ON org_memberships(org_id, email COLLATE NOCASE) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX idx_membership_email;
CREATE UNIQUE INDEX idx_membership_email
    ON org_memberships(org_id, email) WHERE email IS NOT NULL;
