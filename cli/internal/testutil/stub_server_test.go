package testutil_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/cli/internal/api"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
)

func TestStubServer_ServesConfiguredResponse(t *testing.T) {
	stub := &testutil.StubService{
		MeFn: func(_ context.Context, _ *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error) {
			return connect.NewResponse(&framesv1.GetMeResponse{Subject: "u1", Role: "publisher"}), nil
		},
	}
	url := testutil.NewStubServer(t, stub)
	me, err := api.NewClient(url).Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Subject != "u1" || me.Role != "publisher" {
		t.Fatalf("got %+v", me)
	}
}
