package frames_test

import (
	"bytes"
	"context"
	"errors"
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

// TestService_CrossOrgParentReadEnforcement verifies that an org-A publisher
// cannot probe the existence of an org-B frame by referencing it in extends.
// The publish must fail with CodeInvalidArgument, and the error code must be
// identical to the case where the ref names a truly non-existent frame
// (no oracle distinction between "denied" and "absent").
func TestService_CrossOrgParentReadEnforcement(t *testing.T) {
	const secretFrameYAML = `name: secret
description: Secret frame for org B
version: 1.0.0
slots:
  rules:
    - Internal only.
`
	// childExtending builds a publishable child frame YAML that extends the
	// given fully-qualified ref (e.g. "acme/secret") at version 1.0.0.
	childExtending := func(ref string) []byte {
		return []byte(`name: child-frame
description: Child extending cross-org parent
version: 1.0.0
extends:
  - ref: ` + ref + `
    version: 1.0.0
slots:
  rules:
    - Some rule.
`)
	}

	tests := []struct {
		name     string
		childRef string // extends ref without @version
	}{
		{
			name:     "cross-org existing frame is denied (same code as absent)",
			childRef: "acme/secret",
		},
		{
			name:     "cross-org nonexistent frame",
			childRef: "acme/does-not-exist",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := store.NewMemory()
			// Seed org A with a publisher.
			pubACtx := seedOrg(t, repo, "pub-a", "publisher")
			svc := frames.NewService(repo)

			// Seed org B with its own frame "secret".
			pubBCtx := seedSecondOrg(t, repo, "pub-b", "publisher")
			if _, err := svc.PublishFrame(pubBCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(secretFrameYAML)})); err != nil {
				t.Fatalf("publish org-B secret frame: %v", err)
			}

			// Org-A publisher attempts to publish a frame extending the org-B ref.
			_, err := svc.PublishFrame(pubACtx, connect.NewRequest(&framesv1.PublishFrameRequest{
				Content: childExtending(tc.childRef),
			}))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("want CodeInvalidArgument, got %v (%v)", connect.CodeOf(err), err)
			}
		})
	}
}

func TestListFrameVersions(t *testing.T) {
	const v1Frame = `name: brand-voice
description: OpenTeams brand voice
version: 1.0.0
slots:
  rules:
    - Cite benchmarks.
`
	const v2Frame = `name: brand-voice
description: OpenTeams brand voice
version: 1.1.0
slots:
  rules:
    - Cite benchmarks.
    - Use data.
`
	tests := []struct {
		name      string
		hasRead   bool
		wantCode  connect.Code // 0 means OK
		wantCount int
	}{
		{name: "reader sees versions", hasRead: true, wantCode: 0, wantCount: 2},
		{name: "no read returns not found", hasRead: false, wantCode: connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			pubCtx := seedOrg(t, repo, "pub", "publisher")
			svc := frames.NewService(repo)

			// Publish two versions as the publisher.
			for _, content := range [][]byte{[]byte(v1Frame), []byte(v2Frame)} {
				if _, err := svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: content})); err != nil {
					t.Fatalf("publish: %v", err)
				}
			}

			var callerCtx context.Context
			if tt.hasRead {
				// The publisher already has read via the org-level grant.
				callerCtx = pubCtx
			} else {
				// A user in a different org has no read grant.
				callerCtx = seedSecondOrg(t, repo, "outsider", "admin")
			}

			resp, err := svc.ListFrameVersions(callerCtx, connect.NewRequest(&framesv1.ListFrameVersionsRequest{
				OrgSlug: "openteams", Name: "brand-voice",
			}))
			if tt.wantCode != 0 {
				if connect.CodeOf(err) != tt.wantCode {
					t.Fatalf("code = %v, want %v (err=%v)", connect.CodeOf(err), tt.wantCode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(resp.Msg.Versions) != tt.wantCount {
				t.Errorf("versions = %d, want %d", len(resp.Msg.Versions), tt.wantCount)
			}
		})
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

func TestPublishFrame_ValidationErrorDetail(t *testing.T) {
	// An invalid doc: bad name (uppercase), empty description, empty version.
	const badFrame = `name: Bad_Name
description: ""
version: ""
slots:
  rules:
    - ""
`
	tests := []struct {
		name      string
		wantField string // a field path that MUST appear among the violations
	}{
		{name: "bad name reported", wantField: "name"},
		{name: "empty description reported", wantField: "description"},
		{name: "empty version reported", wantField: "version"},
		{name: "empty rule reported", wantField: "slots.rules[0]"},
	}

	repo := store.NewMemory()
	pubCtx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	_, err := svc.PublishFrame(pubCtx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(badFrame)}))
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("want CodeInvalidArgument, got %v", connect.CodeOf(err))
	}

	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("want *connect.Error, got %T", err)
	}
	// Collect field paths from the FieldViolations detail.
	got := map[string]bool{}
	for _, d := range connErr.Details() {
		msg, verr := d.Value()
		if verr != nil {
			continue
		}
		if fv, ok := msg.(*framesv1.FieldViolations); ok {
			for _, v := range fv.Violations {
				got[v.Field] = true
			}
		}
	}
	if len(got) == 0 {
		t.Fatal("no FieldViolations detail attached to error")
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !got[tc.wantField] {
				t.Fatalf("violation for %q missing; got fields %v", tc.wantField, got)
			}
		})
	}
}
