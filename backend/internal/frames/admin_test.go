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

// orgID returns the ID of the "openteams" org seeded in repo.
func orgID(t *testing.T, repo *store.Memory) string {
	t.Helper()
	org, err := repo.GetOrgBySlug(context.Background(), "openteams")
	if err != nil {
		t.Fatalf("orgID: %v", err)
	}
	return org.Id
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
		name  string
		setup func(t *testing.T, repo *store.Memory) context.Context
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

func TestListOrgMembersAdminOnly(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		wantCode connect.Code // 0 means OK
	}{
		{name: "admin sees members", role: "admin", wantCode: 0},
		{name: "viewer gets PermissionDenied", role: "viewer", wantCode: connect.CodePermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			ctx := seedOrg(t, repo, tt.role, tt.role)
			svc := frames.NewService(repo)

			// Seed a pending membership so the list is non-empty.
			_ = repo.AddPendingMembership(ctx, &framesv1.Membership{
				OrgId: orgID(t, repo), Role: "viewer", Email: "p@x.io",
			})

			resp, err := svc.ListOrgMembers(ctx, connect.NewRequest(&framesv1.ListOrgMembersRequest{}))
			if tt.wantCode != 0 {
				if connect.CodeOf(err) != tt.wantCode {
					t.Fatalf("want code %v, got %v (err=%v)", tt.wantCode, connect.CodeOf(err), err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Msg.Members) < 1 {
				t.Fatalf("expected at least 1 member, got %d", len(resp.Msg.Members))
			}
		})
	}
}

func TestSetMemberRoleAndLastAdminGuard(t *testing.T) {
	ctx, svc, repo := newAdminService(t) // admin caller sub="admin"
	oid := orgID(t, repo)

	// demoting the only admin is rejected
	if _, err := svc.SetMemberRole(ctx, connect.NewRequest(&framesv1.SetMemberRoleRequest{UserSub: "admin", Role: "viewer"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("last-admin demote should fail, got %v", err)
	}

	// add a second admin, then demotion of the first is allowed
	_ = repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: oid, Role: "admin", Email: "a2@x.io"})
	_ = repo.ActivatePendingMembership(ctx, "a2@x.io", "admin2")
	if _, err := svc.SetMemberRole(ctx, connect.NewRequest(&framesv1.SetMemberRoleRequest{UserSub: "admin", Role: "viewer"})); err != nil {
		t.Fatalf("demote with 2 admins should succeed, got %v", err)
	}
}

func TestRemoveOrgMemberLastAdminGuard(t *testing.T) {
	ctx, svc, repo := newAdminService(t)
	_ = repo
	if _, err := svc.RemoveOrgMember(ctx, connect.NewRequest(&framesv1.RemoveOrgMemberRequest{UserSub: "admin"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("removing last admin should fail, got %v", err)
	}
}

func TestAddOrgMemberCreatesPending(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		role     string
		wantCode connect.Code // 0 means OK
	}{
		{name: "valid publisher invite", email: "new@x.io", role: "publisher", wantCode: 0},
		{name: "invalid role returns InvalidArgument", email: "x@x.io", role: "wizard", wantCode: connect.CodeInvalidArgument},
		{name: "empty email returns InvalidArgument", email: "", role: "viewer", wantCode: connect.CodeInvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, svc, repo := newAdminService(t)

			resp, err := svc.AddOrgMember(ctx, connect.NewRequest(&framesv1.AddOrgMemberRequest{
				Email: tt.email, Role: tt.role,
			}))
			if tt.wantCode != 0 {
				if connect.CodeOf(err) != tt.wantCode {
					t.Fatalf("want code %v, got %v (err=%v)", tt.wantCode, connect.CodeOf(err), err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Msg.Member.UserSub != "" || resp.Msg.Member.Email != tt.email || resp.Msg.Member.Role != tt.role {
				t.Fatalf("unexpected member %+v", resp.Msg.Member)
			}
			if _, err := repo.GetPendingMembershipByEmail(ctx, tt.email); err != nil {
				t.Fatalf("pending row not persisted: %v", err)
			}
		})
	}
}

func TestMembershipMutations_ViewerGetsPermissionDenied(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context, svc *frames.Service) error
	}{
		{
			name: "AddOrgMember denied for viewer",
			call: func(ctx context.Context, svc *frames.Service) error {
				_, err := svc.AddOrgMember(ctx, connect.NewRequest(&framesv1.AddOrgMemberRequest{
					Email: "x@x.io",
					Role:  "viewer",
				}))
				return err
			},
		},
		{
			name: "SetMemberRole denied for viewer",
			call: func(ctx context.Context, svc *frames.Service) error {
				_, err := svc.SetMemberRole(ctx, connect.NewRequest(&framesv1.SetMemberRoleRequest{
					UserSub: "someone",
					Role:    "viewer",
				}))
				return err
			},
		},
		{
			name: "RemoveOrgMember denied for viewer",
			call: func(ctx context.Context, svc *frames.Service) error {
				_, err := svc.RemoveOrgMember(ctx, connect.NewRequest(&framesv1.RemoveOrgMemberRequest{
					UserSub: "someone",
				}))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			viewerCtx := seedOrg(t, repo, "v", "viewer")
			svc := frames.NewService(repo)
			err := tt.call(viewerCtx, svc)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("want PermissionDenied, got code=%v err=%v", connect.CodeOf(err), err)
			}
		})
	}
}
