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
