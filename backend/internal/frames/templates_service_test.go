package frames

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// templateFixture builds a service with two orgs and a caller in each, so
// org-scoping assertions always have a control that must succeed.
type templateFixture struct {
	svc  *Service
	repo *store.Memory
}

func newTemplateFixture(t *testing.T, role string) (templateFixture, context.Context) {
	t.Helper()
	ctx := context.Background()
	repo := store.NewMemory()
	for _, spec := range []struct{ id, slug string }{{"org-a", "acme"}, {"org-b", "other"}} {
		if err := repo.CreateOrg(ctx, &framesv1.Org{
			Id: spec.id, Slug: spec.slug, DisplayName: spec.slug, CreatedAt: timestamppb.Now(),
		}); err != nil {
			t.Fatalf("create org %s: %v", spec.id, err)
		}
	}
	if err := repo.UpsertMembership(ctx, &framesv1.Membership{
		OrgId: "org-a", UserSub: "user-a", Role: role, AddedAt: timestamppb.Now(),
	}); err != nil {
		t.Fatalf("membership: %v", err)
	}
	callerCtx := auth.WithClaims(ctx, &auth.Claims{Subject: "user-a", Email: "a@example.com"})
	return templateFixture{svc: NewService(repo), repo: repo}, callerCtx
}

func seedOrgTemplate(t *testing.T, repo *store.Memory, id, orgID, title string, rules map[string]FieldRule) {
	t.Helper()
	rulesJSON, err := MarshalFieldRules(rules)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	prefill, err := MarshalPrefill(Prefill{Slots: Slots{Style: "Plain sentences."}})
	if err != nil {
		t.Fatalf("marshal prefill: %v", err)
	}
	now := time.Unix(1700000000, 0).UTC()
	if err := repo.CreateFrameTemplate(context.Background(), &store.FrameTemplate{
		ID: id, OrgID: orgID, Title: title, Description: "seeded",
		Prefill: prefill, FieldRules: rulesJSON, CreatedBy: "user-a",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed template %s: %v", id, err)
	}
}

func TestListFrameTemplatesIncludesBuiltinsAndOrgRows(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	seedOrgTemplate(t, f.repo, "t1", "org-a", "Our House Style", nil)
	seedOrgTemplate(t, f.repo, "t2", "org-b", "Not Yours", nil)

	resp, err := f.svc.ListFrameTemplates(ctx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]*framesv1.FrameTemplateSummary{}
	for _, s := range resp.Msg.Templates {
		byID[s.Id] = s
	}
	// Built-ins come first, Blank leading, so the picker is stable.
	if len(resp.Msg.Templates) == 0 || resp.Msg.Templates[0].Id != BlankTemplateID {
		t.Fatalf("first entry = %v, want %q", resp.Msg.Templates, BlankTemplateID)
	}
	if !byID[BlankTemplateID].Builtin {
		t.Error("blank is not flagged builtin")
	}
	own, ok := byID["t1"]
	if !ok {
		t.Fatal("the caller's own org template is missing")
	}
	if own.Builtin {
		t.Error("an org template is flagged builtin")
	}
	if own.Title != "Our House Style" {
		t.Errorf("title = %q", own.Title)
	}
	// The other org's row must not appear. This is the assertion that matters;
	// the presence checks above are its control.
	if _, leaked := byID["t2"]; leaked {
		t.Error("another org's template appeared in the list")
	}
	if !resp.Msg.CanManage {
		t.Error("can_manage = false for an admin")
	}
}

func TestListFrameTemplatesCanManageFollowsRole(t *testing.T) {
	tests := []struct {
		role          string
		wantCanManage bool
	}{
		{role: "admin", wantCanManage: true},
		{role: "publisher", wantCanManage: false},
		{role: "viewer", wantCanManage: false},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			f, ctx := newTemplateFixture(t, tt.role)
			resp, err := f.svc.ListFrameTemplates(ctx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
			if err != nil {
				t.Fatalf("list as %s: %v", tt.role, err)
			}
			if resp.Msg.CanManage != tt.wantCanManage {
				t.Errorf("can_manage = %v, want %v", resp.Msg.CanManage, tt.wantCanManage)
			}
			// Every role can still list: templates are readable to any member.
			if len(resp.Msg.Templates) == 0 {
				t.Error("no templates returned")
			}
		})
	}
}

func TestGetFrameTemplate(t *testing.T) {
	f, ctx := newTemplateFixture(t, "publisher")
	seedOrgTemplate(t, f.repo, "t1", "org-a", "Our House Style", map[string]FieldRule{
		"style": {Level: RequirementRequired, Note: "Voice and tone."},
	})
	seedOrgTemplate(t, f.repo, "t2", "org-b", "Not Yours", nil)

	tests := []struct {
		name     string
		id       string
		wantCode connect.Code
		wantRule Requirement
		wantNote string
	}{
		{name: "own org template", id: "t1", wantRule: RequirementRequired, wantNote: "Voice and tone."},
		{name: "builtin", id: "builtin:domain-vocabulary", wantRule: RequirementRequired},
		{name: "another org's template is not found", id: "t2", wantCode: connect.CodeNotFound},
		{name: "missing builtin is not found", id: "builtin:nope", wantCode: connect.CodeNotFound},
		{name: "unknown id is not found", id: "01J000000000000000000000AB", wantCode: connect.CodeNotFound},
		{name: "empty id is invalid", id: "", wantCode: connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := f.svc.GetFrameTemplate(ctx, connect.NewRequest(&framesv1.GetFrameTemplateRequest{Id: tt.id}))
			if tt.wantCode != 0 {
				if connect.CodeOf(err) != tt.wantCode {
					t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := resp.Msg.Template
			if got.Id != tt.id {
				t.Errorf("id = %q, want %q", got.Id, tt.id)
			}
			if got.Builtin != IsBuiltin(tt.id) {
				t.Errorf("builtin = %v, want %v", got.Builtin, IsBuiltin(tt.id))
			}
			if len(got.FieldRules) == 0 {
				t.Fatal("no field rules returned")
			}
			if tt.wantNote != "" {
				if got.FieldRules["style"].Note != tt.wantNote {
					t.Errorf("note = %q, want %q", got.FieldRules["style"].Note, tt.wantNote)
				}
			}
			// Prefill must come back as something ParsePrefill accepts, or the
			// clients cannot seed a form from it.
			if _, err := ParsePrefill(got.Prefill); err != nil {
				t.Errorf("prefill is not parseable: %v\n%s", err, got.Prefill)
			}
		})
	}
}

func TestFieldRulesJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		rules map[string]FieldRule
	}{
		{name: "nil", rules: nil},
		{name: "empty", rules: map[string]FieldRule{}},
		{
			name: "every level",
			rules: map[string]FieldRule{
				"style":       {Level: RequirementRequired, Note: "Voice."},
				"rules":       {Level: RequirementRecommended, Note: "Constraints."},
				"terminology": {Level: RequirementOptional},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := MarshalFieldRules(tt.rules)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got, err := ParseFieldRules(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(got) != len(tt.rules) {
				t.Fatalf("got %d rules, want %d (%s)", len(got), len(tt.rules), b)
			}
			for key, want := range tt.rules {
				if got[key] != want {
					t.Errorf("rule %q = %+v, want %+v", key, got[key], want)
				}
			}
		})
	}
}

func TestFieldRulesJSONStoresLevelNames(t *testing.T) {
	// Stored by name, never by the enum's integer: reordering the constants
	// must not silently reinterpret every row already on disk.
	b, err := MarshalFieldRules(map[string]FieldRule{"style": {Level: RequirementRequired}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"level":"required"`; !strings.Contains(string(b), want) {
		t.Errorf("encoded as %s, want it to contain %s", b, want)
	}
}

func TestParseFieldRulesRejectsUnknownLevel(t *testing.T) {
	_, err := ParseFieldRules([]byte(`{"style":{"level":"mandatory"}}`))
	if err == nil {
		t.Fatal("an unknown level was accepted")
	}
	// Naming the offending value matters: this error reaches an admin editing a
	// template, and "invalid input" would not tell them which level to fix.
	if !strings.Contains(err.Error(), "unknown requirement level") {
		t.Errorf("err = %v, want it to name the unknown level", err)
	}
	if !strings.Contains(err.Error(), "style") {
		t.Errorf("err = %v, want it to name the slot", err)
	}
}

func TestCreateFrameTemplate(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	prefill, err := MarshalPrefill(Prefill{Slots: Slots{Style: "Plain sentences."}})
	if err != nil {
		t.Fatalf("marshal prefill: %v", err)
	}
	resp, err := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(&framesv1.CreateFrameTemplateRequest{
		Title: "Our House Style", Description: "How we sound", Prefill: prefill,
		FieldRules: map[string]*framesv1.FieldRule{
			"style": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED, Note: "Voice and tone."},
		},
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := resp.Msg.Template
	if got.Id == "" || IsBuiltin(got.Id) {
		t.Errorf("id = %q, want a non-builtin id", got.Id)
	}
	if got.Builtin {
		t.Error("a created template is flagged builtin")
	}
	// It has to come back from a fresh read, not just from the create response.
	fetched, err := f.svc.GetFrameTemplate(ctx, connect.NewRequest(&framesv1.GetFrameTemplateRequest{Id: got.Id}))
	if err != nil {
		t.Fatalf("get after create: %v", err)
	}
	if fetched.Msg.Template.FieldRules["style"].Level != framesv1.Requirement_REQUIREMENT_REQUIRED {
		t.Errorf("stored level = %v", fetched.Msg.Template.FieldRules["style"].Level)
	}
	if fetched.Msg.Template.FieldRules["style"].Note != "Voice and tone." {
		t.Errorf("stored note = %q", fetched.Msg.Template.FieldRules["style"].Note)
	}
}

func TestCreateFrameTemplateValidation(t *testing.T) {
	goodPrefill, err := MarshalPrefill(Prefill{Slots: Slots{Style: "x"}})
	if err != nil {
		t.Fatalf("marshal prefill: %v", err)
	}
	tests := []struct {
		name     string
		req      *framesv1.CreateFrameTemplateRequest
		wantCode connect.Code
		wantMsg  string
	}{
		{
			name: "valid",
			req:  &framesv1.CreateFrameTemplateRequest{Title: "T", Description: "D", Prefill: goodPrefill},
		},
		{
			name:     "title is required",
			req:      &framesv1.CreateFrameTemplateRequest{Description: "D", Prefill: goodPrefill},
			wantCode: connect.CodeInvalidArgument, wantMsg: "title",
		},
		{
			name:     "description is required",
			req:      &framesv1.CreateFrameTemplateRequest{Title: "T", Prefill: goodPrefill},
			wantCode: connect.CodeInvalidArgument, wantMsg: "description",
		},
		{
			name: "prefill must not carry identity",
			req: &framesv1.CreateFrameTemplateRequest{
				Title: "T", Description: "D", Prefill: []byte("name: sneaky\nslots: {}\n"),
			},
			wantCode: connect.CodeInvalidArgument, wantMsg: "must not set name",
		},
		{
			name: "an unknown slot in field_rules is refused",
			req: &framesv1.CreateFrameTemplateRequest{
				Title: "T", Description: "D", Prefill: goodPrefill,
				FieldRules: map[string]*framesv1.FieldRule{"nosuchslot": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED}},
			},
			wantCode: connect.CodeInvalidArgument, wantMsg: "unknown slot",
		},
		{
			name: "empty prefill is accepted and means no seeded content",
			req:  &framesv1.CreateFrameTemplateRequest{Title: "T", Description: "D"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ctx := newTemplateFixture(t, "admin")
			_, err := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(tt.req))
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if connect.CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tt.wantCode)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %v, want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestCreateFrameTemplateDuplicateTitle(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	req := &framesv1.CreateFrameTemplateRequest{Title: "Only One", Description: "D"}
	if _, err := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(req)); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("code = %v (err %v), want AlreadyExists", connect.CodeOf(err), err)
	}
}

func TestTemplateWritesAreAdminOnly(t *testing.T) {
	roles := []struct {
		role      string
		wantAllow bool
	}{
		{role: "admin", wantAllow: true},
		{role: "publisher", wantAllow: false},
		{role: "viewer", wantAllow: false},
	}
	for _, tt := range roles {
		t.Run(tt.role, func(t *testing.T) {
			f, ctx := newTemplateFixture(t, tt.role)
			seedOrgTemplate(t, f.repo, "t1", "org-a", "Existing", nil)

			_, createErr := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(
				&framesv1.CreateFrameTemplateRequest{Title: "New", Description: "D"}))
			_, updateErr := f.svc.UpdateFrameTemplate(ctx, connect.NewRequest(
				&framesv1.UpdateFrameTemplateRequest{Id: "t1", Title: "Renamed", Description: "D"}))
			_, deleteErr := f.svc.DeleteFrameTemplate(ctx, connect.NewRequest(
				&framesv1.DeleteFrameTemplateRequest{Id: "t1"}))

			for name, err := range map[string]error{"create": createErr, "update": updateErr, "delete": deleteErr} {
				if tt.wantAllow {
					if err != nil {
						t.Errorf("%s as %s = %v, want success", name, tt.role, err)
					}
					continue
				}
				if connect.CodeOf(err) != connect.CodePermissionDenied {
					t.Errorf("%s as %s: code = %v (err %v), want PermissionDenied", name, tt.role, connect.CodeOf(err), err)
				}
			}
			// The control: a non-admin can still read, or the assertions above
			// would pass for a caller who simply has no access at all.
			if _, err := f.svc.ListFrameTemplates(ctx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{})); err != nil {
				t.Errorf("list as %s = %v, want success", tt.role, err)
			}
		})
	}
}

func TestBuiltinTemplatesAreImmutable(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	_, updateErr := f.svc.UpdateFrameTemplate(ctx, connect.NewRequest(&framesv1.UpdateFrameTemplateRequest{
		Id: BlankTemplateID, Title: "Hijacked", Description: "D",
	}))
	if connect.CodeOf(updateErr) != connect.CodeInvalidArgument {
		t.Errorf("update builtin: code = %v (err %v), want InvalidArgument", connect.CodeOf(updateErr), updateErr)
	}
	_, deleteErr := f.svc.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{
		Id: BlankTemplateID,
	}))
	if connect.CodeOf(deleteErr) != connect.CodeInvalidArgument {
		t.Errorf("delete builtin: code = %v (err %v), want InvalidArgument", connect.CodeOf(deleteErr), deleteErr)
	}
	// The control: the same admin can mutate one of their own.
	seedOrgTemplate(t, f.repo, "t1", "org-a", "Mine", nil)
	if _, err := f.svc.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{Id: "t1"})); err != nil {
		t.Errorf("delete own template = %v, want success", err)
	}
	// And the built-in is still there afterwards.
	if _, ok := BuiltinTemplate(BlankTemplateID); !ok {
		t.Error("the built-in vanished")
	}
}

func TestUpdateAndDeleteAreOrgScoped(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	seedOrgTemplate(t, f.repo, "theirs", "org-b", "Theirs", nil)
	_, updateErr := f.svc.UpdateFrameTemplate(ctx, connect.NewRequest(&framesv1.UpdateFrameTemplateRequest{
		Id: "theirs", Title: "Stolen", Description: "D",
	}))
	if connect.CodeOf(updateErr) != connect.CodeNotFound {
		t.Errorf("update across orgs: code = %v (err %v), want NotFound", connect.CodeOf(updateErr), updateErr)
	}
	_, deleteErr := f.svc.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{Id: "theirs"}))
	if connect.CodeOf(deleteErr) != connect.CodeNotFound {
		t.Errorf("delete across orgs: code = %v (err %v), want NotFound", connect.CodeOf(deleteErr), deleteErr)
	}
	// The other org's row survived.
	if _, err := f.repo.GetFrameTemplate(context.Background(), "org-b", "theirs"); err != nil {
		t.Errorf("the other org's template was affected: %v", err)
	}
}

func TestOrgTemplateIsVisibleToAnotherMember(t *testing.T) {
	// Journey 5's "persist and are usable by org members" half. A second member
	// of the same org, not the author, must see it.
	f, ctx := newTemplateFixture(t, "admin")
	created, err := f.svc.CreateFrameTemplate(ctx, connect.NewRequest(&framesv1.CreateFrameTemplateRequest{
		Title: "Shared Standard", Description: "D",
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.repo.UpsertMembership(context.Background(), &framesv1.Membership{
		OrgId: "org-a", UserSub: "user-second", Role: "viewer", AddedAt: timestamppb.Now(),
	}); err != nil {
		t.Fatalf("second membership: %v", err)
	}
	otherCtx := auth.WithClaims(context.Background(), &auth.Claims{Subject: "user-second", Email: "s@example.com"})
	resp, err := f.svc.ListFrameTemplates(otherCtx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
	if err != nil {
		t.Fatalf("list as the second member: %v", err)
	}
	found := false
	for _, s := range resp.Msg.Templates {
		if s.Id == created.Msg.Template.Id {
			found = true
		}
	}
	if !found {
		t.Error("a second member of the org cannot see the template")
	}
}
