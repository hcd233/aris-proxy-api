package deepseekcachehitrate

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 120 * time.Second
const e2eStreamReadDeadline = 90 * time.Second

type cacheStats struct {
	InputTokens int
	HitTokens   int
	MissTokens  int
	Explicit    bool
}

func (s cacheStats) HitRate() float64 {
	denominator := s.HitTokens + s.MissTokens
	if denominator == 0 {
		denominator = s.InputTokens
	}
	if denominator <= 0 {
		return 0
	}
	return float64(s.HitTokens) / float64(denominator)
}

func (s cacheStats) Add(other cacheStats) cacheStats {
	return cacheStats{
		InputTokens: s.InputTokens + other.InputTokens,
		HitTokens:   s.HitTokens + other.HitTokens,
		MissTokens:  s.MissTokens + other.MissTokens,
		Explicit:    s.Explicit || other.Explicit,
	}
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIScript struct {
	Model         string          `json:"model"`
	Stream        bool            `json:"stream"`
	StreamOptions *streamOptions  `json:"stream_options,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	Messages      []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model         string          `json:"model"`
	Stream        bool            `json:"stream"`
	StreamOptions *streamOptions  `json:"stream_options,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	Messages      []openAIMessage `json:"messages"`
}

type openAIUsage struct {
	PromptTokens          int  `json:"prompt_tokens"`
	CompletionTokens      int  `json:"completion_tokens"`
	TotalTokens           int  `json:"total_tokens"`
	PromptCacheHitTokens  *int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens *int `json:"prompt_cache_miss_tokens,omitempty"`
	PromptTokensDetails   *struct {
		CachedTokens *int `json:"cached_tokens,omitempty"`
	} `json:"prompt_tokens_details,omitempty"`
}

type anthropicScript struct {
	Model       string             `json:"model"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	System      string             `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicUsage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens,omitempty"`
	PromptCacheHitTokens     *int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens    *int `json:"prompt_cache_miss_tokens,omitempty"`
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := "./fixtures/requests/" + name + ".json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

func loadOpenAIScript(t *testing.T, name string) *openAIScript {
	t.Helper()
	var script openAIScript
	if err := sonic.Unmarshal(loadFixture(t, name), &script); err != nil {
		t.Fatalf("failed to unmarshal openai script %s: %v", name, err)
	}
	return &script
}

func loadAnthropicScript(t *testing.T, name string) *anthropicScript {
	t.Helper()
	var script anthropicScript
	if err := sonic.Unmarshal(loadFixture(t, name), &script); err != nil {
		t.Fatalf("failed to unmarshal anthropic script %s: %v", name, err)
	}
	return &script
}

func mustE2EEnv(t *testing.T) (string, string) {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

func postE2E(t *testing.T, baseURL, apiKey, path string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	return resp
}

func TestDeepSeekCacheHitRate_OpenAI_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)
	script := loadOpenAIScript(t, "openai_multi_turn_non_stream")

	warm := runOpenAIScript(t, baseURL, apiKey, script)
	measured := runOpenAIScript(t, baseURL, apiKey, script)

	assertMeasuredCacheHit(t, "openai non-stream", warm, measured)
}

func TestDeepSeekCacheHitRate_OpenAI_Stream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)
	script := loadOpenAIScript(t, "openai_multi_turn_stream")

	warm := runOpenAIScript(t, baseURL, apiKey, script)
	measured := runOpenAIScript(t, baseURL, apiKey, script)

	assertMeasuredCacheHit(t, "openai stream", warm, measured)
}

func TestDeepSeekCacheHitRate_Anthropic_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)
	script := loadAnthropicScript(t, "anthropic_multi_turn_non_stream")

	warm := runAnthropicScript(t, baseURL, apiKey, script)
	measured := runAnthropicScript(t, baseURL, apiKey, script)

	assertMeasuredCacheHit(t, "anthropic non-stream", warm, measured)
}

func TestDeepSeekCacheHitRate_Anthropic_Stream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)
	script := loadAnthropicScript(t, "anthropic_multi_turn_stream")

	warm := runAnthropicScript(t, baseURL, apiKey, script)
	measured := runAnthropicScript(t, baseURL, apiKey, script)

	assertMeasuredCacheHit(t, "anthropic stream", warm, measured)
}

func runOpenAIScript(t *testing.T, baseURL, apiKey string, script *openAIScript) cacheStats {
	t.Helper()
	conversation, userTurns := splitOpenAIScript(t, script)
	var total cacheStats
	for i, userText := range userTurns {
		messages := append([]openAIMessage{}, conversation...)
		messages = append(messages, openAIMessage{Role: "user", Content: userText})
		req := openAIRequest{
			Model:         script.Model,
			Stream:        script.Stream,
			StreamOptions: script.StreamOptions,
			MaxTokens:     script.MaxTokens,
			Temperature:   script.Temperature,
			Messages:      messages,
		}
		if req.Stream && req.StreamOptions == nil {
			req.StreamOptions = &streamOptions{IncludeUsage: true}
		}
		body, err := sonic.Marshal(&req)
		if err != nil {
			t.Fatalf("failed to marshal openai turn %d: %v", i+1, err)
		}
		assistantText, stats := callOpenAITurn(t, baseURL, apiKey, req.Stream, body)
		total = total.Add(stats)
		conversation = append(messages, openAIMessage{Role: "assistant", Content: assistantText})
	}
	return total
}

func splitOpenAIScript(t *testing.T, script *openAIScript) ([]openAIMessage, []string) {
	t.Helper()
	if script.Model == "" {
		t.Fatal("openai script missing model")
	}
	var systemMessages []openAIMessage
	var userTurns []string
	for i, msg := range script.Messages {
		switch msg.Role {
		case "system":
			systemMessages = append(systemMessages, msg)
		case "user":
			if strings.TrimSpace(msg.Content) == "" {
				t.Fatalf("openai script user message[%d] is empty", i)
			}
			userTurns = append(userTurns, msg.Content)
		default:
			t.Fatalf("openai script message[%d] role=%q, want only system or user; assistant history must be generated by API", i, msg.Role)
		}
	}
	if len(systemMessages) == 0 {
		t.Fatal("openai script must include at least one system message")
	}
	if len(userTurns) < 2 {
		t.Fatalf("openai script must include multiple user turns, got %d", len(userTurns))
	}
	return systemMessages, userTurns
}

func callOpenAITurn(t *testing.T, baseURL, apiKey string, stream bool, body []byte) (string, cacheStats) {
	t.Helper()
	if stream {
		return callOpenAIStream(t, baseURL, apiKey, body)
	}
	return callOpenAINonStream(t, baseURL, apiKey, body)
}

func callOpenAINonStream(t *testing.T, baseURL, apiKey string, body []byte) (string, cacheStats) {
	t.Helper()
	resp := postE2E(t, baseURL, apiKey, "/api/openai/v1/chat/completions", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !sonic.ValidString(string(respBody)) {
		t.Fatalf("response is not valid JSON: %s", string(respBody))
	}

	var obj struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *openAIUsage `json:"usage"`
	}
	if err := sonic.Unmarshal(respBody, &obj); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if strings.TrimSpace(obj.ID) == "" {
		t.Errorf("missing or empty id")
	}
	if strings.TrimSpace(obj.Model) == "" {
		t.Errorf("missing or empty model")
	}
	if len(obj.Choices) == 0 {
		t.Fatalf("missing choices in response: %s", string(respBody))
	}
	if obj.Usage == nil {
		t.Fatalf("missing usage in response: %s", string(respBody))
	}
	assistantText := obj.Choices[0].Message.Content
	if assistantText == "" {
		assistantText = obj.Choices[0].Message.ReasoningContent
	}
	if strings.TrimSpace(assistantText) == "" {
		t.Fatalf("empty assistant message in response: %s", string(respBody))
	}
	return assistantText, openAICacheStats(obj.Usage)
}

func callOpenAIStream(t *testing.T, baseURL, apiKey string, body []byte) (string, cacheStats) {
	t.Helper()
	resp := postE2E(t, baseURL, apiKey, "/api/openai/v1/chat/completions", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(respBody))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected Content-Type = %q, want text/event-stream", ct)
	}
	traceID := resp.Header.Get(constant.HTTPTitleHeaderTraceID)
	if traceID == "" {
		t.Errorf("missing X-Trace-Id header in stream response")
	}

	deadline := time.Now().Add(e2eStreamReadDeadline)
	reader := bufio.NewReader(resp.Body)
	var content strings.Builder
	var reasoning strings.Builder
	var lastUsage *openAIUsage
	var dataLines int
	for {
		if time.Now().After(deadline) {
			t.Fatalf("stream read deadline exceeded after %s (traceID=%s, data_lines=%d)", e2eStreamReadDeadline, traceID, dataLines)
		}
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "data: ") {
			dataLines++
			payload := strings.TrimPrefix(trimmed, "data: ")
			if payload == "[DONE]" {
				break
			}
			usage, hasUsage, contentDelta, reasoningDelta := parseOpenAIStreamPayload(payload)
			if hasUsage {
				lastUsage = usage
			}
			content.WriteString(contentDelta)
			reasoning.WriteString(reasoningDelta)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("failed to read SSE stream (traceID=%s): %v", traceID, readErr)
		}
	}

	assistantText := content.String()
	if assistantText == "" {
		assistantText = reasoning.String()
	}
	if strings.TrimSpace(assistantText) == "" {
		t.Fatalf("stream ended without assistant text (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	if lastUsage == nil {
		t.Fatalf("no usage chunk observed in stream (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	return assistantText, openAICacheStats(lastUsage)
}

func runAnthropicScript(t *testing.T, baseURL, apiKey string, script *anthropicScript) cacheStats {
	t.Helper()
	conversation, userTurns := splitAnthropicScript(t, script)
	var total cacheStats
	for i, userText := range userTurns {
		messages := append([]anthropicMessage{}, conversation...)
		messages = append(messages, anthropicMessage{Role: "user", Content: userText})
		req := anthropicRequest{
			Model:       script.Model,
			Stream:      script.Stream,
			MaxTokens:   script.MaxTokens,
			Temperature: script.Temperature,
			System:      script.System,
			Messages:    messages,
		}
		body, err := sonic.Marshal(&req)
		if err != nil {
			t.Fatalf("failed to marshal anthropic turn %d: %v", i+1, err)
		}
		assistantText, stats := callAnthropicTurn(t, baseURL, apiKey, req.Stream, body)
		total = total.Add(stats)
		conversation = append(messages, anthropicMessage{Role: "assistant", Content: assistantText})
	}
	return total
}

func splitAnthropicScript(t *testing.T, script *anthropicScript) ([]anthropicMessage, []string) {
	t.Helper()
	if script.Model == "" {
		t.Fatal("anthropic script missing model")
	}
	if strings.TrimSpace(script.System) == "" {
		t.Fatal("anthropic script must include system")
	}
	var userTurns []string
	for i, msg := range script.Messages {
		if msg.Role != "user" {
			t.Fatalf("anthropic script message[%d] role=%q, want only user; assistant history must be generated by API", i, msg.Role)
		}
		if strings.TrimSpace(msg.Content) == "" {
			t.Fatalf("anthropic script user message[%d] is empty", i)
		}
		userTurns = append(userTurns, msg.Content)
	}
	if len(userTurns) < 2 {
		t.Fatalf("anthropic script must include multiple user turns, got %d", len(userTurns))
	}
	return nil, userTurns
}

func callAnthropicTurn(t *testing.T, baseURL, apiKey string, stream bool, body []byte) (string, cacheStats) {
	t.Helper()
	if stream {
		return callAnthropicStream(t, baseURL, apiKey, body)
	}
	return callAnthropicNonStream(t, baseURL, apiKey, body)
}

func callAnthropicNonStream(t *testing.T, baseURL, apiKey string, body []byte) (string, cacheStats) {
	t.Helper()
	resp := postE2E(t, baseURL, apiKey, "/api/anthropic/v1/messages", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !sonic.ValidString(string(respBody)) {
		t.Fatalf("response is not valid JSON: %s", string(respBody))
	}

	var obj struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content"`
		Usage *anthropicUsage `json:"usage"`
	}
	if err := sonic.Unmarshal(respBody, &obj); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if strings.TrimSpace(obj.ID) == "" {
		t.Errorf("missing or empty id")
	}
	if strings.TrimSpace(obj.Type) == "" {
		t.Errorf("missing or empty type")
	}
	if strings.TrimSpace(obj.Role) == "" {
		t.Errorf("missing or empty role")
	}
	if strings.TrimSpace(obj.Model) == "" {
		t.Errorf("missing or empty model")
	}
	if len(obj.Content) == 0 {
		t.Fatalf("missing content in response: %s", string(respBody))
	}
	if obj.Usage == nil {
		t.Fatalf("missing usage in response: %s", string(respBody))
	}
	assistantText := collectAnthropicContent(obj.Content)
	if strings.TrimSpace(assistantText) == "" {
		t.Fatalf("empty assistant message in response: %s", string(respBody))
	}
	return assistantText, anthropicCacheStats(obj.Usage)
}

func callAnthropicStream(t *testing.T, baseURL, apiKey string, body []byte) (string, cacheStats) {
	t.Helper()
	resp := postE2E(t, baseURL, apiKey, "/api/anthropic/v1/messages", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(respBody))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected Content-Type = %q, want text/event-stream", ct)
	}
	traceID := resp.Header.Get(constant.HTTPTitleHeaderTraceID)
	if traceID == "" {
		t.Errorf("missing X-Trace-Id header in stream response")
	}

	deadline := time.Now().Add(e2eStreamReadDeadline)
	reader := bufio.NewReader(resp.Body)
	var content strings.Builder
	var reasoning strings.Builder
	var currentEvent string
	var dataLines int
	var gotStart bool
	var gotStop bool
	var lastUsage *anthropicUsage
	for {
		if time.Now().After(deadline) {
			t.Fatalf("stream read deadline exceeded after %s (traceID=%s, data_lines=%d)", e2eStreamReadDeadline, traceID, dataLines)
		}
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if eventType, ok := strings.CutPrefix(trimmed, "event: "); ok {
			currentEvent = eventType
			if eventType == "message_start" {
				gotStart = true
			}
			if eventType == "message_stop" {
				gotStop = true
			}
		}
		if payload, ok := strings.CutPrefix(trimmed, "data: "); ok {
			dataLines++
			usage, hasUsage, contentDelta, reasoningDelta := parseAnthropicStreamPayload(currentEvent, payload)
			if hasUsage {
				lastUsage = usage
			}
			content.WriteString(contentDelta)
			reasoning.WriteString(reasoningDelta)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("failed to read SSE stream (traceID=%s): %v", traceID, readErr)
		}
	}

	if !gotStart {
		t.Fatalf("stream ended without message_start event (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	if !gotStop {
		t.Fatalf("stream ended without message_stop event (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	assistantText := content.String()
	if assistantText == "" {
		assistantText = reasoning.String()
	}
	if strings.TrimSpace(assistantText) == "" {
		t.Fatalf("stream ended without assistant text (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	if lastUsage == nil {
		t.Fatalf("no usage event observed in stream (traceID=%s, data_lines=%d)", traceID, dataLines)
	}
	return assistantText, anthropicCacheStats(lastUsage)
}

func parseOpenAIStreamPayload(payload string) (*openAIUsage, bool, string, string) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *openAIUsage `json:"usage"`
	}
	if err := sonic.UnmarshalString(payload, &chunk); err != nil {
		return nil, false, "", ""
	}
	var content strings.Builder
	var reasoning strings.Builder
	for _, choice := range chunk.Choices {
		content.WriteString(choice.Delta.Content)
		reasoning.WriteString(choice.Delta.ReasoningContent)
	}
	return chunk.Usage, chunk.Usage != nil, content.String(), reasoning.String()
}

func parseAnthropicStreamPayload(eventType, payload string) (*anthropicUsage, bool, string, string) {
	var event struct {
		Message *struct {
			Usage *anthropicUsage `json:"usage"`
		} `json:"message"`
		Delta *struct {
			Text        string `json:"text"`
			Thinking    string `json:"thinking"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
		Usage *anthropicUsage `json:"usage"`
	}
	if err := sonic.UnmarshalString(payload, &event); err != nil {
		return nil, false, "", ""
	}
	var usage *anthropicUsage
	if event.Usage != nil {
		usage = event.Usage
	}
	if event.Message != nil && event.Message.Usage != nil {
		usage = event.Message.Usage
	}
	if eventType != "content_block_delta" || event.Delta == nil {
		return usage, usage != nil, "", ""
	}
	return usage, usage != nil, event.Delta.Text + event.Delta.PartialJSON, event.Delta.Thinking
}

func collectAnthropicContent(blocks []struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
}) string {
	var content strings.Builder
	var reasoning strings.Builder
	for _, block := range blocks {
		content.WriteString(block.Text)
		reasoning.WriteString(block.Thinking)
	}
	if content.String() != "" {
		return content.String()
	}
	return reasoning.String()
}

func openAICacheStats(usage *openAIUsage) cacheStats {
	stats := cacheStats{InputTokens: usage.PromptTokens}
	if usage.PromptCacheHitTokens != nil {
		stats.HitTokens = *usage.PromptCacheHitTokens
		stats.Explicit = true
	} else if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens != nil {
		stats.HitTokens = *usage.PromptTokensDetails.CachedTokens
		stats.Explicit = true
	}
	if usage.PromptCacheMissTokens != nil {
		stats.MissTokens = *usage.PromptCacheMissTokens
		stats.Explicit = true
	} else if stats.InputTokens > stats.HitTokens {
		stats.MissTokens = stats.InputTokens - stats.HitTokens
	}
	return stats
}

func anthropicCacheStats(usage *anthropicUsage) cacheStats {
	stats := cacheStats{InputTokens: usage.InputTokens}
	if usage.CacheReadInputTokens != nil {
		stats.HitTokens = *usage.CacheReadInputTokens
		stats.Explicit = true
	}
	if usage.PromptCacheHitTokens != nil {
		stats.HitTokens = *usage.PromptCacheHitTokens
		stats.Explicit = true
	}
	if usage.PromptCacheMissTokens != nil {
		stats.MissTokens = *usage.PromptCacheMissTokens
		stats.Explicit = true
	} else if stats.InputTokens > stats.HitTokens {
		stats.MissTokens = stats.InputTokens - stats.HitTokens
	}
	return stats
}

func assertMeasuredCacheHit(t *testing.T, name string, warm, measured cacheStats) {
	t.Helper()
	if !measured.Explicit {
		t.Fatalf("%s cache usage fields are missing: warm=%+v measured=%+v", name, warm, measured)
	}
	if measured.HitTokens <= 0 {
		t.Fatalf("%s cache was not hit: warm=%+v measured=%+v warm_rate=%.4f measured_rate=%.4f",
			name, warm, measured, warm.HitRate(), measured.HitRate())
	}
	t.Logf("%s cache hit rate: warm=%.4f (%d/%d), measured=%.4f (%d/%d)",
		name,
		warm.HitRate(), warm.HitTokens, warm.HitTokens+warm.MissTokens,
		measured.HitRate(), measured.HitTokens, measured.HitTokens+measured.MissTokens,
	)
}
