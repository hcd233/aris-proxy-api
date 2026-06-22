package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressAnthropicMessages_CompressesStringContent(t *testing.T) {
	t.Parallel()
	toolUseID := "toolu_001"
	largeContent := makeLargeJSONArray()
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "user",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{
						Type:      constant.CompressionJSONKeyToolResult,
						ToolUseID: &toolUseID,
						Content:   &dto.AnthropicToolResultContent{Text: largeContent},
					},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if messages[0].Content.Blocks[0].Content.Text == largeContent {
		t.Error("expected content to be replaced with compressed output")
	}
	if len(stats.Items) == 0 || stats.Items[0].ToolCallID != toolUseID {
		t.Error("expected ToolCallID to be set in result")
	}
}

func TestCompressAnthropicMessages_CompressesArrayContent(t *testing.T) {
	t.Parallel()
	toolUseID := "toolu_002"
	largeContent := makeLargeJSONArray()
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "user",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{
						Type:      constant.CompressionJSONKeyToolResult,
						ToolUseID: &toolUseID,
						Content: &dto.AnthropicToolResultContent{
							Blocks: []*dto.AnthropicContentBlock{
								{Type: "text", Text: &largeContent},
							},
						},
					},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed for array content")
	}
	if len(stats.Items) == 0 || stats.Items[0].ToolCallID != toolUseID {
		t.Error("expected ToolCallID to be set in result")
	}
}

func TestCompressAnthropicMessages_SkipsNonToolResultBlocks(t *testing.T) {
	t.Parallel()
	originalText := "some text"
	messages := []*dto.AnthropicMessageParam{
		{
			Role: "assistant",
			Content: &dto.AnthropicMessageContent{
				Blocks: []*dto.AnthropicContentBlock{
					{Type: "text", Text: &originalText},
				},
			},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-tool_result blocks")
	}
}

func TestCompressAnthropicMessages_NilContentSkipped(t *testing.T) {
	t.Parallel()
	messages := []*dto.AnthropicMessageParam{
		{Role: "user", Content: nil},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressAnthropicMessages(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil content")
	}
}
