package migrations_test

import (
	"context"
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
