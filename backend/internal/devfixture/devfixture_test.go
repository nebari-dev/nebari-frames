package devfixture_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/devfixture"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

func newRepoWithOrg(t *testing.T) (*store.Memory, *framesv1.Org) {
	t.Helper()
	repo := store.NewMemory()
	org := &framesv1.Org{Id: "org1", Slug: "dev-org", DisplayName: "Dev Org", CreatedAt: timestamppb.Now()}
	if err := repo.CreateOrg(context.Background(), org); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	return repo, org
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		times int
	}{
		{name: "single load seeds fixture", times: 1},
		{name: "double load is idempotent", times: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, org := newRepoWithOrg(t)
			for i := 0; i < tt.times; i++ {
				if err := devfixture.Load(ctx, repo, "dev-org"); err != nil {
					t.Fatalf("Load (call %d): %v", i+1, err)
				}
			}

			mems, err := repo.ListMembershipsByOrg(ctx, org.Id)
			if err != nil {
				t.Fatalf("ListMembershipsByOrg: %v", err)
			}
			if len(mems) != 3 {
				t.Fatalf("members = %d, want 3", len(mems))
			}

			fr, err := repo.ListFramesByOrg(ctx, org.Id)
			if err != nil {
				t.Fatalf("ListFramesByOrg: %v", err)
			}
			if len(fr) != 4 {
				t.Fatalf("frames = %d, want 4", len(fr))
			}

			base, err := repo.GetFrameBySlugName(ctx, "dev-org", "base-ml-env")
			if err != nil {
				t.Fatalf("GetFrameBySlugName base-ml-env: %v", err)
			}
			vers, err := repo.ListFrameVersions(ctx, base.Id)
			if err != nil {
				t.Fatalf("ListFrameVersions: %v", err)
			}
			if len(vers) != 2 {
				t.Fatalf("base-ml-env versions = %d, want 2", len(vers))
			}

			children, err := repo.FrameChildren(ctx, base.Id)
			if err != nil {
				t.Fatalf("FrameChildren: %v", err)
			}
			found := false
			for _, c := range children {
				if c.Name == "pytorch-gpu" {
					found = true
				}
			}
			if !found {
				t.Fatalf("pytorch-gpu not found as child of base-ml-env")
			}
		})
	}
}

func TestLoad_RequiresSeededOrg(t *testing.T) {
	repo := store.NewMemory()
	if err := devfixture.Load(context.Background(), repo, "missing-org"); err == nil {
		t.Fatal("expected error when org is not seeded, got nil")
	}
}
