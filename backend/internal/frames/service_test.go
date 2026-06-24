package frames_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

func seedOrg(t *testing.T, repo *store.Memory, sub, role string) context.Context {
	t.Helper()
	ctx := context.Background()
	_ = repo.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams", DisplayName: "OpenTeams"})
	_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: sub, Role: role})
	return auth.WithClaims(ctx, &auth.Claims{Subject: sub, Email: sub + "@x"})
}

const sampleFrame = `name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
slots:
  rules:
    - Cite benchmarks.
`

func TestService_PublishThenGet(t *testing.T) {
	repo := store.NewMemory()
	pubCtx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	_, err := svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(sampleFrame)}))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	resp, err := svc.GetFrame(pubCtx, connect.NewRequest(&framesv1.GetFrameRequest{OrgSlug: "openteams", Name: "brand-voice"}))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.Msg.Frame.Name != "brand-voice" || !resp.Msg.Permissions.CanEdit {
		t.Fatalf("unexpected get response: %+v", resp.Msg)
	}
}

func TestService_ViewerCannotPublish(t *testing.T) {
	repo := store.NewMemory()
	viewerCtx := seedOrg(t, repo, "v", "viewer")
	svc := frames.NewService(repo)
	_, err := svc.PublishFrame(viewerCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(sampleFrame)}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", err)
	}
}

func TestService_CrossOrgGetIs404(t *testing.T) {
	repo := store.NewMemory()
	pubCtx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)
	_, _ = svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(sampleFrame)}))

	// a user in another org
	_ = repo.CreateOrg(context.Background(), &framesv1.Org{Id: "o2", Slug: "acme"})
	_ = repo.UpsertMembership(context.Background(), &framesv1.Membership{OrgId: "o2", UserSub: "outsider", Role: "admin"})
	outCtx := auth.WithClaims(context.Background(), &auth.Claims{Subject: "outsider"})

	_, err := svc.GetFrame(outCtx, connect.NewRequest(&framesv1.GetFrameRequest{OrgSlug: "openteams", Name: "brand-voice"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound (no existence leak), got %v", err)
	}
}

func TestService_GetMeReportsRole(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)
	resp, err := svc.GetMe(ctx, connect.NewRequest(&framesv1.GetMeRequest{}))
	if err != nil {
		t.Fatalf("getme: %v", err)
	}
	if resp.Msg.Role != "publisher" || !resp.Msg.CanCreate || resp.Msg.Org.Slug != "openteams" {
		t.Fatalf("unexpected GetMe: %+v", resp.Msg)
	}
}
