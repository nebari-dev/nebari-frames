package auth

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ErrNotReady is returned by a validator that has not yet completed OIDC
// discovery. The interceptor maps it to connect.CodeUnavailable (503) so a
// connecting provider never results in unauthenticated access.
var ErrNotReady = errors.New("auth validator not ready: OIDC provider unreachable")

// ReadinessValidator is a TokenValidator that reports whether it can currently
// validate tokens (i.e. OIDC discovery has succeeded).
type ReadinessValidator interface {
	TokenValidator
	Ready() bool
}

// LazyValidator performs OIDC discovery in the background and retries on a
// capped backoff. Until discovery succeeds, Validate returns ErrNotReady and
// Ready() is false. It satisfies ReadinessValidator.
type LazyValidator struct {
	cfg Config
	mu  sync.RWMutex
	v   *Validator // nil until discovery succeeds
}

var _ ReadinessValidator = (*LazyValidator)(nil)

// NewLazyValidator returns immediately and begins retrying OIDC discovery in a
// background goroutine until ctx is cancelled or discovery succeeds.
func NewLazyValidator(ctx context.Context, cfg Config) *LazyValidator {
	return newLazyValidator(ctx, cfg, time.Second, 30*time.Second)
}

// newLazyValidator is the testable constructor with explicit backoff bounds.
func newLazyValidator(ctx context.Context, cfg Config, initial, max time.Duration) *LazyValidator {
	l := &LazyValidator{cfg: cfg}
	go l.run(ctx, initial, max)
	return l
}

func (l *LazyValidator) run(ctx context.Context, initial, max time.Duration) {
	backoff := initial
	for {
		v, err := NewValidator(ctx, l.cfg)
		if err == nil {
			l.mu.Lock()
			l.v = v
			l.mu.Unlock()
			slog.Info("auth: OIDC discovery succeeded; validator ready", "issuer", l.cfg.IssuerURL)
			return
		}
		slog.Info("auth: OIDC discovery not ready; retrying", "issuer", l.cfg.IssuerURL, "error", err, "retry_in", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > max {
			backoff = max
		}
	}
}

// Ready reports whether OIDC discovery has completed.
func (l *LazyValidator) Ready() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.v != nil
}

// Validate delegates to the underlying validator once ready; otherwise returns
// ErrNotReady.
func (l *LazyValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	l.mu.RLock()
	v := l.v
	l.mu.RUnlock()
	if v == nil {
		return nil, ErrNotReady
	}
	return v.Validate(ctx, rawToken)
}
