package model_body

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// 这组测试守护"上游请求体字节稳定"——这是上游 prompt cache 命中的硬前提。
//
// 背景：sonic 默认 ConfigDefault.SortMapKeys=false，对 map[string]X 序列化时
// 顺序由 Go runtime map 迭代决定，每次都不同。
// JSONSchemaProperty.Properties 是 map[string]*JSONSchemaProperty，
// 几乎每个工具的 parameters 都会用到。如果上游每轮收到的同一份请求 tools 字节都不同，
// prompt cache 会从工具段开始全部失效。
//
// 这些测试用一个含 8 个 properties 的 tool 触发 sonic 默认 Marshal 的 map 顺序漂移，
// 然后断言连续 N 次 Marshal 字节级相同。

const toolsRichRequestJSON = `{
  "model": "exposed-model",
  "messages": [{"role":"user","content":"run echo hi"}],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "bash",
        "description": "Execute a bash command and return stdout/stderr",
        "parameters": {
          "type": "object",
          "properties": {
            "command":     {"type":"string","description":"The command to execute"},
            "timeout":     {"type":"integer","description":"Optional timeout in seconds"},
            "workdir":     {"type":"string","description":"Working directory"},
            "description": {"type":"string","description":"Short description"},
            "shell":       {"type":"string","description":"Shell to use"},
            "env":         {"type":"object","description":"Environment variables"},
            "input":       {"type":"string","description":"Stdin"},
            "truncate":    {"type":"integer","description":"Truncate output above N bytes"}
          }
        }
      }
    }
  ]
}`

const anthropicToolsRichRequestJSON = `{
  "model": "exposed-anthropic",
  "max_tokens": 1024,
  "messages": [{"role":"user","content":"run echo hi"}],
  "tools": [
    {
      "name": "bash",
      "description": "Execute a bash command",
      "input_schema": {
        "type": "object",
        "properties": {
          "command":     {"type":"string","description":"The command to execute"},
          "timeout":     {"type":"integer","description":"Optional timeout in seconds"},
          "workdir":     {"type":"string","description":"Working directory"},
          "description": {"type":"string","description":"Short description"},
          "shell":       {"type":"string","description":"Shell to use"},
          "env":         {"type":"object","description":"Environment variables"},
          "input":       {"type":"string","description":"Stdin"},
          "truncate":    {"type":"integer","description":"Truncate output above N bytes"}
        }
      }
    }
  ]
}`

func md5short(b []byte) string {
	s := md5.Sum(b)
	return hex.EncodeToString(s[:])
}

// 用 JSON round-trip 把字面量解到 DTO 上（含 map[string]*JSONSchemaProperty）
func loadOpenAIChatReq(t *testing.T) *dto.OpenAIChatCompletionReq {
	t.Helper()
	req := &dto.OpenAIChatCompletionReq{}
	if err := sonic.Unmarshal([]byte(toolsRichRequestJSON), req); err != nil {
		t.Fatalf("setup: unmarshal openai chat req: %v", err)
	}
	if len(req.Tools) == 0 {
		t.Fatalf("setup: tools must not be empty after unmarshal")
	}
	return req
}

func loadAnthropicMessageReq(t *testing.T) *dto.AnthropicCreateMessageReq {
	t.Helper()
	req := &dto.AnthropicCreateMessageReq{}
	if err := sonic.Unmarshal([]byte(anthropicToolsRichRequestJSON), req); err != nil {
		t.Fatalf("setup: unmarshal anthropic msg req: %v", err)
	}
	if len(req.Tools) == 0 {
		t.Fatalf("setup: anthropic tools must not be empty after unmarshal")
	}
	return req
}

// TestMarshalOpenAIChatCompletionBodyForModel_BytesStable
// 守护 OpenAI ChatCompletion 上游请求体字节级稳定。
func TestMarshalOpenAIChatCompletionBodyForModel_BytesStable(t *testing.T) {
	req := loadOpenAIChatReq(t)

	const rounds = 32
	first := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")
	firstHash := md5short(first)

	for i := 1; i < rounds; i++ {
		next := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")
		if md5short(next) != firstHash {
			t.Fatalf("round %d: marshal output drifted, expected md5=%s got md5=%s\nfirst=%s\nnext =%s",
				i, firstHash, md5short(next), string(first), string(next))
		}
	}
}

// TestMarshalRawOpenAIChatCompletionBodyForModel_BytesStable
// 守护 OpenAI ChatCompletion 透传路径上游请求体字节级稳定。
// 该路径内部用 map[string]sonic.NoCopyRawMessage 重组顶层字段，
// 即使每个 value 是 RawMessage 透传，map 的顶层 key 顺序在 sonic 默认配置下也会乱。
func TestMarshalRawOpenAIChatCompletionBodyForModel_BytesStable(t *testing.T) {
	req := loadOpenAIChatReq(t)
	raw := []byte(toolsRichRequestJSON)

	const rounds = 32
	first := proxyutil.MarshalRawOpenAIChatCompletionBodyForModel(raw, req, "upstream-model")
	firstHash := md5short(first)

	for i := 1; i < rounds; i++ {
		next := proxyutil.MarshalRawOpenAIChatCompletionBodyForModel(raw, req, "upstream-model")
		if md5short(next) != firstHash {
			t.Fatalf("round %d: marshal output drifted, expected md5=%s got md5=%s\nfirst=%s\nnext =%s",
				i, firstHash, md5short(next), string(first), string(next))
		}
	}
}

// TestMarshalAnthropicMessageBodyForModel_BytesStable Anthropic 同等检查。
func TestMarshalAnthropicMessageBodyForModel_BytesStable(t *testing.T) {
	req := loadAnthropicMessageReq(t)

	const rounds = 32
	first := proxyutil.MarshalAnthropicMessageBodyForModel(req, "upstream-anthropic-model")
	firstHash := md5short(first)

	for i := 1; i < rounds; i++ {
		next := proxyutil.MarshalAnthropicMessageBodyForModel(req, "upstream-anthropic-model")
		if md5short(next) != firstHash {
			t.Fatalf("round %d: marshal output drifted, expected md5=%s got md5=%s\nfirst=%s\nnext =%s",
				i, firstHash, md5short(next), string(first), string(next))
		}
	}
}

// TestReplaceModelInBody_BytesStable
// ReplaceModelInBody 把 body 解到 map[string]any 再 Marshal，
// 默认 sonic 配置下顺序也会乱，必须保证字节稳定。
func TestReplaceModelInBody_BytesStable(t *testing.T) {
	body := []byte(toolsRichRequestJSON)

	const rounds = 32
	first := proxyutil.ReplaceModelInBody(body, "upstream-model")
	firstHash := md5short(first)

	for i := 1; i < rounds; i++ {
		next := proxyutil.ReplaceModelInBody(body, "upstream-model")
		if md5short(next) != firstHash {
			t.Fatalf("round %d: ReplaceModelInBody drifted, expected md5=%s got md5=%s",
				i, firstHash, md5short(next))
		}
	}
}

// TestMarshalOpenAIChatCompletionBodyForModel_PropertiesKeySorted
// 进一步守护：tools.parameters.properties 的子 key 必须按字典序输出，
// 这样 client 提交相同 properties（不论内部 map 迭代顺序）都能命中上游 prompt cache。
func TestMarshalOpenAIChatCompletionBodyForModel_PropertiesKeySorted(t *testing.T) {
	req := loadOpenAIChatReq(t)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")
	bodyStr := string(body)

	// 定位 properties 子对象（取最内层 properties 段）
	propStart := strings.Index(bodyStr, `"properties":{`)
	if propStart < 0 {
		t.Fatalf("expected properties section in body, got: %s", bodyStr)
	}
	// 截取 properties 起始之后的字节做顺序检查
	tail := bodyStr[propStart:]

	wantOrder := []string{
		`"command"`, `"description"`, `"env"`, `"input"`, `"shell"`, `"timeout"`, `"truncate"`, `"workdir"`,
	}
	prev := -1
	for _, key := range wantOrder {
		idx := strings.Index(tail, key)
		if idx < 0 {
			t.Fatalf("expected key %s in properties section, body=%s", key, bodyStr)
		}
		if idx <= prev {
			t.Fatalf("properties keys out of order: %s appeared before previous key (idx=%d, prev=%d)\nbody=%s",
				key, idx, prev, bodyStr)
		}
		prev = idx
	}
}
