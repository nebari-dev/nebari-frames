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
		`CREATE TABLE org_memberships (org_id TEXT NOT NULL REFERENCES orgs(id), user_sub TEXT NOT NULL DEFAULT '', email TEXT, role TEXT NOT NULL, added_at TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX idx_membership_sub ON org_memberships(user_sub) WHERE user_sub <> ''`,
		`CREATE UNIQUE INDEX idx_membership_email ON org_memberships(org_id, email) WHERE email IS NOT NULL`,
		`INSERT INTO orgs VALUES ('o1','acme','Acme','2026-01-01T00:00:00Z')`,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','','boss@x.io','viewer','2026-01-01T00:00:00Z')`,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','s2','Boss@X.io','admin','2026-01-02T00:00:00Z')`,
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

	// One row survives per (org, folded address). The activated membership is
	// kept over the pending invite, so the real member is not replaced by an
	// invitation that nobody has claimed.
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(got), got)
	}
	if got[0].sub != "s2" || got[0].email != "boss@x.io" || got[0].role != "admin" {
		t.Errorf("surviving row = %+v, want the activated admin membership with a folded address", got[0])
	}

	// The new index must reject a case variant rather than storing both.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO org_memberships (org_id,user_sub,email,role,added_at) VALUES ('o1','s3','BOSS@X.IO','viewer','2026-01-03T00:00:00Z')`,
	); err == nil {
		t.Error("a case-variant address was accepted; the index is not case-insensitive")
	}
}
