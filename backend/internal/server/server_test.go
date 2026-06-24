package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/server"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
)

func TestServer_Healthz(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{
			name:     "healthz returns 200",
			path:     "/healthz",
			wantCode: http.StatusOK,
		},
		{
			name:     "unknown path returns 404",
			path:     "/not-found",
			wantCode: http.StatusNotFound,
		},
	}

	srv := server.New(store.NewMemory(), nil) // nil validator = dev mode
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("get %s: %v", tc.path, err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("%s = %d, want %d", tc.path, resp.StatusCode, tc.wantCode)
			}
		})
	}
}
