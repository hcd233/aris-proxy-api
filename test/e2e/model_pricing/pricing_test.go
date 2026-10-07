// Package model_pricing 模型计费 E2E。
//
// 前置：目标环境已配置带定价的模型别名 MODEL_ALIAS（currency=USD、
// input=1/1M、output=2/1M）与未计价别名 MODEL_ALIAS_UNPRICED；
// WEB_JWT 提供 web 管理视角（缺省跳过成本断言）。默认离线 skip，不打生产。
package model_pricing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

func mustEnv(t *testing.T, keys ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s is required for e2e test", k)
		}
		out[k] = v
	}
	return out
}

func newE2EClient() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// postChat 发起一次非流式 chat 调用，断言 200。
func postChat(t *testing.T, baseURL, apiKey, model string) {
	t.Helper()
	body := `{"model":"` + model + `","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/openai/v1/chat/completions", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
}

// getJSON 带 JWT 的 web GET；返回解析后的 JSON。
func getJSON(t *testing.T, baseURL, jwt, path string) map[string]any {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		strings.TrimRight(baseURL, "/")+path, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", path, resp.StatusCode, raw)
	}
	var obj map[string]any
	if err := sonic.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return obj
}

// latestAuditCost 查审计列表最新一行的 cost/currency（deadline + ticker 轮询等待异步落库）。
func latestAuditCost(t *testing.T, baseURL, jwt, model string) (cost any, currency string) {
	t.Helper()
	const pollInterval = 2 * time.Second
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		obj := getJSON(t, baseURL, jwt,
			"/api/web/v1/audit/model/log/list?page=1&pageSize=20&query="+model)
		logs, _ := obj["logs"].([]any)
		for _, l := range logs {
			item, _ := l.(map[string]any)
			if item["modelId"] == model {
				return item["cost"], fmt.Sprint(item["pricingCurrency"])
			}
		}
		select {
		case <-deadline:
			t.Fatalf("no audit row for model %s within deadline", model)
			return nil, ""
		case <-ticker.C:
		}
	}
}

func TestPricing_CostRecordedOnSuccess(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "API_KEY", "MODEL_ALIAS", "WEB_JWT")
	postChat(t, env["BASE_URL"], env["API_KEY"], env["MODEL_ALIAS"])
	cost, currency := latestAuditCost(t, env["BASE_URL"], env["WEB_JWT"], env["MODEL_ALIAS"])
	if cost == nil {
		t.Fatalf("priced model call must have cost, got null (currency=%s)", currency)
	}
	if currency == "" || currency == "null" {
		t.Fatalf("pricing currency snapshot missing")
	}
}

func TestPricing_UnpricedModelHasNullCost(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "API_KEY", "MODEL_ALIAS_UNPRICED", "WEB_JWT")
	postChat(t, env["BASE_URL"], env["API_KEY"], env["MODEL_ALIAS_UNPRICED"])
	cost, _ := latestAuditCost(t, env["BASE_URL"], env["WEB_JWT"], env["MODEL_ALIAS_UNPRICED"])
	if cost != nil {
		t.Fatalf("unpriced model call must have null cost, got %v", cost)
	}
}

func TestPricing_ModelCostRanking(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "WEB_JWT")
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	obj := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/audit/stats/model/cost?startTime="+start+"&endTime="+end)
	if _, ok := obj["data"].([]any); !ok {
		t.Fatalf("model cost data missing: %v", obj)
	}
}
