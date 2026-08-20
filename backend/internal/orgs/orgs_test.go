package orgs_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/orgs"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	sqlitestore "github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
)

func TestResolveCallerReconcilesPendingByEmail(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	_ = repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "publisher", Email: "new@x.io"})

	// inject claims for a user with no sub-keyed membership yet
	ctx = auth.WithClaims(ctx, &auth.Claims{Subject: "sub-new", Email: "new@x.io"})

	caller, err := orgs.ResolveCaller(ctx, repo, orgs.DefaultMembership{})
	if err != nil {
		t.Fatalf("expected reconciliation, got err %v", err)
	}
	if caller.OrgID != "o1" || caller.Role != rbac.RolePublisher {
		t.Fatalf("unexpected caller %+v", caller)
	}
	// second call now resolves directly by sub
	if _, err := repo.GetMembership(ctx, "sub-new"); err != nil {
		t.Fatalf("membership should be active after reconcile: %v", err)
	}
}

func TestResolveCaller(t *testing.T) {
	repo := store.NewMemory()
	base := context.Background()
	_ = repo.UpsertMembership(base, &framesv1.Membership{OrgId: "o1", UserSub: "u1", Role: "publisher"})

	tests := []struct {
		name       string
		ctx        context.Context
		wantErr    error
		wantCaller rbac.Caller
	}{
		{
			name:    "no claims in context",
			ctx:     base,
			wantErr: orgs.ErrNoClaims,
		},
		{
			name:    "claims but no membership",
			ctx:     auth.WithClaims(base, &auth.Claims{Subject: "stranger"}),
			wantErr: orgs.ErrNoMembership,
		},
		{
			name: "happy path",
			ctx:  auth.WithClaims(base, &auth.Claims{Subject: "u1", Email: "u1@x"}),
			wantCaller: rbac.Caller{
				OrgID:   "o1",
				Role:    rbac.RolePublisher,
				Subject: "u1",
				Email:   "u1@x",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller, err := orgs.ResolveCaller(tt.ctx, repo, orgs.DefaultMembership{})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if caller.OrgID != tt.wantCaller.OrgID ||
				caller.Role != tt.wantCaller.Role ||
				caller.Subject != tt.wantCaller.Subject ||
				caller.Email != tt.wantCaller.Email {
				t.Fatalf("caller = %+v, want %+v", caller, tt.wantCaller)
			}
		})
	}
}

func TestResolveCallerDefaultRole(t *testing.T) {
	const orgSlug = "acme"

	// newRepo returns a store holding one org (acme) plus whatever the caller
	// seeds on top of it.
	newRepo := func(t *testing.T, seed func(context.Context, store.Repository)) store.Repository {
		t.Helper()
		ctx := context.Background()
		repo := store.NewMemory()
		if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: orgSlug, DisplayName: "Acme"}); err != nil {
			t.Fatalf("seed org: %v", err)
		}
		if seed != nil {
			seed(ctx, repo)
		}
		return repo
	}

	tests := []struct {
		name string
		seed func(context.Context, store.Repository)
		cfg  orgs.DefaultMembership
		// claims injected for the request under test
		subject string
		email   string

		wantErr       error
		wantRole      rbac.Role
		wantOrgID     string
		wantPersisted bool
	}{
		{
			name:    "no default configured denies an unknown caller",
			cfg:     orgs.DefaultMembership{},
			subject: "stranger",
			wantErr: orgs.ErrNoMembership,
		},
		{
			name:          "default role admits an unknown caller and persists the membership",
			cfg:           orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: orgSlug},
			subject:       "stranger",
			email:         "stranger@x.io",
			wantRole:      rbac.RoleViewer,
			wantOrgID:     "o1",
			wantPersisted: true,
		},
		{
			name: "an existing membership outranks the default",
			seed: func(ctx context.Context, repo store.Repository) {
				_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "u1", Role: "admin"})
			},
			cfg:           orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: orgSlug},
			subject:       "u1",
			wantRole:      rbac.RoleAdmin,
			wantOrgID:     "o1",
			wantPersisted: true,
		},
		{
			name: "a pending invite outranks the default",
			seed: func(ctx context.Context, repo store.Repository) {
				_ = repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "publisher", Email: "invited@x.io"})
			},
			cfg:           orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: orgSlug},
			subject:       "sub-invited",
			email:         "invited@x.io",
			wantRole:      rbac.RolePublisher,
			wantOrgID:     "o1",
			wantPersisted: true,
		},
		{
			name:    "an unresolvable default org denies rather than failing open",
			cfg:     orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "does-not-exist"},
			subject: "stranger",
			wantErr: orgs.ErrNoMembership,
		},
		{
			name:    "a default role with no org slug denies",
			cfg:     orgs.DefaultMembership{Role: rbac.RoleViewer},
			subject: "stranger",
			wantErr: orgs.ErrNoMembership,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t, tt.seed)
			ctx := auth.WithClaims(context.Background(), &auth.Claims{Subject: tt.subject, Email: tt.email})

			caller, err := orgs.ResolveCaller(ctx, repo, tt.cfg)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v, got caller %+v err %v", tt.wantErr, caller, err)
				}
				// A denied caller must never leave a membership row behind.
				if _, gerr := repo.GetMembership(ctx, tt.subject); !errors.Is(gerr, store.ErrNotFound) {
					t.Fatalf("denied caller should not be persisted, GetMembership err = %v", gerr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if caller.Role != tt.wantRole {
				t.Errorf("role = %q, want %q", caller.Role, tt.wantRole)
			}
			if caller.OrgID != tt.wantOrgID {
				t.Errorf("orgID = %q, want %q", caller.OrgID, tt.wantOrgID)
			}
			if caller.Subject != tt.subject {
				t.Errorf("subject = %q, want %q", caller.Subject, tt.subject)
			}
			if tt.wantPersisted {
				m, gerr := repo.GetMembership(ctx, tt.subject)
				if gerr != nil {
					t.Fatalf("membership should be persisted: %v", gerr)
				}
				if rbac.Role(m.Role) != tt.wantRole {
					t.Errorf("persisted role = %q, want %q", m.Role, tt.wantRole)
				}
			}
		})
	}
}

// A second request from the same defaulted user must not create a duplicate
// row nor reset a role an admin has since changed.
func TestResolveCallerDefaultRoleIsNotReappliedOnLaterRequests(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme"}); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	cfg := orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"}
	ctx = auth.WithClaims(ctx, &auth.Claims{Subject: "u1", Email: "u1@x.io"})

	if _, err := orgs.ResolveCaller(ctx, repo, cfg); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	// An admin promotes the user.
	if err := repo.UpdateMembershipRole(ctx, "o1", "u1", "", "publisher"); err != nil {
		t.Fatalf("promote: %v", err)
	}

	caller, err := orgs.ResolveCaller(ctx, repo, cfg)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if caller.Role != rbac.RolePublisher {
		t.Fatalf("role = %q, want publisher: the default must not overwrite a promoted role", caller.Role)
	}
}

// A Keycloak account that is deleted and recreated arrives with a new subject
// but the same email. The store holds one membership per (org, email), so
// provisioning the baseline row collides with the stale one. That must deny
// cleanly rather than surface as an internal error, so the operator sees the
// same "no access" page they would for any other denial.
//
// This runs against the real SQLite store on purpose: the in-memory fake does
// not enforce the (org_id, email) unique index, so the collision is invisible
// there.
func TestResolveCallerDefaultRoleEmailCollisionDenies(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := sqlitestore.New(db)

	if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme", DisplayName: "Acme", CreatedAt: timestamppb.Now()}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	// The stale membership from the user's previous Keycloak account.
	if err := repo.UpsertMembership(ctx, &framesv1.Membership{
		OrgId: "o1", UserSub: "old-sub", Role: "viewer", Email: "user@x.io", AddedAt: timestamppb.Now(),
	}); err != nil {
		t.Fatalf("seed stale membership: %v", err)
	}

	cfg := orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"}
	newCtx := auth.WithClaims(ctx, &auth.Claims{Subject: "new-sub", Email: "user@x.io"})

	caller, err := orgs.ResolveCaller(newCtx, repo, cfg)
	if !errors.Is(err, orgs.ErrNoMembership) {
		t.Fatalf("want ErrNoMembership, got caller %+v err %v", caller, err)
	}
}

// A token that carries no subject cannot identify anyone. Because the store
// treats an empty user_sub as a pending invite, provisioning such a caller would
// write a row that masquerades as an invite, hand it the default role, and squat
// the (org, email) slot so a real invite for that address can never be created.
// Reject it at the authorization boundary, before any store access.
func TestResolveCallerRejectsEmptySubject(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme"}); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	cfg := orgs.DefaultMembership{Role: rbac.RoleAdmin, OrgSlug: "acme"}
	ctx = auth.WithClaims(ctx, &auth.Claims{Subject: "", Email: "ghost@x.io"})

	caller, err := orgs.ResolveCaller(ctx, repo, cfg)
	if !errors.Is(err, orgs.ErrNoClaims) {
		t.Fatalf("want ErrNoClaims for a subjectless token, got caller %+v err %v", caller, err)
	}
	members, err := repo.ListMembershipsByOrg(ctx, "o1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("a subjectless caller wrote %d membership row(s): %+v", len(members), members)
	}
}

// Identity providers do not guarantee the case of the email claim, and an
// invite is typed by hand. Before a default role existed, a case mismatch just
// denied the user and the admin noticed. With a default role it would instead
// hand out the baseline role - which can be *higher* than the role they were
// invited with - so the mismatch must not silently escalate.
func TestResolveCallerMatchesInviteEmailCaseInsensitively(t *testing.T) {
	tests := []struct {
		name        string
		inviteEmail string
		claimEmail  string
	}{
		{name: "exact match", inviteEmail: "boss@x.io", claimEmail: "boss@x.io"},
		{name: "claim is upper", inviteEmail: "boss@x.io", claimEmail: "Boss@X.io"},
		{name: "invite is upper", inviteEmail: "Boss@X.IO", claimEmail: "boss@x.io"},
		{name: "claim has surrounding space", inviteEmail: "boss@x.io", claimEmail: " boss@x.io "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := store.NewMemory()
			if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme"}); err != nil {
				t.Fatalf("seed org: %v", err)
			}
			if err := repo.AddPendingMembership(ctx, &framesv1.Membership{
				OrgId: "o1", Role: "viewer", Email: tt.inviteEmail,
			}); err != nil {
				t.Fatalf("invite: %v", err)
			}
			// The default role deliberately outranks the invited role, so an
			// escalation is visible as a role mismatch rather than a denial.
			cfg := orgs.DefaultMembership{Role: rbac.RolePublisher, OrgSlug: "acme"}
			ctx = auth.WithClaims(ctx, &auth.Claims{Subject: "s1", Email: tt.claimEmail})

			caller, err := orgs.ResolveCaller(ctx, repo, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if caller.Role != rbac.RoleViewer {
				t.Errorf("role = %q, want viewer: the invite must win over the default role", caller.Role)
			}
		})
	}
}

// Concurrent first requests from the same user must converge on one membership
// row, and every caller must see the role that actually got stored rather than
// the one it optimistically assumed. Runs against real SQLite because the
// in-memory fake does not enforce the unique index that makes one writer lose.
func TestResolveCallerDefaultRoleConcurrentFirstRequests(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := sqlitestore.New(db)
	if err := repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "acme", CreatedAt: timestamppb.Now()}); err != nil {
		t.Fatalf("create org: %v", err)
	}

	cfg := orgs.DefaultMembership{Role: rbac.RoleViewer, OrgSlug: "acme"}
	reqCtx := auth.WithClaims(ctx, &auth.Claims{Subject: "u1", Email: "u1@x.io"})

	const n = 8
	roles := make([]rbac.Role, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			caller, err := orgs.ResolveCaller(reqCtx, repo, cfg)
			roles[i], errs[i] = caller.Role, err
		}(i)
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("request %d failed: %v", i, errs[i])
		}
		if roles[i] != rbac.RoleViewer {
			t.Errorf("request %d role = %q, want viewer", i, roles[i])
		}
	}
	members, err := repo.ListMembershipsByOrg(ctx, "o1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("got %d membership rows, want exactly 1", len(members))
	}
}
