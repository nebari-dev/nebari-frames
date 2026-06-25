package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"google.golang.org/protobuf/types/known/timestamppb"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
)

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	db, err := sqlite.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return sqlite.New(db)
}

// seedOrg creates a test org and fails the test if it errors.
func seedOrg(t *testing.T, r *sqlite.Repository, id, slug string) {
	t.Helper()
	ctx := context.Background()
	now := timestamppb.Now()
	if err := r.CreateOrg(ctx, &framesv1.Org{Id: id, Slug: slug, DisplayName: slug, CreatedAt: now}); err != nil {
		t.Fatalf("CreateOrg %s: %v", id, err)
	}
}

// baseInput returns a standard CreateFrameVersionInput for use in tests.
func baseInput(now *timestamppb.Timestamp) store.CreateFrameVersionInput {
	return store.CreateFrameVersionInput{
		Frame: &framesv1.Frame{
			Id: "f1", OrgId: "o1", Name: "brand-voice",
			Description: "d", OwnerSub: "u1",
			LatestVersion: "1.0.0",
			CreatedAt:     now, UpdatedAt: now,
		},
		Version: &framesv1.FrameVersion{
			Version: "1.0.0", Content: []byte("name: brand-voice\n"),
			Digest: "d", SizeBytes: 18,
			PublishedBy: "u1", PublishedAt: now,
		},
		Grants: []store.Grant{
			{SubjectType: "user", SubjectID: "u1", Permission: "edit"},
			{SubjectType: "user", SubjectID: "u1", Permission: "delete"},
			{SubjectType: "org", SubjectID: "o1", Permission: "read"},
		},
		IsNewFrame: true,
	}
}

func TestSQLite_PublishInsertsVersionAndDefaultGrants(t *testing.T) {
	ctx := context.Background()
	now := timestamppb.Now()

	tests := []struct {
		name    string
		run     func(t *testing.T, r *sqlite.Repository)
	}{
		{
			name: "publish-new inserts version and 3 default grants",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				in := baseInput(now)
				if err := r.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("CreateFrameVersion: %v", err)
				}
				grants, err := r.FrameGrants(ctx, "f1")
				if err != nil {
					t.Fatalf("FrameGrants: %v", err)
				}
				if len(grants) != 3 {
					t.Fatalf("want 3 grants, got %d: %v", len(grants), grants)
				}
			},
		},
		{
			name: "GetFrameBySlugName round-trips",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				in := baseInput(now)
				if err := r.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("CreateFrameVersion: %v", err)
				}
				got, err := r.GetFrameBySlugName(ctx, "openteams", "brand-voice")
				if err != nil {
					t.Fatalf("GetFrameBySlugName: %v", err)
				}
				if got.Id != "f1" {
					t.Errorf("want id=f1, got %s", got.Id)
				}
				if got.OrgId != "o1" {
					t.Errorf("want org_id=o1, got %s", got.OrgId)
				}
				if got.Name != "brand-voice" {
					t.Errorf("want name=brand-voice, got %s", got.Name)
				}
			},
		},
		{
			name: "duplicate publish returns ErrAlreadyExists",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				in := baseInput(now)
				if err := r.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("first publish: %v", err)
				}
				err := r.CreateFrameVersion(ctx, in)
				if !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("want ErrAlreadyExists on duplicate, got %v", err)
				}
			},
		},
		{
			name: "GetFrameVersion returns content and extends edges",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				// Create parent frame first
				parentNow := now
				parentIn := store.CreateFrameVersionInput{
					Frame: &framesv1.Frame{
						Id: "fp1", OrgId: "o1", Name: "base-voice",
						Description: "base", OwnerSub: "u1",
						LatestVersion: "1.0.0",
						CreatedAt:     parentNow, UpdatedAt: parentNow,
					},
					Version: &framesv1.FrameVersion{
						Version: "1.0.0", Content: []byte("name: base-voice\n"),
						Digest: "dp", SizeBytes: 17,
						PublishedBy: "u1", PublishedAt: parentNow,
					},
					IsNewFrame: true,
				}
				if err := r.CreateFrameVersion(ctx, parentIn); err != nil {
					t.Fatalf("parent CreateFrameVersion: %v", err)
				}

				// Create child frame with extends edge
				in := store.CreateFrameVersionInput{
					Frame: &framesv1.Frame{
						Id: "f2", OrgId: "o1", Name: "child-voice",
						Description: "child", OwnerSub: "u1",
						LatestVersion: "1.0.0",
						CreatedAt:     now, UpdatedAt: now,
					},
					Version: &framesv1.FrameVersion{
						Version: "1.0.0", Content: []byte("name: child-voice\n"),
						Digest: "dc", SizeBytes: 18,
						PublishedBy: "u1", PublishedAt: now,
					},
					Extends: []store.ParentEdge{
						{ParentFrameID: "fp1", ParentVersion: "1.0.0", OrderIndex: 0},
					},
					IsNewFrame: true,
				}
				if err := r.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("child CreateFrameVersion: %v", err)
				}

				v, edges, excludes, err := r.GetFrameVersion(ctx, "f2", "1.0.0")
				if err != nil {
					t.Fatalf("GetFrameVersion: %v", err)
				}
				if string(v.Content) != "name: child-voice\n" {
					t.Errorf("wrong content: %q", v.Content)
				}
				if len(edges) != 1 {
					t.Fatalf("want 1 extends edge, got %d", len(edges))
				}
				if edges[0].ParentFrameID != "fp1" {
					t.Errorf("wrong parent frame id: %s", edges[0].ParentFrameID)
				}
				if len(excludes) != 0 {
					t.Errorf("want 0 excludes, got %d", len(excludes))
				}
			},
		},
		{
			name: "GetFrameBySlugName not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				_, err := r.GetFrameBySlugName(ctx, "openteams", "nonexistent")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "GetFrameByID not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				_, err := r.GetFrameByID(ctx, "does-not-exist")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "GetFrameVersion not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				_, _, _, err := r.GetFrameVersion(ctx, "no-frame", "1.0.0")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "GetOrgByID round-trips",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				got, err := r.GetOrgByID(ctx, "o1")
				if err != nil {
					t.Fatalf("GetOrgByID: %v", err)
				}
				if got.Slug != "openteams" {
					t.Errorf("want slug=openteams, got %s", got.Slug)
				}
			},
		},
		{
			name: "GetOrgBySlug not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				_, err := r.GetOrgBySlug(ctx, "nobody")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "GetMembership not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				_, err := r.GetMembership(ctx, "ghost@example.com")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			tc.run(t, r)
		})
	}
}

func TestSQLite_MembershipReads(t *testing.T) {
	ctx := context.Background()

	type result struct {
		count int
		err   error
	}

	tests := []struct {
		name    string
		run     func(r *sqlite.Repository) result
		wantLen int
	}{
		{
			name: "list org o1 returns 2 memberships",
			run: func(r *sqlite.Repository) result {
				l, e := r.ListMembershipsByOrg(ctx, "o1")
				return result{len(l), e}
			},
			wantLen: 2,
		},
		{
			name: "count admins o1 returns 1",
			run: func(r *sqlite.Repository) result {
				n, e := r.CountAdmins(ctx, "o1")
				return result{n, e}
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRepo(t)
			seedOrg(t, r, "o1", "openteams")
			seedOrg(t, r, "o2", "other")
			now := timestamppb.Now()
			_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io", AddedAt: now})
			_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "", Role: "viewer", Email: "p@x.io", AddedAt: now})
			_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o2", UserSub: "s9", Role: "admin", Email: "z@x.io", AddedAt: now})
			res := tt.run(r)
			if res.err != nil {
				t.Fatalf("unexpected error: %v", res.err)
			}
			if res.count != tt.wantLen {
				t.Fatalf("got %d want %d", res.count, tt.wantLen)
			}
		})
	}

	// GetPendingMembershipByEmail tests run separately since they are not easily
	// reducible to a single count result.
	t.Run("pending lookup returns correct row", func(t *testing.T) {
		r := newRepo(t)
		seedOrg(t, r, "o1", "openteams")
		now := timestamppb.Now()
		_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io", AddedAt: now})
		_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "", Role: "viewer", Email: "p@x.io", AddedAt: now})

		pend, err := r.GetPendingMembershipByEmail(ctx, "p@x.io")
		if err != nil || pend.UserSub != "" || pend.OrgId != "o1" {
			t.Fatalf("pending lookup got %+v err %v", pend, err)
		}
	})

	t.Run("active email does not match pending lookup", func(t *testing.T) {
		r := newRepo(t)
		seedOrg(t, r, "o1", "openteams")
		now := timestamppb.Now()
		_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io", AddedAt: now})

		if _, err := r.GetPendingMembershipByEmail(ctx, "a@x.io"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("active email should return ErrNotFound for pending lookup, got %v", err)
		}
	})
}

// seedFrameWithVersions opens a fresh in-memory repo, creates a test org and
// frame, and publishes one version per entry in versions (in order), using
// monotonically increasing published_at values. Returns the repo and frame ID.
func seedFrameWithVersions(t *testing.T, versions []string) (*sqlite.Repository, string) {
	t.Helper()
	r := newRepo(t)
	ctx := context.Background()
	seedOrg(t, r, "o1", "openteams")
	base := timestamppb.Now()
	frameID := "fv1"
	for i, ver := range versions {
		ts := timestamppb.New(base.AsTime().Add(time.Duration(i) * time.Second))
		in := store.CreateFrameVersionInput{
			Frame: &framesv1.Frame{
				Id: frameID, OrgId: "o1", Name: "multi-ver",
				Description: "d", OwnerSub: "u1",
				LatestVersion: ver,
				CreatedAt:     base, UpdatedAt: ts,
			},
			Version: &framesv1.FrameVersion{
				Version:     ver,
				Content:     []byte("name: multi-ver\n"),
				Digest:      "d" + ver,
				SizeBytes:   16,
				PublishedBy: "u1",
				PublishedAt: ts,
				Changelog:   "release " + ver,
			},
			IsNewFrame: i == 0,
		}
		if err := r.CreateFrameVersion(ctx, in); err != nil {
			t.Fatalf("seedFrameWithVersions CreateFrameVersion(%s): %v", ver, err)
		}
	}
	return r, frameID
}

func TestListFrameVersions(t *testing.T) {
	tests := []struct {
		name     string
		versions []string // published in this order
		wantLen  int
		wantTop  string // newest first
	}{
		{name: "multiple versions newest first", versions: []string{"1.0.0", "1.1.0", "2.0.0"}, wantLen: 3, wantTop: "2.0.0"},
		{name: "single version", versions: []string{"1.0.0"}, wantLen: 1, wantTop: "1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, frameID := seedFrameWithVersions(t, tt.versions)
			got, err := repo.ListFrameVersions(context.Background(), frameID)
			if err != nil {
				t.Fatalf("ListFrameVersions: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
			if got[0].Version != tt.wantTop {
				t.Errorf("top version = %q, want %q", got[0].Version, tt.wantTop)
			}
		})
	}
}

func TestSQLite_MembershipWrites(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T, r *sqlite.Repository)
	}{
		{
			name: "AddPendingMembership duplicate returns ErrAlreadyExists",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()
				mem := &framesv1.Membership{OrgId: "o1", Role: "viewer", Email: "p@x.io", AddedAt: now}
				if err := r.AddPendingMembership(ctx, mem); err != nil {
					t.Fatalf("first AddPendingMembership: %v", err)
				}
				if err := r.AddPendingMembership(ctx, mem); !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("want ErrAlreadyExists, got %v", err)
				}
			},
		},
		{
			name: "ActivatePendingMembership sets user_sub and GetMembership returns row",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()
				if err := r.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "viewer", Email: "p@x.io", AddedAt: now}); err != nil {
					t.Fatalf("AddPendingMembership: %v", err)
				}
				if err := r.ActivatePendingMembership(ctx, "p@x.io", "sub-1"); err != nil {
					t.Fatalf("ActivatePendingMembership: %v", err)
				}
				got, err := r.GetMembership(ctx, "sub-1")
				if err != nil {
					t.Fatalf("GetMembership: %v", err)
				}
				if got.OrgId != "o1" || got.Email != "p@x.io" {
					t.Fatalf("unexpected membership: %+v", got)
				}
			},
		},
		{
			name: "ActivatePendingMembership not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				if err := r.ActivatePendingMembership(ctx, "nobody@x.io", "sub-x"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "UpdateMembershipRole by userSub succeeds",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()
				_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "viewer", Email: "a@x.io", AddedAt: now})
				if err := r.UpdateMembershipRole(ctx, "o1", "s1", "", "admin"); err != nil {
					t.Fatalf("UpdateMembershipRole: %v", err)
				}
				got, err := r.GetMembership(ctx, "s1")
				if err != nil || got.Role != "admin" {
					t.Fatalf("role not updated: %+v err %v", got, err)
				}
			},
		},
		{
			name: "UpdateMembershipRole not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				if err := r.UpdateMembershipRole(ctx, "o1", "ghost", "", "admin"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "DeleteMembership by userSub succeeds",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()
				_ = r.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "viewer", Email: "a@x.io", AddedAt: now})
				if err := r.DeleteMembership(ctx, "o1", "s1", ""); err != nil {
					t.Fatalf("DeleteMembership: %v", err)
				}
				if _, err := r.GetMembership(ctx, "s1"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound after delete, got %v", err)
				}
			},
		},
		{
			name: "DeleteMembership not-found returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				if err := r.DeleteMembership(ctx, "o1", "ghost", ""); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			tc.run(t, r)
		})
	}
}

func TestSQLite_DeleteFrameDetachesChildren(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T, r *sqlite.Repository)
	}{
		{
			name: "delete parent - child survives and edge is detached",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()

				parentIn := store.CreateFrameVersionInput{
					Frame: &framesv1.Frame{
						Id: "p", OrgId: "o1", Name: "parent",
						OwnerSub: "u1", LatestVersion: "1.0.0",
						CreatedAt: now, UpdatedAt: now,
					},
					Version:    &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("p"), Digest: "dp", SizeBytes: 1, PublishedBy: "u1", PublishedAt: now},
					IsNewFrame: true,
				}
				if err := r.CreateFrameVersion(ctx, parentIn); err != nil {
					t.Fatalf("create parent: %v", err)
				}

				childIn := store.CreateFrameVersionInput{
					Frame: &framesv1.Frame{
						Id: "c", OrgId: "o1", Name: "child",
						OwnerSub: "u1", LatestVersion: "1.0.0",
						CreatedAt: now, UpdatedAt: now,
					},
					Version:    &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("c"), Digest: "dc", SizeBytes: 1, PublishedBy: "u1", PublishedAt: now},
					Extends:    []store.ParentEdge{{ParentFrameID: "p", ParentVersion: "1.0.0", OrderIndex: 0}},
					IsNewFrame: true,
				}
				if err := r.CreateFrameVersion(ctx, childIn); err != nil {
					t.Fatalf("create child: %v", err)
				}

				kids, err := r.FrameChildren(ctx, "p")
				if err != nil {
					t.Fatalf("FrameChildren before delete: %v", err)
				}
				if len(kids) != 1 || kids[0].Id != "c" {
					t.Fatalf("expected 1 child 'c', got %+v", kids)
				}

				if err := r.DeleteFrame(ctx, "p"); err != nil {
					t.Fatalf("DeleteFrame: %v", err)
				}

				if _, err := r.GetFrameByID(ctx, "p"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("parent should be gone, got %v", err)
				}

				if _, err := r.GetFrameByID(ctx, "c"); err != nil {
					t.Fatalf("child should survive, got %v", err)
				}

				kids, err = r.FrameChildren(ctx, "p")
				if err != nil {
					t.Fatalf("FrameChildren after delete: %v", err)
				}
				if len(kids) != 0 {
					t.Fatalf("edges should be detached, got %+v", kids)
				}
			},
		},
		{
			name: "delete non-existent frame returns ErrNotFound",
			run: func(t *testing.T, r *sqlite.Repository) {
				if err := r.DeleteFrame(ctx, "does-not-exist"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("want ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			tc.run(t, r)
		})
	}
}

// TestSQLite_PublishAtomicRollback verifies that a CreateFrameVersion call that
// fails mid-transaction (due to a FK violation on frame_extends) rolls back the
// entire transaction: no frames row, no grants row, and no frame version are
// left behind.
func TestSQLite_PublishAtomicRollback(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T, r *sqlite.Repository)
	}{
		{
			name: "FK violation on extends rolls back frame and grants",
			run: func(t *testing.T, r *sqlite.Repository) {
				seedOrg(t, r, "o1", "openteams")
				now := timestamppb.Now()

				in := store.CreateFrameVersionInput{
					Frame: &framesv1.Frame{
						Id: "f-rollback", OrgId: "o1", Name: "bad-child",
						Description: "d", OwnerSub: "u1",
						LatestVersion: "1.0.0",
						CreatedAt:     now, UpdatedAt: now,
					},
					Version: &framesv1.FrameVersion{
						Version: "1.0.0", Content: []byte("name: bad-child\n"),
						Digest: "d", SizeBytes: 16,
						PublishedBy: "u1", PublishedAt: now,
					},
					Extends: []store.ParentEdge{
						{ParentFrameID: "does-not-exist", ParentVersion: "1.0.0", OrderIndex: 0},
					},
					Grants: []store.Grant{
						{SubjectType: "org", SubjectID: "o1", Permission: "read"},
					},
					IsNewFrame: true,
				}

				// (a) CreateFrameVersion must return a non-nil error.
				err := r.CreateFrameVersion(ctx, in)
				if err == nil {
					t.Fatal("want error from FK violation, got nil")
				}

				// (b) Transaction must have rolled back - frame row must not exist.
				_, getErr := r.GetFrameBySlugName(ctx, "openteams", "bad-child")
				if !errors.Is(getErr, store.ErrNotFound) {
					t.Fatalf("rollback check: want ErrNotFound for frame, got %v", getErr)
				}

				// (c) Grants must also be absent (grants are inserted in same tx).
				grants, grantsErr := r.FrameGrants(ctx, "f-rollback")
				if grantsErr != nil {
					t.Fatalf("rollback check: FrameGrants returned unexpected error: %v", grantsErr)
				}
				if len(grants) != 0 {
					t.Fatalf("rollback check: want 0 grants after rollback, got %d", len(grants))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			tc.run(t, r)
		})
	}
}
