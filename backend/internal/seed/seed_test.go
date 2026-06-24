package seed_test

import (
	"context"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/seed"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

func TestSeed_CreatesOrgAndAdminIdempotently(t *testing.T) {
	repo := store.NewMemory()
	ctx := context.Background()
	cfg := seed.Config{OrgSlug: "openteams", OrgDisplayName: "OpenTeams", AdminSub: "admin-1"}

	if err := seed.Run(ctx, repo, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seed.Run(ctx, repo, cfg); err != nil {
		t.Fatalf("second seed (idempotent): %v", err)
	}

	org, err := repo.GetOrgBySlug(ctx, "openteams")
	if err != nil {
		t.Fatalf("org missing: %v", err)
	}
	m, err := repo.GetMembership(ctx, "admin-1")
	if err != nil || m.Role != "admin" || m.OrgId != org.Id {
		t.Fatalf("admin membership wrong: %+v %v", m, err)
	}
}

func TestSeed_NoopWhenUnconfigured(t *testing.T) {
	repo := store.NewMemory()
	if err := seed.Run(context.Background(), repo, seed.Config{}); err != nil {
		t.Fatalf("noop seed: %v", err)
	}
	if _, err := repo.GetOrgBySlug(context.Background(), ""); err == nil {
		t.Fatal("expected no org created")
	}
}
