package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
)

// Migrations must survive the data they exist to repair. The suite otherwise
// only ever runs them against a fresh database, where nothing needs repairing -
// so this builds the schema as of migration 004, seeds the case-variant invites
// that the case-sensitive unique index used to permit, and migrates forward. A
// failure here is a crash-looping pod: migrations.Run's error reaches main,
// which exits.
//
// The fixture carries every table a later migration touches, not just the ones
// with data to repair: an ALTER against a table the fixture forgot fails as
// "no such table", which looks like a broken migration rather than a stale test.
func TestRunOnCaseVariantLegacyData(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", t.TempDir()+"/legacy.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY AUTOINCREMENT, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL, tstamp TIMESTAMP DEFAULT (datetime('now')))`,
		`INSERT INTO goose_db_version (version_id, is_applied) VALUES (0,1),(1,1),(2,1),(3,1),(4,1)`,
		`CREATE TABLE orgs (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL, created_at TEXT NOT NULL)`,
		// The frames table as of 002. Migration 006 alters it, so a fixture that
		// omitted it would fail on a missing table rather than on the data.
		`CREATE TABLE frames (id TEXT PRIMARY KEY, org_id TEXT NOT NULL REFERENCES orgs(id), name TEXT NOT NULL, description TEXT NOT NULL, owner_sub TEXT NOT NULL, latest_version TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE (org_id, name))`,
		`CREATE TABLE org_memberships (org_id TEXT NOT NULL REFERENCES orgs(id), user_sub TEXT NOT NULL DEFAULT '', email TEXT, role TEXT NOT NULL, added_at TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX idx_membership_sub ON org_memberships(user_sub) WHERE user_sub <> ''`,
		`CREATE UNIQUE INDEX idx_membership_email ON org_memberships(org_id, email) WHERE email IS NOT NULL`,
		`INSERT INTO orgs VALUES ('o1','acme','Acme','2026-01-01T00:00:00Z')`,
		// An activated membership alongside a pending invite for the same person.
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','','boss@x.io','viewer','2026-01-01T00:00:00Z')`,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','s2','Boss@X.io','admin','2026-01-02T00:00:00Z')`,
		// Two pending invites for the same person, the later one at a different role.
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','','carol@x.io','viewer','2026-01-03T00:00:00Z')`,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','','CAROL@X.IO','publisher','2026-01-04T00:00:00Z')`,
		// Stray whitespace, no collision.
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','','  dave@x.io  ','viewer','2026-01-05T00:00:00Z')`,
		// A pre-existing frame, so 006's ALTER runs against a populated table and
		// its NOT NULL DEFAULT is exercised on real rows rather than none.
		`INSERT INTO frames VALUES ('f1','o1','brand-voice','How we write','s2','1.0.0','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`,
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("setup %q: %v", q[:40], err)
		}
	}

	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrations must not fail on pre-existing case-variant rows: %v", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT user_sub, email, role FROM org_memberships ORDER BY rowid`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	type row struct{ sub, email, role string }
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.sub, &r.email, &r.role); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// One row survives per (org, folded address), and every address is folded.
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(got), got)
	}
	by := map[string]row{}
	for _, r := range got {
		by[r.email] = r
	}

	// An activated membership outranks a pending invite: the real member is not
	// replaced by an invitation nobody has claimed.
	if b := by["boss@x.io"]; b.sub != "s2" || b.role != "admin" {
		t.Errorf("boss = %+v, want the activated admin membership", b)
	}
	// Between two pending invites the later one wins: it is what the admin most
	// recently asked for, and keeping the earlier would reinstate a role they had
	// already replaced.
	if c := by["carol@x.io"]; c.sub != "" || c.role != "publisher" {
		t.Errorf("carol = %+v, want the later pending invite (publisher)", c)
	}
	if d := by["dave@x.io"]; d.role != "viewer" {
		t.Errorf("dave = %+v, want the trimmed address to survive", d)
	}

	// 006 adds is_template with a default, so a frame that predates it must come
	// forward as not-a-template rather than NULL.
	var isTemplate sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT is_template FROM frames WHERE id = 'f1'`).Scan(&isTemplate); err != nil {
		t.Fatalf("read is_template on a pre-existing frame: %v", err)
	}
	if !isTemplate.Valid || isTemplate.Int64 != 0 {
		t.Errorf("is_template = %v, want 0 for a frame published before the column existed", isTemplate)
	}

	// The new index must reject a case variant rather than storing both.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','s3','BOSS@X.IO','viewer','2026-01-03T00:00:00Z')`,
	); err == nil {
		t.Error("a case-variant address was accepted; the index is not case-insensitive")
	}
}
