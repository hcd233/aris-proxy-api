package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	clientapi "github.com/hcd233/aris-proxy-api/internal/client/api"
)

func TestCheckHealthReturnsLatency(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	latency, err := clientapi.New(server.URL, "", nil).CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("CheckHealth error: %v", err)
	}
	if latency < 0 || latency > 5*time.Second {
		t.Fatalf("CheckHealth latency %v out of expected range", latency)
	}
}

func TestCheckHealthTrimsTrailingSlash(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path %q, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if _, err := clientapi.New(server.URL+"/", "", nil).CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth error: %v", err)
	}
}

func TestCheckHealthNon2xxFails(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if _, err := clientapi.New(server.URL, "", nil).CheckHealth(context.Background()); err == nil {
		t.Fatal("CheckHealth should fail on 503")
	}
}

func TestCheckAPIKeySendsBearerAndPasses(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := clientapi.New(server.URL, "test-key", nil).CheckAPIKey(context.Background()); err != nil {
		t.Fatalf("CheckAPIKey error: %v", err)
	}
}

func TestCheckAPIKeyUnauthorizedFails(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	if err := clientapi.New(server.URL, "bad-key", nil).CheckAPIKey(context.Background()); err == nil {
		t.Fatal("CheckAPIKey should fail on 401")
	}
}

func TestCheckHealthTimeoutFails(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 阻塞至客户端超时取消请求（客户端超时后 request context 关闭）
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if _, err := clientapi.New(server.URL, "", &http.Client{Timeout: 20 * time.Millisecond}).CheckHealth(context.Background()); err == nil {
		t.Fatal("CheckHealth should fail on timeout")
	}
}
