package frames

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// These tests exercise PublishDocRequest's template check. They live in
// package frames, alongside templates_service_test.go, rather than in the
// external frames_test package that service_test.go uses: newTemplateFixture
// and seedOrgTemplate are unexported, so an external test package cannot reach
// them, and duplicating those fixtures here would defeat the point of reusing
// them.

func TestPublishChecksTemplateRequirements(t *testing.T) {
	docWith := func(slots Slots) *Doc {
		return &Doc{Name: "test-frame", Description: "A test frame", Version: "1.0.0", Slots: slots}
	}
	tests := []struct {
		name       string
		templateID string
		slots      Slots
		wantPaths  []string
		wantCode   connect.Code
	}{
		{
			name:       "no template means no template checks",
			templateID: "",
			slots:      Slots{},
		},
		{
			name:       "required slot empty is refused, naming the slot",
			templateID: "builtin:domain-vocabulary",
			slots:      Slots{},
			wantPaths:  []string{"slots.terminology"},
			wantCode:   connect.CodeInvalidArgument,
		},
		{
			name:       "required slot filled publishes",
			templateID: "builtin:domain-vocabulary",
			slots:      Slots{Terminology: []Term{{Term: "Frame", Definition: "A scoped context artifact."}}},
		},
		{
			// domain-vocabulary marks `rules` recommended. Leaving it empty while
			// the required slot is filled must still publish: recommended is
			// guidance, and this is the case that proves it is not quietly
			// enforced. Without the explicit nil this is the previous case again.
			name:       "a recommended slot left empty still publishes",
			templateID: "builtin:domain-vocabulary",
			slots:      Slots{Terminology: []Term{{Term: "Frame", Definition: "A scoped context artifact."}}, Rules: nil},
		},
		{
			name:       "an unknown template id is refused",
			templateID: "builtin:nope",
			slots:      Slots{},
			wantCode:   connect.CodeInvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ctx := newTemplateFixture(t, "admin")
			_, _, err := f.svc.PublishDocRequest(ctx, docWith(tt.slots), PublishRequest{
				Intent: PublishCreate, TemplateID: tt.templateID,
			})
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if connect.CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tt.wantCode)
			}
			if len(tt.wantPaths) == 0 {
				return
			}
			// The violation must arrive as a FieldViolations detail keyed by the
			// slot path, or the web form cannot put the message on the section.
			var ce *connect.Error
			if !errors.As(err, &ce) {
				t.Fatalf("error is not a connect error: %v", err)
			}
			got := map[string]bool{}
			for _, d := range ce.Details() {
				msg, decodeErr := d.Value()
				if decodeErr != nil {
					continue
				}
				if fv, ok := msg.(*framesv1.FieldViolations); ok {
					for _, v := range fv.Violations {
						got[v.Field] = true
					}
				}
			}
			for _, want := range tt.wantPaths {
				if !got[want] {
					t.Errorf("no field violation for %q; got %v", want, got)
				}
			}
		})
	}
}

func TestPublishMergesSchemaAndTemplateViolations(t *testing.T) {
	// An author fixing schema errors and only then discovering the template's
	// requirements is a worse experience than seeing both at once.
	f, ctx := newTemplateFixture(t, "admin")
	_, _, err := f.svc.PublishDocRequest(ctx, &Doc{
		Name: "BAD NAME", Description: "", Version: "1.0.0",
	}, PublishRequest{Intent: PublishCreate, TemplateID: "builtin:domain-vocabulary"})
	if err == nil {
		t.Fatal("publish succeeded on an invalid document")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	got := map[string]bool{}
	for _, d := range ce.Details() {
		msg, decodeErr := d.Value()
		if decodeErr != nil {
			continue
		}
		if fv, ok := msg.(*framesv1.FieldViolations); ok {
			for _, v := range fv.Violations {
				got[v.Field] = true
			}
		}
	}
	for _, want := range []string{"name", "description", "slots.terminology"} {
		if !got[want] {
			t.Errorf("missing violation for %q; got %v", want, got)
		}
	}
}

func TestPublishWithTemplateAgainstAnExistingFrameIsRefused(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	doc := &Doc{
		Name: "test-frame", Description: "A test frame", Version: "1.0.0",
		Slots: Slots{Terminology: []Term{{Term: "Frame", Definition: "A scoped context artifact."}}},
	}
	if _, _, err := f.svc.PublishDocRequest(ctx, doc, PublishRequest{
		Intent: PublishCreate, TemplateID: "builtin:domain-vocabulary",
	}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	doc.Version = "1.0.1"
	_, _, err := f.svc.PublishDocRequest(ctx, doc, PublishRequest{
		Intent: PublishCreate, TemplateID: "builtin:domain-vocabulary",
	})
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("code = %v (err %v), want AlreadyExists", connect.CodeOf(err), err)
	}
}

func TestAFrameFromATemplateIsOrdinaryAfterwards(t *testing.T) {
	// Journey 9. Requirements are checked at creation only: nothing is recorded
	// on the Frame, so a later version may empty the section, and deleting the
	// template cannot affect it.
	f, ctx := newTemplateFixture(t, "admin")
	seedOrgTemplate(t, f.repo, "t1", "org-a", "Vocabulary", map[string]FieldRule{
		"terminology": {Level: RequirementRequired, Note: "Define your terms."},
	})
	doc := &Doc{
		Name: "test-frame", Description: "A test frame", Version: "1.0.0",
		Slots: Slots{Terminology: []Term{{Term: "Frame", Definition: "A scoped context artifact."}}},
	}
	if _, _, err := f.svc.PublishDocRequest(ctx, doc, PublishRequest{
		Intent: PublishCreate, TemplateID: "t1",
	}); err != nil {
		t.Fatalf("create from template: %v", err)
	}

	// Delete the template out from under the Frame.
	if _, err := f.svc.DeleteFrameTemplate(ctx, connect.NewRequest(&framesv1.DeleteFrameTemplateRequest{Id: "t1"})); err != nil {
		t.Fatalf("delete template: %v", err)
	}

	// A later version may empty the once-required section.
	if _, _, err := f.svc.PublishDocRequest(ctx, &Doc{
		Name: "test-frame", Description: "A test frame", Version: "1.0.1",
	}, PublishRequest{Intent: PublishUpdate}); err != nil {
		t.Fatalf("publishing an update after the template was deleted: %v", err)
	}
}

func TestPublishRejectsAnotherOrgsTemplateID(t *testing.T) {
	f, ctx := newTemplateFixture(t, "admin")
	seedOrgTemplate(t, f.repo, "theirs", "org-b", "Theirs", nil)
	_, _, err := f.svc.PublishDocRequest(ctx, &Doc{
		Name: "test-frame", Description: "A test frame", Version: "1.0.0",
	}, PublishRequest{Intent: PublishCreate, TemplateID: "theirs"})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v (err %v), want InvalidArgument", connect.CodeOf(err), err)
	}
	// The control: the caller's own template on the same call succeeds.
	seedOrgTemplate(t, f.repo, "mine", "org-a", "Mine", nil)
	if _, _, err := f.svc.PublishDocRequest(ctx, &Doc{
		Name: "test-frame", Description: "A test frame", Version: "1.0.0",
	}, PublishRequest{Intent: PublishCreate, TemplateID: "mine"}); err != nil {
		t.Fatalf("own template = %v, want success", err)
	}
}
