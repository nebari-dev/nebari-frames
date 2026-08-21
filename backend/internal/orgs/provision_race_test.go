package orgs_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/orgs"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	sqlitestore "github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// staleRepo makes only the FIRST membership read stale, reproducing the
// interleaving where a caller reads "no membership" just before a concurrent
// request activates the pending invite. Later reads see the truth, as they would
// in a real process.
type staleRepo struct {
	store.Repository
	staleReads int
}

func (r *staleRepo) GetMembership(ctx context.Context, sub string) (*framesv1.Membership, error) {
	if r.staleReads > 0 {
		r.staleReads--
		return nil, store.ErrNotFound
	}
	return r.Repository.GetMembership(ctx, sub)
}

// Provisioning the baseline must never lower a role another request has already
// established. The membership read and the write are not atomic, so a caller can
// arrive here holding a stale "no membership" answer; an UPDATE-first write would
// then rewrite the activated invite's role to the baseline, permanently. If the
// invite was the seed.adminEmail bootstrap, the org loses its only admin.
func TestResolveCallerDoesNotDowngradeAConcurrentlyActivatedInvite(t *testing.T) {
	ctx := context.Background()
	db, _ := sqlitestore.Open(t.TempDir() + "/t.db")
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	real := sqlitestore.New(db)
	_ = real.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme", CreatedAt: timestamppb.Now()})
	_ = real.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "publisher", Email: "boss@x.io", AddedAt: timestamppb.Now()})

	// R1 activates the invite: boss becomes publisher.
	c1, err := orgs.ResolveCaller(auth.WithClaims(ctx, &auth.Claims{Subject: "boss-sub", Email: "boss@x.io"}), real,
		orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"})
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	if c1.Role != rbac.RolePublisher {
		t.Fatalf("first request role = %q, want publisher from the invite", c1.Role)
	}

	// R2 read no membership before R1's activation landed, and its pending
	// lookup now finds nothing, so it falls through to provisioning.
	stale := &staleRepo{Repository: real, staleReads: 1}
	c2, err := orgs.ResolveCaller(auth.WithClaims(ctx, &auth.Claims{Subject: "boss-sub", Email: "boss@x.io"}), stale,
		orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"})
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if c2.Role != rbac.RolePublisher {
		t.Errorf("second request role = %q, want publisher: it must report the stored row, not its assumed baseline", c2.Role)
	}

	after, err := real.GetMembership(ctx, "boss-sub")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if after.Role != "publisher" {
		t.Errorf("stored role = %q, want publisher: the baseline write overwrote an established role", after.Role)
	}
}
