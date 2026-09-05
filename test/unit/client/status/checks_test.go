package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/setup"
	"github.com/hcd233/aris-proxy-api/internal/client/status"
	"github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func testPaths(t *testing.T) trace.Paths {
	t.Helper()
	return trace.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
}

func saveConfig(t *testing.T, paths trace.Paths, host, key string) {
	t.Helper()
	store := trace.NewConfigStore(paths)
	if err := store.Save(context.Background(), trace.Config{Host: host, APIKey: key}); err != nil {
		t.Fatal(err)
	}
}

func TestCollectWithoutConfigSkipsNetwork(t *testing.T) {
	t.Parallel()
	report := status.Collect(context.Background(), testPaths(t), nil)
	if report.ConfigFound {
		t.Fatal("ConfigFound should be false")
	}
	if report.ServerOK || report.AuthOK {
		t.Fatal("network checks should be skipped without config")
	}
	if report.HooksTotal != 3 {
		t.Fatalf("HooksTotal = %d, want 3", report.HooksTotal)
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

	report := status.Collect(context.Background(), paths, nil)
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

func TestCollectLocalFiles(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)

	// spool pending：2 个记录文件
	if err := os.MkdirAll(paths.PendingDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.json", "b.json"} {
		if err := os.WriteFile(filepath.Join(paths.PendingDir(), name), []byte(`{"x":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// rejected：1 个
	if err := os.MkdirAll(paths.RejectedDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.RejectedDir(), "c.json"), []byte(`{"y":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// 当日日志：3 条
	if err := os.MkdirAll(paths.LogDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	logName := constant.ArisClientLogPrefix + time.Now().UTC().Format(constant.ArisClientLogDateFormat) + constant.ArisClientLogSuffix
	if err := os.WriteFile(filepath.Join(paths.LogDir(), logName), []byte("l1\nl2\nl3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// hooks：以当前可执行文件路径注册
	binPath, err := setup.ExecutablePath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trace.InstallCodexHooks(paths, binPath); err != nil {
		t.Fatal(err)
	}

	report := status.Collect(context.Background(), paths, nil)
	if report.PendingCount != 2 {
		t.Fatalf("PendingCount = %d, want 2", report.PendingCount)
	}
	if report.PendingBytes <= 0 {
		t.Fatalf("PendingBytes = %d, want > 0", report.PendingBytes)
	}
	if report.RejectedCount != 1 {
		t.Fatalf("RejectedCount = %d, want 1", report.RejectedCount)
	}
	if report.RecentErrors != 3 {
		t.Fatalf("RecentErrors = %d, want 3", report.RecentErrors)
	}
	if report.HooksFound != 3 || len(report.HooksMissing) != 0 {
		t.Fatalf("Hooks = %d/%v, want 3/[]", report.HooksFound, report.HooksMissing)
	}
}
