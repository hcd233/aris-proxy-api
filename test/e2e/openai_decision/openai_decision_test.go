// Package openai_decision 覆盖 POST /api/openai/v1/decisions 的端到端行为。
package openai_decision

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 90 * time.Second

// mustE2EEnv 返回 (baseURL, apiKey, model)；E2E 默认离线 skip，只有显式提供环境变量时才打到真实环境。
//
// DECISION_MODEL 必须是已在网关上配置为「支持 OpenAI Decision」的上游模型别名。
func mustE2EEnv(t *testing.T) (baseURL, apiKey, model string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	model = os.Getenv("DECISION_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		t.Skip("BASE_URL, API_KEY and DECISION_MODEL are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, model
}

// loadFixture 读取 fixture 并把模型占位符替换为 DECISION_MODEL。
func loadFixture(t *testing.T, name, model string) []byte {
	t.Helper()
	data, err := os.ReadFile("./fixtures/requests/" + name + ".json")
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return []byte(strings.ReplaceAll(string(data), "REPLACE_WITH_DECISION_MODEL", model))
}

// postDecisions 执行一次 POST /api/openai/v1/decisions，返回状态码与响应体。
//
// body 在函数内读完并关闭，调用方只拿到字节；禁止 http.DefaultClient（无超时）。
func postDecisions(t *testing.T, baseURL, apiKey string, body []byte) (status int, respBody []byte) {
	t.Helper()
	client := &http.Client{Timeout: e2eHTTPTimeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/decisions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // test cleanup

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, respBody
}

type decisionRsp struct {
	Model   string `json:"model"`
	Answers []struct {
		Type        string                 `json:"type"`
		Name        *string                `json:"name"`
		Probability *float64               `json:"probability"`
		Choice      sonic.NoCopyRawMessage `json:"choice"`
		Confidence  *float64               `json:"confidence"`
		Score       *float64               `json:"score"`
	} `json:"answers"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// assertDecisionResponse 断言状态码/模型别名/答案数量与类型，返回解析结果。
// 上游可能返回 refusal（模型拒答），这本身是合法响应，故一并接受。
func assertDecisionResponse(t *testing.T, status int, respBody []byte, wantType, model string) decisionRsp {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, respBody)
	}

	var parsed decisionRsp
	if err := sonic.Unmarshal(respBody, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, respBody)
	}
	if parsed.Model != model {
		t.Fatalf("model = %s, want %s (exposed alias)", parsed.Model, model)
	}
	if len(parsed.Answers) != 1 {
		t.Fatalf("answers = %d, want 1", len(parsed.Answers))
	}
	if parsed.Answers[0].Type != wantType && parsed.Answers[0].Type != "refusal" {
		t.Fatalf("answer type = %s, want %s or refusal", parsed.Answers[0].Type, wantType)
	}
	if parsed.Usage.TotalTokens <= 0 {
		t.Fatalf("usage.total_tokens = %d, want > 0", parsed.Usage.TotalTokens)
	}
	return parsed
}

func TestCreateDecision_Predicate(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	status, respBody := postDecisions(t, baseURL, apiKey, loadFixture(t, "predicate", model))
	parsed := assertDecisionResponse(t, status, respBody, "predicate", model)
	if parsed.Answers[0].Probability == nil {
		t.Fatal("predicate answer must carry probability")
	}
}

func TestCreateDecision_ChoiceWithBooleanValue(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	status, respBody := postDecisions(t, baseURL, apiKey, loadFixture(t, "choice", model))
	parsed := assertDecisionResponse(t, status, respBody, "choice", model)
	if parsed.Answers[0].Confidence == nil {
		t.Fatal("choice answer must carry confidence")
	}
}

func TestCreateDecision_Score(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	status, respBody := postDecisions(t, baseURL, apiKey, loadFixture(t, "score", model))
	parsed := assertDecisionResponse(t, status, respBody, "score", model)
	if parsed.Answers[0].Score == nil {
		t.Fatal("score answer must carry score")
	}
}

func TestCreateDecision_UnknownModelReturnsError(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, _ := mustE2EEnv(t)

	body := []byte(`{"model":"definitely-not-a-configured-alias","input":"x","questions":[{"type":"predicate","instructions":"y"}]}`)
	status, respBody := postDecisions(t, baseURL, apiKey, body)
	if status != http.StatusNotFound && status != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s, want 404/400", status, respBody)
	}
}
