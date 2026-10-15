package transport_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestBuildChatStream_Shape 验证流式 chunk 序列：首片带 role、中间带 content、末片带 stop+usage。
func TestBuildChatStream_Shape(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefghijklmno")} // 15 字符 → 4 片
	chunks := transport.BuildChatStream(spec, "mock-chat", "chunk-1")

	if len(chunks) != 5 { // 4 内容片 + 1 结束片
		t.Fatalf("want 5 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Model != "mock-chat" || c.ID != "chunk-1" {
			t.Fatalf("chunk %d meta mismatch: %+v", i, c)
		}
		if len(c.Choices) != 1 {
			t.Fatalf("chunk %d want 1 choice, got %d", i, len(c.Choices))
		}
	}
	if chunks[0].Choices[0].Delta.Role != enum.RoleAssistant {
		t.Fatal("first chunk must carry assistant role")
	}
	last := chunks[len(chunks)-1]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != enum.FinishReasonStop {
		t.Fatal("last chunk finish_reason want stop")
	}
	if last.Usage == nil {
		t.Fatal("last chunk must carry usage")
	}
	if last.Usage.CompletionTokens != 3 {
		t.Fatalf("completion_tokens want 3, got %d", last.Usage.CompletionTokens)
	}
	if last.Usage.TotalTokens != last.Usage.PromptTokens+last.Usage.CompletionTokens {
		t.Fatal("total_tokens must equal prompt+completion")
	}
}

// TestBuildChatCompletion_NonStream 验证非流式响应：单 choice、content 正确、finish_reason=stop。
func TestBuildChatCompletion_NonStream(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 token
	rsp := transport.BuildChatCompletion(spec, "mock-chat")
	if rsp == nil || len(rsp.Choices) != 1 {
		t.Fatalf("want 1 choice, got %+v", rsp)
	}
	if rsp.Choices[0].Message == nil || rsp.Choices[0].Message.Content == nil {
		t.Fatal("choice message content missing")
	}
	if rsp.Choices[0].Message.Content.Text != "abcdefgh" {
		t.Fatalf("content mismatch: %q", rsp.Choices[0].Message.Content.Text)
	}
	if rsp.Choices[0].FinishReason != enum.FinishReasonStop {
		t.Fatalf("finish_reason want stop, got %s", rsp.Choices[0].FinishReason)
	}
	if rsp.Usage == nil || rsp.Usage.CompletionTokens != 2 {
		t.Fatalf("usage mismatch: %+v", rsp.Usage)
	}
}

// TestBuildChatUsage_IncludesInputTokens 验证 usage 带输入维（来自 Spec.InputTokens），
// 满足 spec「input_tokens 从请求 body 估算」的要求，避免审计入账恒为 0。
func TestBuildChatUsage_IncludesInputTokens(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh"), InputTokens: 7} // 8 字符 → 2 output
	got := transport.BuildChatUsage(spec)
	if got.PromptTokens != 7 {
		t.Fatalf("prompt_tokens want 7, got %d", got.PromptTokens)
	}
	if got.CompletionTokens != 2 || got.TotalTokens != 9 {
		t.Fatalf("usage mismatch: %+v", got)
	}
}

// TestBuildResponseUsage_IncludesInputTokens 同上，覆盖 Responses 协议。
func TestBuildResponseUsage_IncludesInputTokens(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh"), InputTokens: 7}
	rsp := transport.BuildResponse(spec, "mock-response")
	if rsp.Usage.InputTokens != 7 {
		t.Fatalf("input_tokens want 7, got %d", rsp.Usage.InputTokens)
	}
	if rsp.Usage.TotalTokens != 9 {
		t.Fatalf("total_tokens want 9, got %d", rsp.Usage.TotalTokens)
	}
}

// TestBuildChatUsage_Bounds 验证 usage 估算口径：1 token≈4 字符，至少 1。
func TestBuildChatUsage_Bounds(t *testing.T) {
	t.Parallel()
	got := transport.BuildChatUsage(transport.MockSpec{Chars: []byte("ab")})
	if got.CompletionTokens != 1 {
		t.Fatalf("short input must yield 1 token, got %d", got.CompletionTokens)
	}
	long := transport.MockSpec{Chars: make([]byte, 40)}
	if n := transport.BuildChatUsage(long).CompletionTokens; n != 10 {
		t.Fatalf("want 10 tokens for 40 chars, got %d", n)
	}
}
