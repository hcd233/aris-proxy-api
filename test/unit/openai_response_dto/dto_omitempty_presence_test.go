package openai_response_dto

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func requireJSONContains(t *testing.T, raw []byte, fragments ...string) {
	t.Helper()
	got := string(raw)
	for _, fragment := range fragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("marshaled JSON missing %s\nraw: %s", fragment, got)
		}
	}
}

func TestDTOExplicitZeroValuesSurviveOmitEmptyRoundTrip(t *testing.T) {
	t.Run("openai chat usage token details", func(t *testing.T) {
		raw := []byte(`{"completion_tokens":0,"prompt_tokens":0,"total_tokens":0,"completion_tokens_details":{"accepted_prediction_tokens":0,"audio_tokens":0,"reasoning_tokens":0,"rejected_prediction_tokens":0},"prompt_tokens_details":{"audio_tokens":0,"cached_tokens":0}}`)
		var usage dto.OpenAICompletionUsage
		if err := sonic.Unmarshal(raw, &usage); err != nil {
			t.Fatalf("unmarshal usage: %v", err)
		}
		got, err := sonic.Marshal(&usage)
		if err != nil {
			t.Fatalf("marshal usage: %v", err)
		}
		requireJSONContains(t, got,
			`"accepted_prediction_tokens":0`,
			`"audio_tokens":0`,
			`"reasoning_tokens":0`,
			`"rejected_prediction_tokens":0`,
			`"cached_tokens":0`,
		)
	})

	t.Run("response annotations and computer actions", func(t *testing.T) {
		raw := []byte(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"","annotations":[{"type":"url_citation","start_index":0,"end_index":0,"title":"","url":"https://example.com"}]}],"action":{"type":"scroll","x":0,"y":0,"scroll_x":0,"scroll_y":0},"actions":[{"type":"click","x":0,"y":0}]}`)
		var item dto.ResponseInputItem
		if err := sonic.Unmarshal(raw, &item); err != nil {
			t.Fatalf("unmarshal response input item: %v", err)
		}
		got, err := sonic.Marshal(&item)
		if err != nil {
			t.Fatalf("marshal response input item: %v", err)
		}
		requireJSONContains(t, got,
			`"text":""`,
			`"start_index":0`,
			`"end_index":0`,
			`"x":0`,
			`"y":0`,
			`"scroll_x":0`,
			`"scroll_y":0`,
		)
	})

	t.Run("anthropic cache token usage", func(t *testing.T) {
		raw := []byte(`{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`)
		var usage dto.AnthropicUsage
		if err := sonic.Unmarshal(raw, &usage); err != nil {
			t.Fatalf("unmarshal anthropic usage: %v", err)
		}
		got, err := sonic.Marshal(&usage)
		if err != nil {
			t.Fatalf("marshal anthropic usage: %v", err)
		}
		requireJSONContains(t, got,
			`"cache_creation_input_tokens":0`,
			`"cache_read_input_tokens":0`,
		)
	})
}
