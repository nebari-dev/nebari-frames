// Package orgs resolves an authenticated caller's org membership and role.
package orgs

import (
	"context"
	"errors"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

var (
	ErrNoClaims     = errors.New("no authenticated claims in context")
	ErrNoMembership = errors.New("user has no org membership")
)

// ResolveCaller builds an rbac.Caller from the request's auth claims and the
// user's org membership.
func ResolveCaller(ctx context.Context, repo store.Repository) (rbac.Caller, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return rbac.Caller{}, ErrNoClaims
	}
	m, err := repo.GetMembership(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return rbac.Caller{}, ErrNoMembership
		}
		return rbac.Caller{}, err
	}
	return rbac.Caller{
		Subject: claims.Subject,
		Email:   claims.Email,
		OrgID:   m.OrgId,
		Role:    rbac.Role(m.Role),
	}, nil
}
