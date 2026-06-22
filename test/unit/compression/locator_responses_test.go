package compression

import (
	"testing"

	comp "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCompressOpenAIResponses_CompressesFunctionCallOutput(t *testing.T) {
	t.Parallel()
	callID := "call_001"
	itemType := constant.CompressionJSONKeyFuncCallOutput
	largeContent := makeLargeJSONArray()
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			CallID: &callID,
			Output: &dto.ResponseInputItemOutput{Text: largeContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 100)

	if stats.ItemsCompressed == 0 {
		t.Fatal("expected at least 1 item compressed")
	}
	if items[0].Output.Text == largeContent {
		t.Error("expected output to be replaced with compressed content")
	}
	if len(stats.Items) == 0 || stats.Items[0].ToolCallID != callID {
		t.Error("expected ToolCallID to be set in result")
	}
}

func TestCompressOpenAIResponses_SkipsSmallOutput(t *testing.T) {
	t.Parallel()
	callID := "call_002"
	itemType := constant.CompressionJSONKeyFuncCallOutput
	smallContent := "small"
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			CallID: &callID,
			Output: &dto.ResponseInputItemOutput{Text: smallContent},
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 100)

	if stats.ItemsCompressed != 0 {
		t.Error("expected 0 items compressed for small content")
	}
	if stats.ItemsSkipped != 1 {
		t.Error("expected 1 item skipped")
	}
	if items[0].Output.Text != smallContent {
		t.Error("expected small output to remain unchanged")
	}
}

func TestCompressOpenAIResponses_SkipsNonFunctionCallOutput(t *testing.T) {
	t.Parallel()
	msgType := "message"
	items := []*dto.ResponseInputItem{
		{
			Type: &msgType,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for non-function_call_output items")
	}
}

func TestCompressOpenAIResponses_NilOutputSkipped(t *testing.T) {
	t.Parallel()
	itemType := constant.CompressionJSONKeyFuncCallOutput
	items := []*dto.ResponseInputItem{
		{
			Type:   &itemType,
			Output: nil,
		},
	}
	dispatcher := comp.NewDispatcher()

	stats := comp.CompressOpenAIResponses(items, dispatcher, 0)

	if stats.ItemsCompressed != 0 || stats.ItemsSkipped != 0 {
		t.Error("expected no items processed for nil output")
	}
}
