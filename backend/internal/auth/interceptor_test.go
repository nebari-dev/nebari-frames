package auth

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

// stubValidator is a TokenValidator returning canned results.
type stubValidator struct {
	claims *Claims
	err    error
}

func (s stubValidator) Validate(_ context.Context, _ string) (*Claims, error) {
	return s.claims, s.err
}

func TestInterceptor(t *testing.T) {
	okClaims := &Claims{Subject: "alice"}
	tests := []struct {
		name       string
		devMode    bool
		validator  TokenValidator
		authHeader string
		wantCode   connect.Code // 0 means success (no connect error)
		wantSub    string       // subject expected in context on success
	}{
		{name: "dev mode injects dev-user", devMode: true, validator: nil, wantSub: "dev-user"},
		{name: "missing token is unauthenticated", devMode: false, validator: stubValidator{}, authHeader: "", wantCode: connect.CodeUnauthenticated},
		{name: "not-ready validator is unavailable", devMode: false, validator: stubValidator{err: ErrNotReady}, authHeader: "Bearer x", wantCode: connect.CodeUnavailable},
		{name: "bad token is unauthenticated", devMode: false, validator: stubValidator{err: errors.New("bad sig")}, authHeader: "Bearer x", wantCode: connect.CodeUnauthenticated},
		{name: "valid token attaches claims", devMode: false, validator: stubValidator{claims: okClaims}, authHeader: "Bearer x", wantSub: "alice"},
		{name: "nil validator non-dev is unavailable", devMode: false, validator: nil, authHeader: "Bearer x", wantCode: connect.CodeUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotSub string
			next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
				if c, ok := ClaimsFromContext(ctx); ok {
					gotSub = c.Subject
				}
				return connect.NewResponse(&emptypb.Empty{}), nil
			})
			interceptor := NewInterceptor(tc.validator, tc.devMode)
			req := connect.NewRequest(&emptypb.Empty{})
			if tc.authHeader != "" {
				req.Header().Set("Authorization", tc.authHeader)
			}
			_, err := interceptor(next)(context.Background(), req)

			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if gotSub != tc.wantSub {
					t.Fatalf("subject = %q, want %q", gotSub, tc.wantSub)
				}
				return
			}
			if got := connect.CodeOf(err); got != tc.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tc.wantCode, err)
			}
		})
	}
}
