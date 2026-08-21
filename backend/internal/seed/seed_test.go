package seed_test

import (
	"context"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/seed"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

func TestRun_AdminEmail(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		seedEmail   string
		preSeed     func(t *testing.T, repo store.Repository, orgID string)
		wantPending bool // expect a pending (UserSub=="") admin invite for the email
	}{
		{
			name:        "fresh email adds pending admin invite",
			seedEmail:   "admin@example.com",
			wantPending: true,
		},
		{
			name:      "idempotent when invite already pending",
			seedEmail: "admin@example.com",
			preSeed: func(t *testing.T, repo store.Repository, orgID string) {
				if err := repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: orgID, Email: "admin@example.com", Role: "admin"}); err != nil {
					t.Fatal(err)
				}
			},
			wantPending: true,
		},
		{
			name:      "no-op when email already an active member",
			seedEmail: "admin@example.com",
			preSeed: func(t *testing.T, repo store.Repository, orgID string) {
				if err := repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: orgID, UserSub: "sub-123", Email: "admin@example.com", Role: "admin"}); err != nil {
					t.Fatal(err)
				}
			},
			wantPending: false,
		},
		{
			name:        "seed twice with same AdminEmail: second run idempotent",
			seedEmail:   "twice@example.com",
			wantPending: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := store.NewMemory()
			// Seed org first so preSeed has an org id.
			if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "acme", OrgDisplayName: "Acme"}); err != nil {
				t.Fatalf("seed org: %v", err)
			}
			org, err := repo.GetOrgBySlug(ctx, "acme")
			if err != nil {
				t.Fatal(err)
			}
			if tc.preSeed != nil {
				tc.preSeed(t, repo, org.Id)
			}
			if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "acme", AdminEmail: tc.seedEmail}); err != nil {
				t.Fatalf("seed admin email: %v", err)
			}
			pending, err := repo.GetPendingMembershipByEmail(ctx, tc.seedEmail)
			gotPending := err == nil && pending != nil && pending.UserSub == ""
			if gotPending != tc.wantPending {
				t.Fatalf("pending invite = %v, want %v (err=%v)", gotPending, tc.wantPending, err)
			}
			// For "twice" test case, run seed again and verify it's idempotent.
			if tc.name == "seed twice with same AdminEmail: second run idempotent" {
				if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "acme", AdminEmail: tc.seedEmail}); err != nil {
					t.Fatalf("second seed run: %v", err)
				}
				// Verify still exactly one pending invite for this email.
				pending2, err2 := repo.GetPendingMembershipByEmail(ctx, tc.seedEmail)
				if err2 != nil || pending2 == nil || pending2.UserSub != "" {
					t.Fatalf("second run corrupted pending invite: %+v (err=%v)", pending2, err2)
				}
			}
		})
	}
}

func TestSeed_Run(t *testing.T) {
	tests := []struct {
		name string
		cfg  seed.Config
		// preSeed, if non-nil, is applied to the repo before the table's own
		// Run calls so a row can exercise the "row already exists" path.
		preSeed func(t *testing.T, ctx context.Context, repo *store.Memory)
		// wantOrg is the slug expected to exist after Run; "" means no org.
		wantOrg string
		// wantDisplayName is asserted on the org when wantOrg is non-empty.
		wantDisplayName string
		// wantAdminSub is the membership user_sub expected with role "admin";
		// "" means no membership assertion.
		wantAdminSub string
	}{
		{
			name:            "creates org and admin",
			cfg:             seed.Config{OrgSlug: "openteams", OrgDisplayName: "OpenTeams", AdminSub: "admin-1"},
			wantOrg:         "openteams",
			wantDisplayName: "OpenTeams",
			wantAdminSub:    "admin-1",
		},
		{
			name:            "defaults DisplayName to OrgSlug",
			cfg:             seed.Config{OrgSlug: "acme", AdminSub: "admin-2"},
			wantOrg:         "acme",
			wantDisplayName: "acme",
			wantAdminSub:    "admin-2",
		},
		{
			name: "org exists, admin added",
			cfg:  seed.Config{OrgSlug: "existing", OrgDisplayName: "Existing Org", AdminSub: "admin-3"},
			preSeed: func(t *testing.T, ctx context.Context, repo *store.Memory) {
				t.Helper()
				if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "org-existing", Slug: "existing", DisplayName: "Existing Org"}); err != nil {
					t.Fatalf("pre-seed org: %v", err)
				}
			},
			wantOrg:         "existing",
			wantDisplayName: "Existing Org",
			wantAdminSub:    "admin-3",
		},
		{
			name:    "noop when OrgSlug empty",
			cfg:     seed.Config{},
			wantOrg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			ctx := context.Background()

			if tt.preSeed != nil {
				tt.preSeed(t, ctx, repo)
			}

			// Run twice to assert idempotency: a second Run must not error or
			// duplicate any row.
			if err := seed.Run(ctx, repo, tt.cfg); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := seed.Run(ctx, repo, tt.cfg); err != nil {
				t.Fatalf("second seed (idempotent): %v", err)
			}

			if tt.wantOrg == "" {
				if _, err := repo.GetOrgBySlug(ctx, ""); err == nil {
					t.Fatal("expected no org created")
				}
				return
			}

			org, err := repo.GetOrgBySlug(ctx, tt.wantOrg)
			if err != nil {
				t.Fatalf("org missing: %v", err)
			}
			if org.DisplayName != tt.wantDisplayName {
				t.Fatalf("display name = %q, want %q", org.DisplayName, tt.wantDisplayName)
			}

			if tt.wantAdminSub != "" {
				m, err := repo.GetMembership(ctx, tt.wantAdminSub)
				if err != nil || m.Role != "admin" || m.OrgId != org.Id {
					t.Fatalf("admin membership wrong: %+v %v", m, err)
				}
			}
		})
	}
}

// With auth.defaultRole enabled, any user who has signed in already holds a
// membership, which used to make the configured admin bootstrap a no-op: the
// sub path saw an existing membership and returned, and the email path saw an
// active member with that address and returned. An operator who enabled the
// default role before configuring an admin could therefore end up with an org
// that has no admin at all and no in-product way to create one.
//
// seed.Run now promotes the configured admin when the org has none, and leaves
// roles alone when an admin already exists.
func TestRun_PromotesConfiguredAdminWhenOrgHasNone(t *testing.T) {
	tests := []struct {
		name string
		// existing memberships before seeding
		seed    func(context.Context, *store.Memory)
		cfg     seed.Config
		wantSub string // membership to inspect afterwards
		// expected role of wantSub after Run
		wantRole string
	}{
		{
			name: "a baseline-provisioned user configured as admin by sub is promoted",
			seed: func(ctx context.Context, repo *store.Memory) {
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "boss-sub", Role: "viewer", Email: "boss@x.io"})
			},
			cfg:      seed.Config{OrgSlug: "acme", AdminSub: "boss-sub"},
			wantSub:  "boss-sub",
			wantRole: "admin",
		},
		{
			name: "a baseline-provisioned user configured as admin by email is promoted",
			seed: func(ctx context.Context, repo *store.Memory) {
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "boss-sub", Role: "viewer", Email: "boss@x.io"})
			},
			cfg:      seed.Config{OrgSlug: "acme", AdminEmail: "boss@x.io"},
			wantSub:  "boss-sub",
			wantRole: "admin",
		},
		{
			name: "a deliberate demotion is left alone while another admin exists",
			seed: func(ctx context.Context, repo *store.Memory) {
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "boss-sub", Role: "viewer", Email: "boss@x.io"})
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "other-admin", Role: "admin", Email: "other@x.io"})
			},
			cfg:      seed.Config{OrgSlug: "acme", AdminSub: "boss-sub"},
			wantSub:  "boss-sub",
			wantRole: "viewer",
		},
		{
			name: "an existing admin is untouched",
			seed: func(ctx context.Context, repo *store.Memory) {
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "boss-sub", Role: "admin", Email: "boss@x.io"})
			},
			cfg:      seed.Config{OrgSlug: "acme", AdminSub: "boss-sub"},
			wantSub:  "boss-sub",
			wantRole: "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := store.NewMemory()
			if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme", DisplayName: "Acme"}); err != nil {
				t.Fatalf("create org: %v", err)
			}
			tt.seed(ctx, repo)

			if err := seed.Run(ctx, repo, tt.cfg); err != nil {
				t.Fatalf("run: %v", err)
			}

			m, err := repo.GetMembership(ctx, tt.wantSub)
			if err != nil {
				t.Fatalf("get membership %q: %v", tt.wantSub, err)
			}
			if m.Role != tt.wantRole {
				t.Errorf("role = %q, want %q", m.Role, tt.wantRole)
			}
		})
	}
}

// A membership is unique per subject across all orgs, so the configured admin's
// existing membership may belong to a different org than the one being seeded -
// which happens as soon as seed.orgSlug changes while the database persists.
// Promoting a row in the wrong org matches nothing, and returning that error
// from Run makes the server exit, crash-looping on an opaque "not found".
func TestRun_DoesNotFailWhenConfiguredAdminBelongsToAnotherOrg(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "old", AdminSub: "root"}); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	old, err := repo.GetOrgBySlug(ctx, "old")
	if err != nil {
		t.Fatalf("get old org: %v", err)
	}
	// A user who picked up a membership in the original org.
	if err := repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: old.Id, UserSub: "boss", Role: "viewer"}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	// The operator changes seed.orgSlug and names that user as admin.
	if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "new", AdminSub: "boss"}); err != nil {
		t.Fatalf("second seed must not fail the server startup: %v", err)
	}
}

// A stale pending invite must not prevent the break-glass promote. A pending row
// is not an admin (CountAdmins ignores it), so an org holding only a pending
// invite plus baseline-role members has zero admins; naming a signed-in user via
// seed.adminSub has to still work. Recovery by email cannot help here - the
// invite is for an address that user never signed in with - which is why
// adminSub is the reliable lever.
func TestRun_AStalePendingInviteDoesNotBlockPromotion(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme"}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	// An invite for the configured admin that has not been claimed.
	if err := repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "admin", Email: "boss@x.io"}); err != nil {
		t.Fatalf("invite: %v", err)
	}
	// The same person signed in under a different address and got the baseline.
	if err := repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "boss-sub", Role: "viewer", Email: "boss.work@x.io"}); err != nil {
		t.Fatalf("baseline row: %v", err)
	}
	if n, _ := repo.CountAdmins(ctx, "o1"); n != 0 {
		t.Fatalf("precondition: want 0 admins, got %d", n)
	}

	// Seeding with the sub of the signed-in user must promote them.
	if err := seed.Run(ctx, repo, seed.Config{OrgSlug: "acme", AdminSub: "boss-sub", AdminEmail: "boss@x.io"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, _ := repo.CountAdmins(ctx, "o1"); n != 1 {
		t.Errorf("admins = %d, want 1: the org must not be left without one", n)
	}
}
