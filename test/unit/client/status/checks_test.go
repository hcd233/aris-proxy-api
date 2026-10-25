package status

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	clientconfig "github.com/hcd233/aris-proxy-api/internal/client/config"
	"github.com/hcd233/aris-proxy-api/internal/client/status"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func testPaths(t *testing.T) clientconfig.Paths {
	t.Helper()
	return clientconfig.Paths{Root: filepath.Join(t.TempDir(), constant.ArisClientRootDirName)}
}

func saveConfig(t *testing.T, paths clientconfig.Paths, host, key string) {
	t.Helper()
	store := clientconfig.NewStore(paths)
	if err := store.Save(t.Context(), clientconfig.Config{Host: host, APIKey: key}); err != nil {
		t.Fatal(err)
	}
}

func TestCollectWithoutConfigSkipsNetwork(t *testing.T) {
	t.Parallel()
	report := status.Collect(t.Context(), testPaths(t), nil)
	if report.ConfigFound {
		t.Fatal("ConfigFound should be false")
	}
	if report.ServerOK || report.AuthOK {
		t.Fatal("network checks should be skipped without config")
	}
}

func TestCollectAgainstLiveServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == constant.ArisClientCheckPath {
			if r.Header.Get(constant.HTTPHeaderAuthorization) != "Bearer sk-12345678" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	paths := testPaths(t)
	saveConfig(t, paths, server.URL, "sk-12345678")

	report := status.Collect(t.Context(), paths, nil)
	if !report.ConfigFound || report.Host != server.URL {
		t.Fatalf("unexpected config projection: %+v", report)
	}
	if !report.ServerOK {
		t.Fatalf("ServerOK should be true, err: %s", report.ServerErr)
	}
	if report.ServerLatency < 0 || report.ServerLatency > 5*time.Second {
		t.Fatalf("ServerLatency %v out of range", report.ServerLatency)
	}
	if !report.AuthOK {
		t.Fatalf("AuthOK should be true, err: %s", report.AuthErr)
	}
	if report.AuthMaskedKey != "••••5678" {
		t.Fatalf("AuthMaskedKey = %q, want ••••5678", report.AuthMaskedKey)
	}
}
