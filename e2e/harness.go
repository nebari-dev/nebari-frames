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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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

// transport verifies the server against the certificate named by
// FRAMES_E2E_CA_CERT, falling back to the default transport when unset.
//
// Two shapes of certificate turn up here and they need different handling.
//
// A real CA becomes a trust anchor and Go verifies the chain normally.
//
// The Nebari sandbox gateway instead serves a SELF-SIGNED LEAF: subject equals
// issuer, `CA:FALSE` marked critical, and a presented chain of one. That cannot
// be a trust anchor. Go enforces basic constraints and refuses it, which is why
// the same file verifies under curl (OpenSSL will anchor on a self-signed cert
// by exact match) and fails under Go with "certificate signed by unknown
// authority". There is no CA anywhere in that deployment to chain to.
//
// For that case the certificate is PINNED. InsecureSkipVerify turns off Go's
// chain building, and VerifyPeerCertificate then applies a stricter test than a
// chain would: the certificate the server presents must be byte-identical to the
// one on disk. Any other certificate, expired or not, signed by anyone, fails.
// This is pinning and it fails closed; it is not skipped verification, and the
// suite would be worthless if it were.
func transport(t *testing.T) http.RoundTripper {
	t.Helper()
	path := os.Getenv(envCACert)
	if path == "" {
		return http.DefaultTransport
	}
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s=%q: %v", envCACert, path, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatalf("%s=%q held no PEM block", envCACert, path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("%s=%q is not a parseable certificate: %v", envCACert, path, err)
	}

	if cert.IsCA {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		roots.AddCert(cert)
		return &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		}
	}

	pinned := cert.Raw
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Chain verification is replaced, not dropped: see VerifyPeerCertificate.
			InsecureSkipVerify: true, //nolint:gosec // pinned below
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				for _, raw := range rawCerts {
					if bytes.Equal(raw, pinned) {
						return nil
					}
				}
				return fmt.Errorf(
					"server presented %d certificate(s), none matching the one pinned from %s",
					len(rawCerts), path)
			},
		},
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
