// Package orgs resolves an authenticated caller's org membership and role.
package orgs

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

var (
	ErrNoClaims     = errors.New("no authenticated claims in context")
	ErrNoMembership = errors.New("user has no org membership")
)

// DefaultMembership configures the baseline access granted to an authenticated
// caller the store has never seen. The zero value denies such callers, which
// keeps a deployment that wants membership to be explicit fail-closed.
//
// Enabling it moves the trust boundary onto the identity provider: anyone the
// realm admits gets the baseline role. It also means RemoveOrgMember stops
// being a revocation, since the removed user is re-provisioned at the baseline
// on their next request.
type DefaultMembership struct {
	Role    rbac.Role // baseline role; empty means deny
	OrgSlug string    // org the baseline membership joins; empty means deny
}

// enabled reports whether a baseline membership should be provisioned. Both
// fields are required: a role with nowhere to put it is not a usable config.
func (d DefaultMembership) enabled() bool { return d.Role != "" && d.OrgSlug != "" }

// ResolveCaller builds an rbac.Caller from the request's auth claims and the
// user's org membership.
func ResolveCaller(ctx context.Context, repo store.Repository, def DefaultMembership) (rbac.Caller, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return rbac.Caller{}, ErrNoClaims
	}
	// A subject is what identifies the caller, and the store overloads an empty
	// user_sub to mean "pending invite". Provisioning a subjectless caller would
	// therefore write a row that impersonates an invite, carry the default role,
	// and occupy the (org, email) slot so a real invite for that address could
	// never be created. OIDC requires `sub`, so this is a malformed token.
	if claims.Subject == "" {
		return rbac.Caller{}, ErrNoClaims
	}
	m, err := repo.GetMembership(ctx, claims.Subject)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return rbac.Caller{}, err
		}
		m, err = withoutMembership(ctx, repo, claims, def)
		if err != nil {
			return rbac.Caller{}, err
		}
	}
	return rbac.Caller{
		Subject: claims.Subject,
		Email:   claims.Email,
		OrgID:   m.OrgId,
		Role:    rbac.Role(m.Role),
	}, nil
}

// withoutMembership resolves a caller with no sub-keyed membership: first a
// pending invite matched by email, then the configured default membership. The
// invite is tried first so an admin's explicitly chosen role is never silently
// downgraded to the baseline.
func withoutMembership(ctx context.Context, repo store.Repository, claims *auth.Claims, def DefaultMembership) (*framesv1.Membership, error) {
	// The claim is trimmed before lookup, and the lookup itself is
	// case-insensitive: an invite is typed by a human while the claim comes from
	// the identity provider, and the two disagreeing on case must not cause the
	// invite to be missed. Missing it would silently hand the caller the default
	// role, which can outrank the role they were actually invited with.
	if email := store.CanonicalEmail(claims.Email); email != "" {
		pending, err := repo.GetPendingMembershipByEmail(ctx, email)
		switch {
		case err == nil:
			// Activate by the stored address, not the claim: they may differ in
			// case, and the update must match the row the lookup returned.
			if aerr := repo.ActivatePendingMembership(ctx, pending.Email, claims.Subject); aerr != nil {
				return nil, aerr
			}
			pending.UserSub = claims.Subject
			return pending, nil
		case !errors.Is(err, store.ErrNotFound):
			return nil, err
		}
	}
	return provisionDefault(ctx, repo, claims, def)
}

// provisionDefault writes the baseline membership for a caller the store has
// never seen. It is deliberately the last resort: with no default configured it
// denies, which is the pre-existing behavior.
func provisionDefault(ctx context.Context, repo store.Repository, claims *auth.Claims, def DefaultMembership) (*framesv1.Membership, error) {
	if !def.enabled() {
		return nil, ErrNoMembership
	}
	org, err := repo.GetOrgBySlug(ctx, def.OrgSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// A default role pointed at an org that does not exist is a
			// misconfiguration. Deny rather than fail open or surface a 500:
			// the operator asked for baseline access to *that* org, not to any.
			slog.Warn("orgs: default role configured for unknown org, denying caller",
				"org_slug", def.OrgSlug, "subject", claims.Subject)
			return nil, ErrNoMembership
		}
		return nil, err
	}
	m := &framesv1.Membership{
		OrgId:   org.Id,
		UserSub: claims.Subject,
		Role:    string(def.Role),
		// Canonical, not verbatim: a row carrying the claim's stray whitespace or
		// casing would not collide with a later invite typed normally, so that
		// invite would be accepted and then never take effect.
		Email:   store.CanonicalEmail(claims.Email),
		AddedAt: timestamppb.Now(),
	}
	// Insert-only. UpsertMembership is UPDATE-first, so using it here would let
	// a caller acting on a stale "no membership" read rewrite a row that another
	// request established in the meantime - turning an activated invite's role
	// into the baseline, permanently.
	if err := repo.CreateMembership(ctx, m); err != nil {
		if !errors.Is(err, store.ErrAlreadyExists) {
			return nil, err
		}
		// Something else holds this slot. Either a concurrent request already
		// wrote a membership for this subject - possibly by activating an invite,
		// so its role outranks the baseline we assumed - or the (org, email) pair
		// belongs to a different subject, which is what a deleted-and-recreated
		// identity-provider account looks like. Re-reading distinguishes them:
		// a row for our subject is authoritative, and no row means the address is
		// taken, which is a denial rather than a server fault.
		existing, rerr := repo.GetMembership(ctx, claims.Subject)
		if rerr == nil {
			return existing, nil
		}
		if errors.Is(rerr, store.ErrNotFound) {
			slog.Warn("orgs: cannot provision default membership, email already belongs to another subject; denying caller",
				"org_slug", def.OrgSlug, "subject", claims.Subject)
			return nil, ErrNoMembership
		}
		return nil, rerr
	}
	return m, nil
}
