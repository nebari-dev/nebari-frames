package auth

import (
	"context"
	"time"
)

// Claims holds the verified identity extracted from an OIDC token.
type Claims struct {
	Subject string
	Email   string
	Groups  []string
	Expiry  time.Time // token expiration (exp claim); used by the MCP bearer middleware
}

type claimsKey struct{}

// WithClaims returns a new context carrying the given claims.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromContext extracts claims from the context.
// Returns (nil, false) if no claims are present or if a nil *Claims was stored.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, _ := ctx.Value(claimsKey{}).(*Claims)
	if c == nil {
		return nil, false
	}
	return c, true
}

// DevClaims returns a fresh copy of the fixed identity injected when
// authentication is disabled (FRAMES_DEV_MODE). A new value is returned each
// call so callers cannot mutate shared state.
func DevClaims() *Claims {
	return &Claims{Subject: "dev-user", Email: "dev@localhost"}
}
