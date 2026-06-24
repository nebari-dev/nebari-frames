package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
)

func TestRun_CreatesOrgTables(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO orgs (id, slug, display_name, created_at) VALUES (?, ?, ?, ?)`,
		"org1", "openteams", "OpenTeams", "2026-06-24T00:00:00Z",
	); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO org_memberships (org_id, user_sub, role, added_at) VALUES (?, ?, ?, ?)`,
		"org1", "user-abc", "admin", "2026-06-24T00:00:00Z",
	); err != nil {
		t.Fatalf("insert membership: %v", err)
	}
}

func TestRun_FrameSchemaAndForeignKeys(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// A frame_version referencing a non-existent frame must fail (FK on).
	_, err = db.Exec(
		`INSERT INTO frame_versions (frame_id, version, content, digest, size_bytes, published_by, published_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"missing", "1.0.0", []byte("x"), "d", 1, "u", "2026-06-24T00:00:00Z",
	)
	if err == nil {
		t.Fatal("expected FK violation inserting orphan frame_version, got nil")
	}

	// status defaults to 'published' and review columns are nullable.
	mustExec(t, db, `INSERT INTO orgs (id, slug, display_name, created_at) VALUES ('o','s','S','t')`)
	mustExec(t, db, `INSERT INTO frames (id, org_id, name, description, owner_sub, latest_version, created_at, updated_at)
	                 VALUES ('f','o','n','d','u','1.0.0','t','t')`)
	mustExec(t, db, `INSERT INTO frame_versions (frame_id, version, content, digest, size_bytes, published_by, published_at)
	                 VALUES ('f','1.0.0', x'00','d',1,'u','t')`)
	var status string
	var reviewedBy sql.NullString
	if err := db.QueryRow(`SELECT status, reviewed_by FROM frame_versions WHERE frame_id='f'`).
		Scan(&status, &reviewedBy); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != "published" || reviewedBy.Valid {
		t.Fatalf("want status=published reviewed_by=NULL, got status=%q reviewed_by.Valid=%v", status, reviewedBy.Valid)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
