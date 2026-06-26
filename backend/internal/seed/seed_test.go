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
