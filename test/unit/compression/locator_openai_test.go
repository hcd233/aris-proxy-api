package compression

import (
	"strconv"
	"strings"
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressOpenAIChat_CompressesToolOutput(t *testing.T) {
	t.Parallel()
	toolCallID := "call_001"
	largeContent := makeLargeJSONArray()
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    &dto.OpenAIMessageContent{Text: largeContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if messages[0].Content.Text == largeContent {
		t.Error("expected message content to be replaced with compressed output")
	}
	if len(stats.Items) == 0 {
		t.Fatal("expected stats.Items to contain per-item results")
	}
	if stats.Items[0].ToolCallID != toolCallID {
		t.Error("expected ToolCallID to be set in result")
	}
	if stats.Items[0].Input != largeContent {
		t.Error("expected Input to contain original content")
	}
}

func TestCompressOpenAIChat_SkipsSmallToolOutput(t *testing.T) {
	t.Parallel()
	toolCallID := "call_002"
	smallContent := "small"
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    &dto.OpenAIMessageContent{Text: smallContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 100)

	if stats.ItemsCompressed != 0 {
		t.Error("expected 0 items compressed for small content")
	}
	if stats.ItemsSkipped != 1 {
		t.Error("expected 1 item skipped")
	}
	if messages[0].Content.Text != smallContent {
		t.Error("expected small content to remain unchanged")
	}
}

func TestCompressOpenAIChat_SkipsNonToolMessages(t *testing.T) {
	t.Parallel()
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:    enum.RoleUser,
			Content: &dto.OpenAIMessageContent{Text: "user message"},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-tool messages")
	}
}

func TestCompressOpenAIChat_NilContentSkipped(t *testing.T) {
	t.Parallel()
	toolCallID := "call_003"
	messages := []*dto.OpenAIChatCompletionMessageParam{
		{
			Role:       enum.RoleTool,
			ToolCallID: &toolCallID,
			Content:    nil,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIChat(messages, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil content")
	}
}

func makeLargeJSONArray() string {
	const count = 20
	var b strings.Builder
	b.WriteByte('[')
	for i := range count {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"id":`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`,"name":"item_`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`","data":"some data here"}`)
	}
	b.WriteByte(']')
	return b.String()
}
