package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// devClaims are injected when running without an OIDC provider so that
// write operations (PublishSkill) work in local development.
var devClaims = &Claims{
	Subject: "dev-user",
	Email:   "dev@localhost",
}

// NewInterceptor returns a ConnectRPC unary interceptor that validates
// Bearer tokens using the given validator. If validator is nil (local dev
// mode), requests pass through with default dev claims injected.
func NewInterceptor(v TokenValidator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if v == nil {
				ctx = WithClaims(ctx, devClaims)
				return next(ctx, req)
			}

			// CutPrefix handles both missing prefix and empty token in one check:
			// "Bearer xyz" -> ("xyz", true), "Bearer " -> ("", true), "Basic x" -> ("", false)
			token, ok := strings.CutPrefix(req.Header().Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			claims, err := v.Validate(ctx, token)
			if err != nil {
				// Return a generic message to avoid leaking OIDC error details to clients.
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			ctx = WithClaims(ctx, claims)
			return next(ctx, req)
		}
	}
}
