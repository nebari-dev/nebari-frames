package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

func TestMemory_OrgAndMembership(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()

	// Seed an org used by the read scenarios below.
	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams", DisplayName: "OpenTeams"}); err != nil {
		t.Fatalf("seed CreateOrg: %v", err)
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "duplicate slug returns ErrAlreadyExists",
			run: func(t *testing.T) {
				err := m.CreateOrg(ctx, &framesv1.Org{Id: "o2", Slug: "openteams"})
				if !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("duplicate slug: want ErrAlreadyExists, got %v", err)
				}
			},
		},
		{
			name: "org round-trip by slug",
			run: func(t *testing.T) {
				got, err := m.GetOrgBySlug(ctx, "openteams")
				if err != nil || got.Id != "o1" {
					t.Fatalf("GetOrgBySlug: got %v, %v", got, err)
				}
			},
		},
		{
			name: "missing membership returns ErrNotFound",
			run: func(t *testing.T) {
				if _, err := m.GetMembership(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("missing membership: want ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestMemoryMembershipReads(t *testing.T) {
	ctx := context.Background()
	m := store.NewMemory()
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io"})
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "", Role: "viewer", Email: "p@x.io"})
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o2", UserSub: "s9", Role: "admin", Email: "z@x.io"})

	tests := []struct {
		name    string
		run     func() (int, error)
		wantLen int
	}{
		{"list org o1", func() (int, error) { l, e := m.ListMembershipsByOrg(ctx, "o1"); return len(l), e }, 2},
		{"count admins o1", func() (int, error) { return m.CountAdmins(ctx, "o1") }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.run()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantLen {
				t.Fatalf("got %d want %d", got, tt.wantLen)
			}
		})
	}

	pend, err := m.GetPendingMembershipByEmail(ctx, "p@x.io")
	if err != nil || pend.UserSub != "" || pend.OrgId != "o1" {
		t.Fatalf("pending lookup got %+v err %v", pend, err)
	}
	if _, err := m.GetPendingMembershipByEmail(ctx, "a@x.io"); err == nil {
		t.Fatal("active email should not match a pending lookup")
	}
}

func TestMemory_CreateFrameVersionAndGrants(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams"}); err != nil {
		t.Fatalf("seed CreateOrg: %v", err)
	}

	in := store.CreateFrameVersionInput{
		Frame:      &framesv1.Frame{Id: "f1", OrgId: "o1", Name: "brand-voice", OwnerSub: "u1", LatestVersion: "1.0.0"},
		Version:    &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("x"), Digest: "d", PublishedBy: "u1"},
		Grants:     []store.Grant{{SubjectType: "user", SubjectID: "u1", Permission: "edit"}, {SubjectType: "org", SubjectID: "o1", Permission: "read"}},
		IsNewFrame: true,
	}

	// These scenarios share mutable state and assert a sequence
	// (create -> grants present -> duplicate rejected), so run in order.
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create new frame version succeeds",
			run: func(t *testing.T) {
				if err := m.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("CreateFrameVersion: %v", err)
				}
			},
		},
		{
			name: "default grants are stored on new frame",
			run: func(t *testing.T) {
				grants, _ := m.FrameGrants(ctx, "f1")
				if len(grants) != 2 {
					t.Fatalf("want 2 grants, got %d", len(grants))
				}
			},
		},
		{
			name: "duplicate version returns ErrAlreadyExists",
			run: func(t *testing.T) {
				if err := m.CreateFrameVersion(ctx, in); !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("duplicate version: want ErrAlreadyExists, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
