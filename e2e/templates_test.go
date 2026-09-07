//go:build e2e

package e2e

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
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
