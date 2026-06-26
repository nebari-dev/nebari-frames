package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// devClaims are injected when running without an OIDC provider so that
// write/authenticated operations work in local development.
var devClaims = &Claims{
	Subject: "dev-user",
	Email:   "dev@localhost",
}

// NewInterceptor returns a ConnectRPC unary interceptor enforcing OIDC auth.
// When devMode is true, authentication is skipped and fixed dev-user claims are
// injected (local development only). Otherwise a valid Bearer token is required:
// a not-ready validator yields CodeUnavailable (503), and a missing or invalid
// token yields CodeUnauthenticated.
func NewInterceptor(v TokenValidator, devMode bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if devMode {
				ctx = WithClaims(ctx, devClaims)
				return next(ctx, req)
			}

			// Guard the invariant that non-dev mode always has a real validator.
			// A nil validator here would panic on v.Validate below; returning
			// CodeUnavailable keeps the service fail-closed rather than crashing.
			if v == nil {
				return nil, connect.NewError(connect.CodeUnavailable, errors.New("authentication temporarily unavailable"))
			}

			// CutPrefix handles both missing prefix and empty token in one check:
			// "Bearer xyz" -> ("xyz", true), "Bearer " -> ("", true), "Basic x" -> ("", false)
			token, ok := strings.CutPrefix(req.Header().Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				// nil message is intentional: no information leak for an obviously-missing token.
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			claims, err := v.Validate(ctx, token)
			if err != nil {
				if errors.Is(err, ErrNotReady) {
					return nil, connect.NewError(connect.CodeUnavailable, errors.New("authentication temporarily unavailable"))
				}
				// Generic message to avoid leaking OIDC error details to clients.
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			ctx = WithClaims(ctx, claims)
			return next(ctx, req)
		}
	}
}
