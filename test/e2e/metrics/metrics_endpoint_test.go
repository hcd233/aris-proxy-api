package metrics

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		t.Skip("BASE_URL is required for e2e test")
	}
	return strings.TrimRight(baseURL, "/")
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

func TestMetricsEndpoint_Returns200(t *testing.T) {
	t.Parallel()
	baseURL := mustE2EEnv(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/metrics", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRuntimeMetricsEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	baseURL := mustE2EEnv(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/metrics/runtime?range=15m", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 without token, got %d", resp.StatusCode)
	}
}

func TestRuntimeMetricsEndpoint_AdminReturnsSeries(t *testing.T) {
	t.Parallel()
	baseURL := mustE2EEnv(t)
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		t.Skip("ADMIN_TOKEN is required for admin e2e test")
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/metrics/runtime?range=15m", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 with admin token, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var result dto.HTTPResponse[dto.RuntimeMetricsRsp]
	if err := sonic.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Body.Error != nil {
		t.Fatalf("expected no error in response, got %v", result.Body.Error)
	}
	if result.Body.Series.SSEActive == nil {
		t.Error("expected series.sseActive map to be present (may be empty)")
	}
}
