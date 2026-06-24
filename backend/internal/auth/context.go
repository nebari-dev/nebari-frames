package auth

import "context"

// Claims holds the verified identity extracted from an OIDC token.
type Claims struct {
	Subject string
	Email   string
	Groups  []string
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
