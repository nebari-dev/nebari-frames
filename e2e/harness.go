//go:build e2e

// Package e2e drives a deployed nebari-frames through its public gateway with a
// real Keycloak token. It is build-tagged and environment-gated so `make test`
// never reaches it: these tests need a cluster, and a suite that silently
// no-ops when one is absent would be worse than one that is obviously skipped.
//
// It uses the generated Connect client rather than curl, so the requests are
// typed and the suite runs locally against `make dev` as easily as against the
// sandbox.
package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

const (
	envBaseURL = "FRAMES_E2E_BASE_URL"
	envToken   = "FRAMES_E2E_TOKEN"
)

func baseURL() string { return os.Getenv(envBaseURL) }

// newClient returns a client carrying the caller's bearer token, skipping the
// test when the environment is not set up. It reports which variable is missing:
// "skipped" with no reason is how a suite quietly stops running.
func newClient(t *testing.T) framesv1connect.FrameServiceClient {
	t.Helper()
	url, token := baseURL(), os.Getenv(envToken)
	if url == "" || token == "" {
		t.Skipf("set %s and %s to run the e2e suite", envBaseURL, envToken)
	}
	return framesv1connect.NewFrameServiceClient(&http.Client{
		Timeout:   30 * time.Second,
		Transport: bearer{base: http.DefaultTransport, token: token},
	}, url)
}

// newAnonClient returns a client with no credentials, for asserting that the
// endpoint actually rejects unauthenticated callers.
func newAnonClient(t *testing.T) framesv1connect.FrameServiceClient {
	t.Helper()
	if baseURL() == "" {
		t.Skipf("set %s to run the e2e suite", envBaseURL)
	}
	return framesv1connect.NewFrameServiceClient(
		&http.Client{Timeout: 30 * time.Second}, baseURL())
}

type bearer struct {
	base  http.RoundTripper
	token string
}

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req)
}
