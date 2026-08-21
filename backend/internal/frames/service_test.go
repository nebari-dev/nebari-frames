package frames_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

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

// seedReadableFrameDirect seeds a frame that org member "v" (viewer in org o1) can read.
// It uses a direct store call so we don't depend on PublishFrame's side-effects.
func seedReadableFrameDirect(t *testing.T, repo *store.Memory, ctx context.Context) {
	t.Helper()
	err := repo.CreateFrameVersion(ctx, store.CreateFrameVersionInput{
		Frame: &framesv1.Frame{
			Id: "f-alpha", OrgId: "o1", Name: "alpha", Description: "A",
			OwnerSub: "pub", LatestVersion: "1.0.0",
			CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now(),
		},
		Version: &framesv1.FrameVersion{
			Version:     "1.0.0",
			Content:     []byte("name: alpha\ndescription: A\nversion: 1.0.0\nslots:\n  rules:\n    - r1\n"),
			PublishedAt: timestamppb.Now(),
		},
		Grants:     []store.Grant{{SubjectType: "org", SubjectID: "o1", Permission: "read"}},
		IsNewFrame: true,
	})
	if err != nil {
		t.Fatalf("seedReadableFrameDirect: %v", err)
	}
}

// seedUnreadableFrameDirect seeds a frame that org member "v" cannot read.
func seedUnreadableFrameDirect(t *testing.T, repo *store.Memory, ctx context.Context) {
	t.Helper()
	err := repo.CreateFrameVersion(ctx, store.CreateFrameVersionInput{
		Frame: &framesv1.Frame{
			Id: "f-secret", OrgId: "o1", Name: "secret", Description: "S",
			OwnerSub: "someone-else", LatestVersion: "1.0.0",
			CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now(),
		},
		Version: &framesv1.FrameVersion{
			Version:     "1.0.0",
			Content:     []byte("name: secret\ndescription: S\nversion: 1.0.0\nslots:\n  rules:\n    - hidden\n"),
			PublishedAt: timestamppb.Now(),
		},
		Grants:     []store.Grant{{SubjectType: "user", SubjectID: "someone-else", Permission: "read"}},
		IsNewFrame: true,
	})
	if err != nil {
		t.Fatalf("seedUnreadableFrameDirect: %v", err)
	}
}

func TestListReadable_FiltersByRBAC(t *testing.T) {
	repo := store.NewMemory()
	// Viewer in org o1 - not admin, so RBAC filtering actually applies.
	ctx := seedOrg(t, repo, "v", "viewer")
	svc := frames.NewService(repo)

	// Seed both frames directly so grants are controlled precisely.
	seedReadableFrameDirect(t, repo, context.Background())
	seedUnreadableFrameDirect(t, repo, context.Background())

	got, err := svc.ListReadable(ctx)
	if err != nil {
		t.Fatalf("ListReadable: %v", err)
	}
	names := map[string]bool{}
	for _, f := range got {
		names[f.Name] = true
	}
	if !names["alpha"] {
		t.Error("expected readable frame 'alpha' in list")
	}
	if names["secret"] {
		t.Error("unreadable frame 'secret' must not be listed (no existence leak)")
	}
	for _, f := range got {
		if f.OrgSlug == "" || f.OrgDisplay == "" || f.Version == "" {
			t.Errorf("ReadableFrame missing fields: %+v", f)
		}
	}
}

func TestResolveDoc_DeniedReadIsNotFound(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "v", "viewer")
	svc := frames.NewService(repo)

	seedUnreadableFrameDirect(t, repo, context.Background())

	_, err := svc.ResolveDoc(ctx, "openteams", "secret", "")
	if err == nil {
		t.Fatal("expected not-found error for unreadable frame")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("want CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestConvertFrame_YamlToMarkdown(t *testing.T) {
	const doc = `name: brand-voice
description: Voice guardrails.
version: 1.0.0
visibility: internal
slots:
  rules:
    - Cite benchmarks.
`
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	resp, err := svc.ConvertFrame(ctx, connect.NewRequest(&framesv1.ConvertFrameRequest{
		Source: &framesv1.ConvertFrameRequest_Yaml{Yaml: []byte(doc)},
	}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := string(resp.Msg.Markdown)
	for _, want := range []string{"type: frame [0.2]", "## Rules", "- Cite benchmarks."} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown missing %q:\n%s", want, got)
		}
	}
}

func TestConvertFrame_MarkdownToYaml(t *testing.T) {
	const md = `---
type: frame [0.2]
name: brand-voice
description: Voice guardrails.
visibility: internal
version: 1.0.0
inherits: openteams/company-core@1.2.0
---

## Rules

- Cite benchmarks.
`
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	resp, err := svc.ConvertFrame(ctx, connect.NewRequest(&framesv1.ConvertFrameRequest{
		Source: &framesv1.ConvertFrameRequest_Markdown{Markdown: []byte(md)},
	}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	parsed, err := frames.Parse(resp.Msg.Yaml)
	if err != nil {
		t.Fatalf("converted yaml does not parse: %v", err)
	}
	if parsed.Visibility != "internal" || parsed.Name != "brand-voice" {
		t.Errorf("metadata lost: %+v", parsed)
	}
	if len(parsed.Extends) != 1 || parsed.Extends[0].Version != "1.2.0" {
		t.Errorf("inherits not converted: %+v", parsed.Extends)
	}
	if err := frames.Validate(parsed); err != nil {
		t.Errorf("converted doc should validate: %v", err)
	}
}

// A structural markdown error must arrive as FieldViolations on "markdown" so
// the editor can show it against the source, the same way publish errors map to
// their inputs.
func TestConvertFrame_StructuralErrorDetail(t *testing.T) {
	const md = `---
type: frame [0.2]
name: c
description: d
visibility: internal
---

## Ways of Working

- something
`
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	_, err := svc.ConvertFrame(ctx, connect.NewRequest(&framesv1.ConvertFrameRequest{
		Source: &framesv1.ConvertFrameRequest_Markdown{Markdown: []byte(md)},
	}))
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
	found := false
	for _, d := range connErr.Details() {
		msg, verr := d.Value()
		if verr != nil {
			continue
		}
		fv, ok := msg.(*framesv1.FieldViolations)
		if !ok {
			continue
		}
		for _, v := range fv.Violations {
			if v.Field == "markdown" && strings.Contains(v.Message, "did you mean \"## Norms\"?") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a markdown FieldViolation naming the suggestion, got %v", connErr.Details())
	}
}

// Conversion is stateless but still org-scoped: a caller with no membership is
// rejected before any parsing happens.
func TestConvertFrame_RequiresMembership(t *testing.T) {
	repo := store.NewMemory()
	svc := frames.NewService(repo)
	_, err := svc.ConvertFrame(context.Background(), connect.NewRequest(&framesv1.ConvertFrameRequest{
		Source: &framesv1.ConvertFrameRequest_Yaml{Yaml: []byte("name: c\ndescription: d\nversion: 1.0.0\nslots: {}\n")},
	}))
	if err == nil {
		t.Fatal("want error for a caller with no org membership")
	}
}

// docFor builds a minimal valid Doc for PublishDoc tests.
func docFor(name, version string, rules ...string) *frames.Doc {
	return &frames.Doc{
		Name:        name,
		Description: name + " description",
		Version:     version,
		Slots:       frames.Slots{Rules: rules},
	}
}

func TestService_PublishDoc(t *testing.T) {
	tests := []struct {
		name string
		// role of the calling user in org o1
		role string
		// seed publishes brand-voice@1.0.0 as "owner" first
		seedExisting bool
		docName      string
		version      string
		intent       frames.PublishIntent
		wantCode     connect.Code // 0 means success
	}{
		{
			name: "publisher creates a new frame", role: "publisher",
			docName: "brand-voice", version: "1.0.0", intent: frames.PublishCreate,
		},
		{
			name: "viewer cannot create", role: "viewer",
			docName: "brand-voice", version: "1.0.0", intent: frames.PublishCreate,
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "admin creates a new frame", role: "admin",
			docName: "brand-voice", version: "1.0.0", intent: frames.PublishCreate,
		},
		{
			name: "create refuses a name that already exists", role: "admin",
			seedExisting: true,
			docName:      "brand-voice", version: "2.0.0", intent: frames.PublishCreate,
			wantCode: connect.CodeAlreadyExists,
		},
		{
			name: "update requires the frame to exist", role: "admin",
			docName: "brand-voice", version: "1.0.0", intent: frames.PublishUpdate,
			wantCode: connect.CodeNotFound,
		},
		{
			name: "admin updates an existing frame", role: "admin",
			seedExisting: true,
			docName:      "brand-voice", version: "2.0.0", intent: frames.PublishUpdate,
		},
		{
			name: "upsert creates when absent, preserving the RPC's behavior", role: "publisher",
			docName: "brand-voice", version: "1.0.0", intent: frames.PublishUpsert,
		},
		{
			name: "upsert updates when present", role: "admin",
			seedExisting: true,
			docName:      "brand-voice", version: "2.0.0", intent: frames.PublishUpsert,
		},
		{
			name: "a republished version is rejected", role: "admin",
			seedExisting: true,
			docName:      "brand-voice", version: "1.0.0", intent: frames.PublishUpdate,
			wantCode: connect.CodeAlreadyExists,
		},
		{
			name: "an invalid document is rejected before any write", role: "admin",
			docName: "Not A Valid Name", version: "1.0.0", intent: frames.PublishCreate,
			wantCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			ctx := seedOrg(t, repo, "caller", tt.role)
			svc := frames.NewService(repo)

			if tt.seedExisting {
				ownerCtx := auth.WithClaims(context.Background(), &auth.Claims{Subject: "owner", Email: "owner@x"})
				_ = repo.UpsertMembership(context.Background(), &framesv1.Membership{OrgId: "o1", UserSub: "owner", Role: "publisher"})
				if _, _, err := svc.PublishDoc(ownerCtx, docFor("brand-voice", "1.0.0", "seeded"), "seed", frames.PublishCreate); err != nil {
					t.Fatalf("seed publish: %v", err)
				}
			}

			frame, version, err := svc.PublishDoc(ctx, docFor(tt.docName, tt.version, "a rule"), "changelog", tt.intent)
			if tt.wantCode != 0 {
				if connect.CodeOf(err) != tt.wantCode {
					t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if frame.Name != tt.docName {
				t.Errorf("frame name = %q, want %q", frame.Name, tt.docName)
			}
			if version.Version != tt.version {
				t.Errorf("version = %q, want %q", version.Version, tt.version)
			}
			if frame.LatestVersion != tt.version {
				t.Errorf("latest version = %q, want %q", frame.LatestVersion, tt.version)
			}
		})
	}
}

// A publisher who is not the owner and holds no edit grant must not be able to
// overwrite someone else's frame, whichever intent they pass.
func TestService_PublishDocDoesNotBypassEditPermission(t *testing.T) {
	for _, intent := range []frames.PublishIntent{frames.PublishUpsert, frames.PublishUpdate, frames.PublishCreate} {
		repo := store.NewMemory()
		ownerCtx := seedOrg(t, repo, "owner", "publisher")
		svc := frames.NewService(repo)
		if _, _, err := svc.PublishDoc(ownerCtx, docFor("brand-voice", "1.0.0", "owned"), "", frames.PublishCreate); err != nil {
			t.Fatalf("seed publish: %v", err)
		}

		ctx := context.Background()
		_ = repo.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "other", Role: "publisher"})
		otherCtx := auth.WithClaims(ctx, &auth.Claims{Subject: "other", Email: "other@x"})

		_, _, err := svc.PublishDoc(otherCtx, docFor("brand-voice", "9.9.9", "hijacked"), "", intent)
		if err == nil {
			t.Fatalf("intent %v: a non-owner publisher overwrote a frame they cannot edit", intent)
		}
		code := connect.CodeOf(err)
		if code != connect.CodePermissionDenied && code != connect.CodeAlreadyExists {
			t.Errorf("intent %v: code = %v, want PermissionDenied or AlreadyExists", intent, code)
		}
	}
}

// The Connect path must store the exact bytes the client submitted: normalizing
// them through a Doc round trip would strip comments and change the digest of a
// logically identical document.
func TestService_PublishFramePreservesSubmittedBytes(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	content := []byte("# a comment the author cares about\n" + sampleFrame)
	if _, err := svc.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: content})); err != nil {
		t.Fatalf("publish: %v", err)
	}

	resp, err := svc.GetFrame(ctx, connect.NewRequest(&framesv1.GetFrameRequest{OrgSlug: "openteams", Name: "brand-voice"}))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(resp.Msg.Version.Content, content) {
		t.Errorf("stored content was rewritten:\n got: %q\nwant: %q", resp.Msg.Version.Content, content)
	}
}

// Authorization must precede parsing: a caller who may not publish should be
// denied without the server first parsing content they supplied. Malformed YAML
// from a viewer therefore surfaces PermissionDenied, not InvalidArgument. The
// ordering predates the create/update split; this pins it so extracting the
// shared publish path cannot quietly invert it.
func TestService_PublishFrameAuthorizesBeforeParsing(t *testing.T) {
	repo := store.NewMemory()
	viewerCtx := seedOrg(t, repo, "viewer-user", "viewer")
	svc := frames.NewService(repo)

	_, err := svc.PublishFrame(viewerCtx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content: []byte("this: is: not: valid: yaml: at: all"),
	}))
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("code = %v (err %v), want PermissionDenied: parsing ran before the role check", got, err)
	}
}

// SourceDoc returns a frame's own stored document, NOT the inheritance-resolved
// one. A write path that fed a resolved doc back in would flatten the parent's
// content into the child and drop the extends edges, so this distinction is the
// difference between a safe update and silent inheritance loss.
func TestService_SourceDocIsUnresolved(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)

	parent := `name: base
description: Base
version: 1.0.0
slots:
  rules:
    - from parent
`
	child := `name: child
description: Child
version: 1.0.0
visibility: private
scope: company
maintainer: platform team
extends:
  - ref: openteams/base
    version: 1.0.0
slots:
  rules:
    - from child
`
	for _, content := range []string{parent, child} {
		if _, err := svc.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(content)})); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	src, err := svc.SourceDoc(ctx, "child", "")
	if err != nil {
		t.Fatalf("SourceDoc: %v", err)
	}
	if got := src.Slots.Rules; len(got) != 1 || got[0] != "from child" {
		t.Errorf("rules = %v, want only the child's own rule (parent content must not be merged in)", got)
	}
	if len(src.Extends) != 1 || src.Extends[0].Ref != "openteams/base" {
		t.Errorf("extends = %+v, want the child's own pinned parent", src.Extends)
	}
	if src.Visibility != "private" || src.Scope != "company" || src.Maintainer != "platform team" {
		t.Errorf("metadata lost: visibility=%q scope=%q maintainer=%q", src.Visibility, src.Scope, src.Maintainer)
	}

	// Contrast: ResolveDoc merges the parent in and is therefore unsafe to
	// round-trip back into a write.
	resolved, err := svc.ResolveDoc(ctx, "openteams", "child", "")
	if err != nil {
		t.Fatalf("ResolveDoc: %v", err)
	}
	if len(resolved.Slots.Rules) != 2 {
		t.Errorf("resolved rules = %v, want both parent and child rules", resolved.Slots.Rules)
	}
}

func TestService_SourceDocRespectsRead(t *testing.T) {
	repo := store.NewMemory()
	ownerCtx := seedOrg(t, repo, "owner", "publisher")
	svc := frames.NewService(repo)
	if _, _, err := svc.PublishDoc(ownerCtx, docFor("private-frame", "1.0.0", "secret"), "", frames.PublishCreate); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A member of a different org must not read it, and must not learn it exists.
	otherCtx := seedSecondOrg(t, repo, "outsider", "admin")
	if _, err := svc.SourceDoc(otherCtx, "private-frame", ""); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("code = %v, want NotFound for a cross-org read", connect.CodeOf(err))
	}
}

// An LLM-driven write path can emit arbitrarily large content, so the cap the
// design doc promises has to be real - and enforced for both front doors.
func TestService_PublishRejectsOversizedContent(t *testing.T) {
	huge := strings.Repeat("x", frames.MaxContentBytes+1)

	t.Run("connect path", func(t *testing.T) {
		repo := store.NewMemory()
		ctx := seedOrg(t, repo, "pub", "publisher")
		svc := frames.NewService(repo)
		content := "name: big\ndescription: d\nversion: 1.0.0\nslots:\n  goals: " + huge + "\n"
		_, err := svc.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{Content: []byte(content)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("code = %v (err %v), want InvalidArgument", connect.CodeOf(err), err)
		}
	})

	t.Run("publish doc path", func(t *testing.T) {
		repo := store.NewMemory()
		ctx := seedOrg(t, repo, "pub", "publisher")
		svc := frames.NewService(repo)
		doc := docFor("big", "1.0.0")
		doc.Slots.Goals = huge
		_, _, err := svc.PublishDoc(ctx, doc, "", frames.PublishCreate)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("code = %v (err %v), want InvalidArgument", connect.CodeOf(err), err)
		}
	})

	// Pins the boundary exactly: content of precisely MaxContentBytes is allowed
	// and one byte more is not, so the comparison cannot drift between > and >=.
	t.Run("the boundary is inclusive", func(t *testing.T) {
		// Binary-search the padding that makes the marshalled document land on
		// exactly the limit; YAML framing makes the offset awkward to hardcode.
		sizeFor := func(pad int) int {
			d := docFor("ok", "1.0.0")
			d.Slots.Goals = strings.Repeat("y", pad)
			b, err := frames.Marshal(d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			return len(b)
		}
		lo, hi := 0, frames.MaxContentBytes
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if sizeFor(mid) <= frames.MaxContentBytes {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		if got := sizeFor(lo); got != frames.MaxContentBytes {
			t.Fatalf("could not construct content of exactly %d bytes (closest %d)", frames.MaxContentBytes, got)
		}

		atLimit := docFor("ok", "1.0.0")
		atLimit.Slots.Goals = strings.Repeat("y", lo)
		repo := store.NewMemory()
		ctx := seedOrg(t, repo, "pub", "publisher")
		if _, _, err := frames.NewService(repo).PublishDoc(ctx, atLimit, "", frames.PublishCreate); err != nil {
			t.Errorf("content of exactly %d bytes was rejected: %v", frames.MaxContentBytes, err)
		}

		over := docFor("ok", "1.0.0")
		over.Slots.Goals = strings.Repeat("y", lo+1)
		repo2 := store.NewMemory()
		ctx2 := seedOrg(t, repo2, "pub", "publisher")
		_, _, err := frames.NewService(repo2).PublishDoc(ctx2, over, "", frames.PublishCreate)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("one byte over the limit: code = %v (err %v), want InvalidArgument", connect.CodeOf(err), err)
		}
	})
}

// latest_version must not move backwards. Publishing an older version would
// otherwise make every default read - GetFrame, ListFrames, MCP get_frame, and
// the merge base of the next update - resolve to the older document, quietly
// unpublishing newer content.
func TestService_PublishRejectsNonAdvancingVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string // published in order; the last one is the assertion
		wantCode connect.Code
	}{
		{name: "advancing patch", versions: []string{"1.0.0", "1.0.1"}},
		{name: "advancing minor", versions: []string{"1.0.0", "1.1.0"}},
		{name: "advancing major", versions: []string{"1.9.9", "2.0.0"}},
		{name: "double digits sort numerically", versions: []string{"1.9.0", "1.10.0"}},
		{name: "going backwards is rejected", versions: []string{"2.0.0", "1.0.1"}, wantCode: connect.CodeInvalidArgument},
		{name: "minor going backwards is rejected", versions: []string{"1.2.0", "1.1.9"}, wantCode: connect.CodeInvalidArgument},
		{name: "republishing the same version is rejected", versions: []string{"1.0.0", "1.0.0"}, wantCode: connect.CodeAlreadyExists},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := store.NewMemory()
			ctx := seedOrg(t, repo, "pub", "publisher")
			svc := frames.NewService(repo)
			var err error
			for i, v := range tt.versions {
				intent := frames.PublishUpdate
				if i == 0 {
					intent = frames.PublishCreate
				}
				_, _, err = svc.PublishDoc(ctx, docFor("brand-voice", v, "r"), "", intent)
				if i < len(tt.versions)-1 && err != nil {
					t.Fatalf("seeding %s: %v", v, err)
				}
			}
			if got := connect.CodeOf(err); tt.wantCode == 0 && err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else if tt.wantCode != 0 && got != tt.wantCode {
				t.Fatalf("code = %v (err %v), want %v", got, err, tt.wantCode)
			}
		})
	}
}

// Two callers that both read version 1.0.0 and then publish must not silently
// lose one another's changes. The second publish is rejected because the Frame
// moved on beneath it.
func TestService_PublishDetectsAStaleBase(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)
	if _, _, err := svc.PublishDoc(ctx, docFor("brand-voice", "1.0.0", "original"), "", frames.PublishCreate); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Both callers read 1.0.0 as their base.
	first := docFor("brand-voice", "1.1.0", "original", "from the first caller")
	second := docFor("brand-voice", "1.2.0", "original", "from the second caller")

	if _, _, err := svc.PublishDocFrom(ctx, first, "", frames.PublishUpdate, "1.0.0"); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	_, _, err := svc.PublishDocFrom(ctx, second, "", frames.PublishUpdate, "1.0.0")
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition: the second caller's base was stale",
			connect.CodeOf(err), err)
	}

	// The first caller's change survived.
	doc, err := svc.SourceDoc(ctx, "brand-voice", "")
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if doc.Version != "1.1.0" {
		t.Errorf("latest = %q, want 1.1.0", doc.Version)
	}
}

// An empty base version means "I did not check", which keeps the Connect API's
// existing behaviour rather than forcing every caller to supply one.
func TestService_PublishWithoutABaseVersionIsUnchecked(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)
	if _, _, err := svc.PublishDoc(ctx, docFor("brand-voice", "1.0.0", "a"), "", frames.PublishCreate); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := svc.PublishDoc(ctx, docFor("brand-voice", "1.1.0", "b"), "", frames.PublishUpdate); err != nil {
		t.Errorf("unchecked publish should succeed: %v", err)
	}
}

// The version error is reported as a field violation so the web form can mark
// the version input, the way it already does for a duplicate version.
func TestService_NonAdvancingVersionIsAFieldViolation(t *testing.T) {
	repo := store.NewMemory()
	ctx := seedOrg(t, repo, "pub", "publisher")
	svc := frames.NewService(repo)
	if _, _, err := svc.PublishDoc(ctx, docFor("brand-voice", "2.0.0", "a"), "", frames.PublishCreate); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, _, err := svc.PublishDoc(ctx, docFor("brand-voice", "1.0.0", "b"), "", frames.PublishUpdate)
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("want a connect error, got %v", err)
	}
	found := false
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if fv, ok := v.(*framesv1.FieldViolations); ok {
			for _, viol := range fv.Violations {
				if viol.Field == "version" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("no field violation on 'version'; details = %v", ce.Details())
	}
}
