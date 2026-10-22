// Package apikey_auth_status 验证 API Key 鉴权的失败语义：
// 不存在的 Key 必须是 401，而不是 503；503 只保留给 DB 不可用等基础设施错误。
//
// 默认离线 skip，需显式提供 BASE_URL。
package apikey_auth_status

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestUnknownAPIKeyReturns401 用不存在的 Key 请求 /models，必须 401。
func TestUnknownAPIKeyReturns401(t *testing.T) {
	t.Parallel()

	baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if baseURL == "" {
		t.Skip("BASE_URL is required for e2e test")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/openai/v1/models", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "sk-aris-e2e-nonexistent-key-000000")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown api key status = %d, want 401", resp.StatusCode)
	}
}
