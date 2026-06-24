package frames_test

import (
	"bytes"
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

// seedSecondOrg creates a separate org (acme) and a member, returning that
// member's claims context. Used for cross-org isolation tests.
func seedSecondOrg(t *testing.T, repo *store.Memory, sub, role string) context.Context {
	t.Helper()
	ctx := context.Background()
	_ = repo.CreateOrg(ctx, &framesv1.Org{Id: "o2", Slug: "acme", DisplayName: "Acme"})
	_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o2", UserSub: sub, Role: role})
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
	outCtx := seedSecondOrg(t, repo, "outsider", "admin")

	_, err := svc.GetFrame(outCtx, connect.NewRequest(&framesv1.GetFrameRequest{OrgSlug: "openteams", Name: "brand-voice"}))
	if err == nil {
		t.Fatal("want NotFound error, got nil (existence leak)")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound (no existence leak), got %v", err)
	}
}

const parentFrame = `name: base-voice
description: Base voice frame
version: 1.0.0
slots:
  rules:
    - Always cite sources.
`

const childWithSameOrgRef = `name: brand-voice
description: Brand voice extending base
version: 1.0.0
extends:
  - ref: openteams/base-voice
    version: 1.0.0
slots:
  rules:
    - Cite benchmarks.
`

// TestService_ResolveSameOrgParent verifies that a child frame extending a
// same-org parent resolves successfully and pulls in the parent's contributed
// slot content. The readFetcher resolves each parent ref against the caller's
// org slug (mirroring PublishFrame); previously it used an empty fallback org
// slug, a latent break for any same-org ref that omits the slug prefix.
//
// NOTE: schema Validate currently requires extends refs to be fully qualified
// (org_slug/frame_name), so a truly bare ref cannot be published. This test
// exercises the same-org resolution path end-to-end with the stored
// (validated) "openteams/base-voice" form. The splitRef fallback fix removes
// the latent landmine should validation ever permit bare refs.
func TestService_ResolveSameOrgParent(t *testing.T) {
	repo := store.NewMemory()
	pubCtx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	if _, err := svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(parentFrame)})); err != nil {
		t.Fatalf("publish parent: %v", err)
	}
	if _, err := svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(childWithSameOrgRef)})); err != nil {
		t.Fatalf("publish child: %v", err)
	}

	resp, err := svc.ResolveFrame(pubCtx, connect.NewRequest(&framesv1.ResolveFrameRequest{OrgSlug: "openteams", Name: "brand-voice"}))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// The resolved YAML must include the parent's contributed rule.
	if !bytes.Contains(resp.Msg.ResolvedContent, []byte("Always cite sources.")) {
		t.Fatalf("resolved content missing parent rule; got:\n%s", resp.Msg.ResolvedContent)
	}
	// And the child's own rule.
	if !bytes.Contains(resp.Msg.ResolvedContent, []byte("Cite benchmarks.")) {
		t.Fatalf("resolved content missing child rule; got:\n%s", resp.Msg.ResolvedContent)
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
