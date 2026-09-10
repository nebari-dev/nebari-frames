//go:build e2e

package e2e

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

// TestAuthIsEnforced is the guard against a vacuous suite. Everything below
// asserts what an authenticated caller can do; without this, a deployment that
// accepted anonymous requests would make all of it pass for the wrong reason.
func TestAuthIsEnforced(t *testing.T) {
	ctx := context.Background()
	anon := newAnonClient(t)
	_, err := anon.ListFrameTemplates(ctx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
	if err == nil {
		t.Fatal("an unauthenticated caller listed templates")
	}
	if code := connect.CodeOf(err); code != connect.CodeUnauthenticated {
		t.Fatalf("code = %v (err %v), want Unauthenticated", code, err)
	}
}

func TestBuiltinTemplatesAreServed(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	resp, err := client.ListFrameTemplates(ctx, connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]*framesv1.FrameTemplateSummary{}
	for _, s := range resp.Msg.Templates {
		byID[s.Id] = s
	}
	for _, want := range []string{
		"builtin:blank", "builtin:brand-voice", "builtin:business-process",
		"builtin:domain-vocabulary", "builtin:engineering-norms", "builtin:product-context",
	} {
		got, ok := byID[want]
		if !ok {
			t.Errorf("built-in %q is missing from a real deployment", want)
			continue
		}
		if !got.Builtin {
			t.Errorf("%q is not flagged builtin", want)
		}
		if got.Title == "" || got.Description == "" {
			t.Errorf("%q has an empty title or description: %+v", want, got)
		}
	}
	// The seeded e2e user is the org admin, so they may manage templates.
	if !resp.Msg.CanManage {
		t.Error("can_manage = false for the seeded admin")
	}
}

// createTemplate makes an org template and removes it when the test ends. The
// suite runs against a live deployment, so leaving rows behind would make later
// runs depend on earlier ones.
func createTemplate(t *testing.T, client framesv1connect.FrameServiceClient, title string, rules map[string]*framesv1.FieldRule) *framesv1.FrameTemplate {
	t.Helper()
	ctx := context.Background()
	resp, err := client.CreateFrameTemplate(ctx, connect.NewRequest(&framesv1.CreateFrameTemplateRequest{
		Title: title, Description: "created by the e2e suite", FieldRules: rules,
	}))
	if err != nil {
		t.Fatalf("create template %q: %v", title, err)
	}
	id := resp.Msg.Template.Id
	t.Cleanup(func() {
		_, _ = client.DeleteFrameTemplate(context.Background(),
			connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{Id: id}))
	})
	return resp.Msg.Template
}

func listIDs(t *testing.T, client framesv1connect.FrameServiceClient) map[string]*framesv1.FrameTemplateSummary {
	t.Helper()
	resp, err := client.ListFrameTemplates(context.Background(),
		connect.NewRequest(&framesv1.ListFrameTemplatesRequest{}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := map[string]*framesv1.FrameTemplateSummary{}
	for _, s := range resp.Msg.Templates {
		out[s.Id] = s
	}
	return out
}

// TestOrgTemplateLifecycle is journey 5 through the real stack.
func TestOrgTemplateLifecycle(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	title := "E2E House Style " + uniqueSuffix(t)

	created := createTemplate(t, client, title, map[string]*framesv1.FieldRule{
		"style": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED, Note: "Voice and tone."},
	})
	if created.Builtin {
		t.Error("a created template came back flagged builtin")
	}

	listed := listIDs(t, client)
	if got, ok := listed[created.Id]; !ok {
		t.Fatalf("the created template is not in the list")
	} else if got.Builtin || got.Title != title {
		t.Errorf("listed as %+v, want title %q and builtin=false", got, title)
	}

	renamed := title + " v2"
	if _, err := client.UpdateFrameTemplate(ctx, connect.NewRequest(&framesv1.UpdateFrameTemplateRequest{
		Id: created.Id, Title: renamed, Description: "renamed by the e2e suite",
		FieldRules: map[string]*framesv1.FieldRule{
			"style": {Level: framesv1.Requirement_REQUIREMENT_RECOMMENDED, Note: "Softened."},
		},
	})); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := listIDs(t, client)[created.Id]; got == nil || got.Title != renamed {
		t.Errorf("after update, listed as %+v, want title %q", got, renamed)
	}

	fetched, err := client.GetFrameTemplate(ctx, connect.NewRequest(&framesv1.GetFrameTemplateRequest{Id: created.Id}))
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if fetched.Msg.Template.FieldRules["style"].Level != framesv1.Requirement_REQUIREMENT_RECOMMENDED {
		t.Errorf("the rule was not updated: %+v", fetched.Msg.Template.FieldRules)
	}

	if _, err := client.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{
		Id: created.Id,
	})); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, still := listIDs(t, client)[created.Id]; still {
		t.Error("the template is still listed after being deleted")
	}
	if _, err := client.GetFrameTemplate(ctx, connect.NewRequest(&framesv1.GetFrameTemplateRequest{
		Id: created.Id,
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("get after delete: code = %v (err %v), want NotFound", connect.CodeOf(err), err)
	}
}

// TestBuiltinsAreImmutableOverTheWire is journey 7.
func TestBuiltinsAreImmutableOverTheWire(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	_, updateErr := client.UpdateFrameTemplate(ctx, connect.NewRequest(&framesv1.UpdateFrameTemplateRequest{
		Id: "builtin:blank", Title: "Hijacked", Description: "d",
	}))
	if connect.CodeOf(updateErr) != connect.CodeInvalidArgument {
		t.Errorf("update builtin: code = %v (err %v), want InvalidArgument", connect.CodeOf(updateErr), updateErr)
	}
	_, deleteErr := client.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{
		Id: "builtin:blank",
	}))
	if connect.CodeOf(deleteErr) != connect.CodeInvalidArgument {
		t.Errorf("delete builtin: code = %v (err %v), want InvalidArgument", connect.CodeOf(deleteErr), deleteErr)
	}
	// The control: it is still there and still readable.
	if _, err := client.GetFrameTemplate(ctx, connect.NewRequest(&framesv1.GetFrameTemplateRequest{
		Id: "builtin:blank",
	})); err != nil {
		t.Errorf("the built-in became unreadable: %v", err)
	}
}

// TestRequiredSlotRefusesThenAccepts is journey 3. It walks the whole
// refusal-then-success path through the gateway, and asserts the
// FieldViolations detail survives the wire: without the detail the web form
// cannot put the message on the section.
func TestRequiredSlotRefusesThenAccepts(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	name := "e2e-vocab-" + uniqueSuffix(t)
	t.Cleanup(func() {
		_, _ = client.DeleteFrame(context.Background(), connect.NewRequest(&framesv1.DeleteFrameRequest{
			OrgSlug: orgSlug(t, client), Name: name, Force: true,
		}))
	})

	empty := []byte("name: " + name + "\ndescription: E2E vocabulary\nversion: 1.0.0\nslots: {}\n")
	_, err := client.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content: empty, TemplateId: "builtin:domain-vocabulary",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("publish with an empty required slot: code = %v (err %v), want InvalidArgument", connect.CodeOf(err), err)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	found := false
	for _, d := range ce.Details() {
		msg, decodeErr := d.Value()
		if decodeErr != nil {
			continue
		}
		if fv, ok := msg.(*framesv1.FieldViolations); ok {
			for _, v := range fv.Violations {
				if v.Field == "slots.terminology" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("no FieldViolations detail naming slots.terminology survived the wire")
	}

	filled := []byte("name: " + name + "\ndescription: E2E vocabulary\nversion: 1.0.0\n" +
		"slots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n")
	if _, err := client.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content: filled, TemplateId: "builtin:domain-vocabulary",
	})); err != nil {
		t.Fatalf("publish with the slot filled: %v", err)
	}
}

// TestAFrameFromATemplateStaysOrdinary is journey 9.
func TestAFrameFromATemplateStaysOrdinary(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	suffix := uniqueSuffix(t)
	tmpl := createTemplate(t, client, "E2E Vocabulary "+suffix, map[string]*framesv1.FieldRule{
		"terminology": {Level: framesv1.Requirement_REQUIREMENT_REQUIRED, Note: "Define your terms."},
	})
	name := "e2e-ordinary-" + suffix
	org := orgSlug(t, client)
	t.Cleanup(func() {
		_, _ = client.DeleteFrame(context.Background(), connect.NewRequest(&framesv1.DeleteFrameRequest{
			OrgSlug: org, Name: name, Force: true,
		}))
	})

	if _, err := client.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content: []byte("name: " + name + "\ndescription: E2E ordinary\nversion: 1.0.0\n" +
			"slots:\n  terminology:\n    - term: Frame\n      definition: A scoped context artifact.\n"),
		TemplateId: tmpl.Id,
	})); err != nil {
		t.Fatalf("create from template: %v", err)
	}

	// Delete the template out from under the Frame.
	if _, err := client.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{
		Id: tmpl.Id,
	})); err != nil {
		t.Fatalf("delete template: %v", err)
	}

	// The Frame still resolves.
	if _, err := client.ResolveFrame(ctx, connect.NewRequest(&framesv1.ResolveFrameRequest{
		OrgSlug: org, Name: name,
	})); err != nil {
		t.Errorf("resolve after the template was deleted: %v", err)
	}

	// And a later version may empty the once-required section, because nothing
	// was ever recorded on the Frame.
	if _, err := client.PublishFrame(ctx, connect.NewRequest(&framesv1.PublishFrameRequest{
		Content: []byte("name: " + name + "\ndescription: E2E ordinary\nversion: 1.0.1\nslots: {}\n"),
	})); err != nil {
		t.Errorf("publishing an update that empties the required slot: %v", err)
	}
}

// orgSlug asks the server which org the caller is in, rather than hardcoding the
// chart's seed value: the suite should still work against a differently seeded
// deployment.
func orgSlug(t *testing.T, client framesv1connect.FrameServiceClient) string {
	t.Helper()
	resp, err := client.GetMe(context.Background(), connect.NewRequest(&framesv1.GetMeRequest{}))
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if resp.Msg.Org == nil {
		t.Fatal("GetMe returned no org")
	}
	return resp.Msg.Org.Slug
}

// uniqueSuffix keeps names from colliding between concurrent or repeated runs
// against the same deployment.
func uniqueSuffix(t *testing.T) string {
	t.Helper()
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
