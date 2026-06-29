package mcp

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
)

type stubValidator struct {
	claims *auth.Claims
	err    error
}

func (s stubValidator) Validate(_ context.Context, _ string) (*auth.Claims, error) {
	return s.claims, s.err
}

func TestNewTokenVerifier(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	tests := []struct {
		name        string
		val         stubValidator
		wantErr     bool
		wantInvalid bool // err unwraps to ErrInvalidToken
		wantSub     string
		wantExp     time.Time
	}{
		{
			name:    "valid token",
			val:     stubValidator{claims: &auth.Claims{Subject: "u1", Email: "u1@x", Expiry: exp}},
			wantSub: "u1", wantExp: exp,
		},
		{
			name:        "bad signature / wrong audience",
			val:         stubValidator{err: errors.New("verify token: audience mismatch")},
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "validator not ready",
			val:         stubValidator{err: auth.ErrNotReady},
			wantErr:     true,
			wantInvalid: true, // mapped to 401 to stay fail-closed
		},
	}
	verifyReq := httptest.NewRequest("POST", "/mcp", nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vf := newTokenVerifier(tt.val)
			ti, err := vf(context.Background(), "tok", verifyReq)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantInvalid && !errors.Is(err, mcpauth.ErrInvalidToken) {
					t.Errorf("err %v does not unwrap to ErrInvalidToken", err)
				}
				return
			}
			if ti.UserID != tt.wantSub {
				t.Errorf("UserID=%q want %q", ti.UserID, tt.wantSub)
			}
			if !ti.Expiration.Equal(tt.wantExp) {
				t.Errorf("Expiration=%v want %v", ti.Expiration, tt.wantExp)
			}
			gotClaims, ok := claimsFromTokenInfo(ti)
			if !ok || gotClaims.Subject != tt.wantSub {
				t.Errorf("claims not round-tripped via Extra: %+v ok=%v", gotClaims, ok)
			}
		})
	}
}
