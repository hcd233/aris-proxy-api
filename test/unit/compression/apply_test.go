package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

func TestApplyResultsToMessages_MatchesByToolCallID(t *testing.T) {
	t.Parallel()
	rawBefore := "original content"
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "compressed"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID:  "call_001",
			Input:       rawBefore,
			Output:      "compressed",
			Strategy:    "smart_crusher",
			Applied:     true,
			BytesBefore: len(rawBefore),
			BytesAfter:  len("compressed"),
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent == nil || *messages[0].RawContent != rawBefore {
		t.Error("expected RawContent to be set to original content")
	}
	if messages[0].CompressionStrategy != "smart_crusher" {
		t.Error("expected CompressionStrategy to be 'smart_crusher'")
	}
}

func TestApplyResultsToMessages_SkipsUnmatched(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_999",
			Input:      "other",
			Strategy:   "smart_crusher",
			Applied:    true,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil for unmatched message")
	}
	if messages[0].CompressionStrategy != "" {
		t.Error("expected CompressionStrategy to remain empty for unmatched message")
	}
}

func TestApplyResultsToMessages_SkipsNotApplied(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_001",
			Input:      "original",
			Strategy:   "passthrough",
			Applied:    false,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil when Applied=false")
	}
}

func TestApplyResultsToMessages_EmptyResultsNoop(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:       enum.RoleTool,
			ToolCallID: "call_001",
			Content:    &vo.UnifiedContent{Text: "unchanged"},
		},
	}

	comp.ApplyResultsToMessages(messages, nil)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil with empty results")
	}
}

func TestApplyResultsToMessages_SkipsEmptyToolCallID(t *testing.T) {
	t.Parallel()
	messages := []*vo.UnifiedMessage{
		{
			Role:    enum.RoleUser,
			Content: &vo.UnifiedContent{Text: "user message"},
		},
	}
	results := []comp.ItemCompressionResult{
		{
			ToolCallID: "call_001",
			Input:      "original",
			Strategy:   "smart_crusher",
			Applied:    true,
		},
	}

	comp.ApplyResultsToMessages(messages, results)

	if messages[0].RawContent != nil {
		t.Error("expected RawContent to remain nil for message without ToolCallID")
	}
}
