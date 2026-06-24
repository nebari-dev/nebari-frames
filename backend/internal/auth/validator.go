package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// TokenValidator validates a raw Bearer token and returns the extracted claims.
// The interceptor depends on this interface for testability.
type TokenValidator interface {
	Validate(ctx context.Context, rawToken string) (*Claims, error)
}

// Validator is the production OIDC token validator.
// It fetches JWKS from the OIDC provider and verifies JWT signatures.
type Validator struct {
	verifier *oidc.IDTokenVerifier
	cfg      Config
}

var _ TokenValidator = (*Validator)(nil)

// NewValidator creates a Validator by fetching the OIDC discovery document
// from cfg.IssuerURL. Fails fast if the provider is unreachable.
func NewValidator(ctx context.Context, cfg Config) (*Validator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %s: %w", cfg.IssuerURL, err)
	}
	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})
	return &Validator{verifier: verifier, cfg: cfg}, nil
}

// Validate verifies the token signature, expiry, and audience, then extracts claims.
func (v *Validator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}

	// Claims() unmarshals the already-verified JWT payload into a map.
	// This can only fail if the OIDC library has an internal JSON bug
	// after successful verification - effectively unreachable in practice.
	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	claims := &Claims{
		Subject: idToken.Subject,
	}

	if email, ok := raw["email"].(string); ok {
		claims.Email = email
	}

	if groups, ok := raw[v.cfg.GroupsClaim]; ok {
		if arr, ok := groups.([]any); ok {
			for _, g := range arr {
				if s, ok := g.(string); ok {
					claims.Groups = append(claims.Groups, s)
				}
			}
		}
	}

	return claims, nil
}

// IsAdmin checks whether the claims include the given admin group.
// This is a pure function with no dependency on the OIDC provider.
func IsAdmin(adminGroup string, claims *Claims) bool {
	if claims == nil {
		return false
	}
	for _, g := range claims.Groups {
		if g == adminGroup {
			return true
		}
	}
	return false
}
