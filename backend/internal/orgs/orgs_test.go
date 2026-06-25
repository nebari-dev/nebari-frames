package orgs_test

import (
	"context"
	"errors"
	"testing"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/orgs"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

func TestResolveCallerReconcilesPendingByEmail(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	_ = repo.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "publisher", Email: "new@x.io"})

	// inject claims for a user with no sub-keyed membership yet
	ctx = auth.WithClaims(ctx, &auth.Claims{Subject: "sub-new", Email: "new@x.io"})

	caller, err := orgs.ResolveCaller(ctx, repo)
	if err != nil {
		t.Fatalf("expected reconciliation, got err %v", err)
	}
	if caller.OrgID != "o1" || caller.Role != rbac.RolePublisher {
		t.Fatalf("unexpected caller %+v", caller)
	}
	// second call now resolves directly by sub
	if _, err := repo.GetMembership(ctx, "sub-new"); err != nil {
		t.Fatalf("membership should be active after reconcile: %v", err)
	}
}

func TestResolveCaller(t *testing.T) {
	repo := store.NewMemory()
	base := context.Background()
	_ = repo.UpsertMembership(base, &framesv1.Membership{OrgId: "o1", UserSub: "u1", Role: "publisher"})

	tests := []struct {
		name       string
		ctx        context.Context
		wantErr    error
		wantCaller rbac.Caller
	}{
		{
			name:    "no claims in context",
			ctx:     base,
			wantErr: orgs.ErrNoClaims,
		},
		{
			name:    "claims but no membership",
			ctx:     auth.WithClaims(base, &auth.Claims{Subject: "stranger"}),
			wantErr: orgs.ErrNoMembership,
		},
		{
			name: "happy path",
			ctx:  auth.WithClaims(base, &auth.Claims{Subject: "u1", Email: "u1@x"}),
			wantCaller: rbac.Caller{
				OrgID:   "o1",
				Role:    rbac.RolePublisher,
				Subject: "u1",
				Email:   "u1@x",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller, err := orgs.ResolveCaller(tt.ctx, repo)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if caller.OrgID != tt.wantCaller.OrgID ||
				caller.Role != tt.wantCaller.Role ||
				caller.Subject != tt.wantCaller.Subject ||
				caller.Email != tt.wantCaller.Email {
				t.Fatalf("caller = %+v, want %+v", caller, tt.wantCaller)
			}
		})
	}
}
