package frames_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// newAdminService returns an admin caller context, a new Service, and the
// backing Memory repo. The org is "openteams" with slug "openteams".
func newAdminService(t *testing.T) (context.Context, *frames.Service, *store.Memory) {
	t.Helper()
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "admin", "admin")
	svc := frames.NewService(repo)
	return ctx, svc, repo
}

// publishFrame publishes a frame from YAML bytes, fataling on error.
func publishFrame(t *testing.T, ctx context.Context, svc *frames.Service, content []byte) {
	t.Helper()
	_, err := svc.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: content}))
	if err != nil {
		t.Fatalf("publish frame: %v", err)
	}
}

// connectErrorDetails unpacks details from a connect.Error, fataling if err
// is not a *connect.Error. Returns the proto.Message from each detail.
func connectErrorDetails(t *testing.T, err error) []proto.Message {
	t.Helper()
	ce, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	out := make([]proto.Message, 0, len(ce.Details()))
	for _, d := range ce.Details() {
		msg, verr := d.Value()
		if verr != nil {
			t.Logf("connectErrorDetails: failed to decode detail: %v", verr)
			continue
		}
		out = append(out, msg)
	}
	return out
}

func TestDeleteFrameBlockThenForce(t *testing.T) {
	ctx, svc, repo := newAdminService(t)

	// Publish parent frame.
	publishFrame(t, ctx, svc, []byte(`name: parent
description: Parent frame
version: 1.0.0
slots:
  rules:
    - Parent rule.
`))

	// Publish child frame that extends parent.
	publishFrame(t, ctx, svc, []byte(`name: child
description: Child extending parent
version: 1.0.0
extends:
  - ref: openteams/parent
    version: 1.0.0
slots:
  rules:
    - Child rule.
`))

	// force=false should block and list the child.
	_, err := svc.DeleteFrame(ctx, connect.NewRequest(&framesv1.DeleteFrameRequest{
		OrgSlug: "openteams",
		Name:    "parent",
	}))
	if err == nil {
		t.Fatal("expected block error, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", got)
	}
	var blocked *framesv1.DeleteBlocked
	for _, d := range connectErrorDetails(t, err) {
		if b, ok := d.(*framesv1.DeleteBlocked); ok {
			blocked = b
		}
	}
	if blocked == nil {
		t.Fatal("no DeleteBlocked detail found in error")
	}
	if len(blocked.BlockingFrames) != 1 || blocked.BlockingFrames[0] != "openteams/child" {
		t.Fatalf("unexpected blocking frames: %v", blocked.BlockingFrames)
	}

	// force=true should detach and delete; child must survive.
	if _, err := svc.DeleteFrame(ctx, connect.NewRequest(&framesv1.DeleteFrameRequest{
		OrgSlug: "openteams",
		Name:    "parent",
		Force:   true,
	})); err != nil {
		t.Fatalf("force delete failed: %v", err)
	}

	if _, err := repo.GetFrameBySlugName(ctx, "openteams", "parent"); err != store.ErrNotFound {
		t.Fatalf("parent should be gone, got %v", err)
	}
	if _, err := repo.GetFrameBySlugName(ctx, "openteams", "child"); err != nil {
		t.Fatalf("child should survive, got %v", err)
	}
}

func TestDeleteFrameDeniedForViewer(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, repo *store.Memory) context.Context
	}{
		{
			name: "viewer in same org gets NotFound",
			setup: func(t *testing.T, repo *store.Memory) context.Context {
				return seedOrg(t, repo, "viewer", "viewer")
			},
		},
		{
			name: "user in different org gets NotFound",
			setup: func(t *testing.T, repo *store.Memory) context.Context {
				return seedSecondOrg(t, repo, "outsider", "admin")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			// Publish a frame as admin.
			adminCtx := seedOrg(t, repo, "admin", "admin")
			svc := frames.NewService(repo)
			publishFrame(t, adminCtx, svc, []byte(`name: secret-frame
description: Admin frame
version: 1.0.0
slots:
  rules:
    - Only admins.
`))

			// Non-deleter attempts to delete.
			callerCtx := tt.setup(t, repo)
			_, err := svc.DeleteFrame(callerCtx, connect.NewRequest(&framesv1.DeleteFrameRequest{
				OrgSlug: "openteams",
				Name:    "secret-frame",
			}))
			if err == nil {
				t.Fatal("expected NotFound error, got nil")
			}
			if got := connect.CodeOf(err); got != connect.CodeNotFound {
				t.Fatalf("want NotFound (no existence leak), got %v", got)
			}
		})
	}
}
