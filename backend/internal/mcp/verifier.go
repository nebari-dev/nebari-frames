package mcp

import (
	"context"
	"fmt"
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
)

const claimsExtraKey = "frames.claims"

// newTokenVerifier adapts the project's OIDC TokenValidator to the MCP SDK's
// TokenVerifier. The supplied validator MUST be configured with the MCP
// audience (the canonical /mcp URL) so audience binding (RFC 8707) is enforced:
// a token minted for another audience fails validation and is rejected with 401.
//
// All validation failures - including a not-ready validator - are wrapped as
// ErrInvalidToken so the middleware returns 401 and never grants access while
// the OIDC provider is unreachable (fail-closed).
func newTokenVerifier(v auth.TokenValidator) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		claims, err := v.Validate(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", mcpauth.ErrInvalidToken, err)
		}
		return &mcpauth.TokenInfo{
			UserID:     claims.Subject,
			Expiration: claims.Expiry,
			Extra:      map[string]any{claimsExtraKey: claims},
		}, nil
	}
}

// claimsFromTokenInfo extracts the project Claims previously stashed by
// newTokenVerifier in TokenInfo.Extra.
func claimsFromTokenInfo(ti *mcpauth.TokenInfo) (*auth.Claims, bool) {
	if ti == nil {
		return nil, false
	}
	c, ok := ti.Extra[claimsExtraKey].(*auth.Claims)
	return c, ok
}
