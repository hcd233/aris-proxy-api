package openai_chat_completion

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

func loadRequest(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("./fixtures/requests/tool_call_non_stream.json")
	if err != nil {
		t.Fatalf("failed to read request fixture: %v", err)
	}
	return data
}

func TestChatCompletion_ToolCall_NonStream(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}

	reqBody := loadRequest(t)

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/openai/v1/chat/completions", strings.NewReader(string(reqBody)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d, body: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if !sonic.ValidString(string(respBody)) {
		t.Fatalf("response is not valid JSON: %s", string(respBody))
	}

	var obj map[string]any
	if err := sonic.Unmarshal(respBody, &obj); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if obj["id"] == nil || strings.TrimSpace(obj["id"].(string)) == "" {
		t.Errorf("missing or empty id")
	}
	if obj["model"] == nil || strings.TrimSpace(obj["model"].(string)) == "" {
		t.Errorf("missing or empty model")
	}
	if obj["choices"] == nil {
		t.Errorf("missing choices")
	}
	if obj["usage"] == nil {
		t.Errorf("missing usage")
	}
}
