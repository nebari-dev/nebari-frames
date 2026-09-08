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
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

const (
	envBaseURL = "FRAMES_E2E_BASE_URL"
	envToken   = "FRAMES_E2E_TOKEN"
	envCACert  = "FRAMES_E2E_CA_CERT"
)

func baseURL() string { return os.Getenv(envBaseURL) }

// transport trusts the CA named by FRAMES_E2E_CA_CERT in addition to the system
// roots, falling back to the default transport when the variable is unset.
//
// Trust is loaded here rather than left to SSL_CERT_FILE because the two are not
// equivalent in practice. curl and Go disagree about what may serve as a trust
// anchor - Go requires the CA basic constraint that curl will do without - so a
// run can provision a token over curl and then fail every RPC with "certificate
// signed by unknown authority" using the very same file. Reading the file here
// turns that into a named failure against a named path instead of a puzzle.
//
// Verification stays on. A suite that skips certificate checks is not exercising
// the path it claims to.
func transport(t *testing.T) http.RoundTripper {
	t.Helper()
	path := os.Getenv(envCACert)
	if path == "" {
		return http.DefaultTransport
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s=%q: %v", envCACert, path, err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		t.Fatalf("%s=%q held no PEM certificate", envCACert, path)
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}
}

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
		Transport: bearer{base: transport(t), token: token},
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
		&http.Client{Timeout: 30 * time.Second, Transport: transport(t)}, baseURL())
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
