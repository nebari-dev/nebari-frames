package store_test

import (
	"context"
	"errors"
	"testing"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

func TestMemory_OrgAndMembershipRoundTrip(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()

	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams", DisplayName: "OpenTeams"}); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o2", Slug: "openteams"}); !errors.Is(err, store.ErrAlreadyExists) {
		t.Fatalf("duplicate slug: want ErrAlreadyExists, got %v", err)
	}
	got, err := m.GetOrgBySlug(ctx, "openteams")
	if err != nil || got.Id != "o1" {
		t.Fatalf("GetOrgBySlug: got %v, %v", got, err)
	}
	if _, err := m.GetMembership(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing membership: want ErrNotFound, got %v", err)
	}
}

func TestMemory_CreateFrameVersionAndGrants(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	_ = m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams"})

	in := store.CreateFrameVersionInput{
		Frame:      &framesv1.Frame{Id: "f1", OrgId: "o1", Name: "brand-voice", OwnerSub: "u1", LatestVersion: "1.0.0"},
		Version:    &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("x"), Digest: "d", PublishedBy: "u1"},
		Grants:     []store.Grant{{SubjectType: "user", SubjectID: "u1", Permission: "edit"}, {SubjectType: "org", SubjectID: "o1", Permission: "read"}},
		IsNewFrame: true,
	}
	if err := m.CreateFrameVersion(ctx, in); err != nil {
		t.Fatalf("CreateFrameVersion: %v", err)
	}
	grants, _ := m.FrameGrants(ctx, "f1")
	if len(grants) != 2 {
		t.Fatalf("want 2 grants, got %d", len(grants))
	}
	if err := m.CreateFrameVersion(ctx, in); !errors.Is(err, store.ErrAlreadyExists) {
		t.Fatalf("duplicate version: want ErrAlreadyExists, got %v", err)
	}
}
