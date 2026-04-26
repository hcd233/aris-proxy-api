package openai_stream_tool_call

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type testCase struct {
	Name        string                           `json:"name"`
	Description string                           `json:"description"`
	Chunks      []*dto.OpenAIChatCompletionChunk `json:"chunks"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return testCase{}
}

func TestNormalizeOpenAIStreamToolCalls_IndexZeroAndFollowupID(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "index_zero_and_followup_id")
	if len(tc.Chunks) != 2 {
		t.Fatalf("len(Chunks) = %d, want 2", len(tc.Chunks))
	}

	toolCallIDs := make(map[int]string)
	for _, chunk := range tc.Chunks {
		util.NormalizeOpenAIStreamToolCalls(chunk, toolCallIDs)
	}

	firstPayload, err := sonic.MarshalString(tc.Chunks[0])
	if err != nil {
		t.Fatalf("failed to marshal first chunk: %v", err)
	}
	if !sonic.ValidString(firstPayload) {
		t.Fatalf("first payload is invalid JSON: %s", firstPayload)
	}
	if !contains(firstPayload, "\"index\":0") {
		t.Errorf("first payload = %s, want index 0", firstPayload)
	}

	followup := tc.Chunks[1].Choices[0].Delta.ToolCalls[0]
	if followup.ID != "call_123" {
		t.Errorf("followup ID = %q, want %q", followup.ID, "call_123")
	}
	followupPayload, err := sonic.MarshalString(tc.Chunks[1])
	if err != nil {
		t.Fatalf("failed to marshal followup chunk: %v", err)
	}
	if !contains(followupPayload, "\"id\":\"call_123\"") {
		t.Errorf("followup payload = %s, want repeated tool call id", followupPayload)
	}
}

func contains(value string, substr string) bool {
	return len(substr) == 0 || len(value) >= len(substr) && containsAt(value, substr)
}

func containsAt(value string, substr string) bool {
	for i := 0; i+len(substr) <= len(value); i++ {
		if value[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
