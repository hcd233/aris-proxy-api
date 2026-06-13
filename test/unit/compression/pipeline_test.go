package compression

import (
	"context"
	"testing"

	compression "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestPipelineCompressToolMessages(t *testing.T) {
	t.Parallel()
	cfg := compression.DefaultPipelineConfig()
	p := compression.NewPipeline(cfg)

	messages := []compression.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "query"},
		{Role: "tool", Content: `[{"id":1,"name":"a","value":10},{"id":2,"name":"b","value":10},{"id":3,"name":"c","value":10},{"id":4,"name":"d","value":10},{"id":5,"name":"e","value":10},{"id":6,"name":"f","value":10},{"id":7,"name":"g","value":10},{"id":8,"name":"h","value":10},{"id":9,"name":"i","value":10},{"id":10,"name":"j","value":"end"}]`},
	}

	result, res := p.Compress(context.Background(), messages)
	if res == nil {
		t.Fatal("result should not be nil")
	}
	t.Logf("tokens before: %d, after: %d", res.TokensBefore, res.TokensAfter)
	t.Logf("strategies: %v", res.Strategies)
	_ = result
}

func TestPipelineCompressToolCallsContentList(t *testing.T) {
	t.Parallel()
	cfg := compression.DefaultPipelineConfig()
	p := compression.NewPipeline(cfg)

	messages := []compression.Message{
		{Role: "user", Content: "query"},
		{Role: "tool", Content: `[{"id":1,"name":"a"},{"id":2,"name":"a"},{"id":3,"name":"a"},{"id":4,"name":"a"},{"id":5,"name":"a"},{"id":6,"name":"b"}]`},
	}

	result, _ := p.Compress(context.Background(), messages)
	for _, msg := range result {
		if msg.Role == "tool" {
			t.Logf("compressed tool content: %s", msg.Content)
		}
	}
}

func TestPipelineSkipsSystemUserAssistant(t *testing.T) {
	t.Parallel()
	cfg := compression.DefaultPipelineConfig()
	p := compression.NewPipeline(cfg)

	original := []compression.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "user query"},
		{Role: "assistant", Content: "assistant response"},
	}

	result, _ := p.Compress(context.Background(), original)
	for i, msg := range result {
		if msg.Content != original[i].Content {
			t.Errorf("message %d changed unexpectedly: %s", i, msg.Content)
		}
	}
}

func TestPipelineEmptyMessages(t *testing.T) {
	t.Parallel()
	cfg := compression.DefaultPipelineConfig()
	p := compression.NewPipeline(cfg)

	result, res := p.Compress(context.Background(), nil)
	if len(result) != 0 {
		t.Error("result should be empty")
	}
	if res == nil {
		t.Fatal("result summary should not be nil")
	}
}

func TestPipelineNoopPipeline(t *testing.T) {
	t.Parallel()
	p := compression.NewNoopPipeline()
	original := []compression.Message{{Role: "tool", Content: "unchanged"}}
	result, _ := p.Compress(context.Background(), original)
	if result[0].Content != "unchanged" {
		t.Error("noop should not modify content")
	}
}
