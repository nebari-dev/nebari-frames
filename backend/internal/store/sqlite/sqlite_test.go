package sqlite_test

import (
	"context"
	"errors"
	"testing"

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
			{"user", "u1", "edit"},
			{"user", "u1", "delete"},
			{"org", "o1", "read"},
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
