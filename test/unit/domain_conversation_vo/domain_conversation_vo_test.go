// Package domain_conversation_vo 针对 domain/conversation/vo 包的字节级回归测试
package domain_conversation_vo

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// unifiedContentCase 统一测试用例结构，覆盖 JSON 往返、Parts 序列化优先级、Tool Checksum 等场景
type unifiedContentCase struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	JSON        string                   `json:"json"`
	ExpectText  string                   `json:"expectText"`
	ExpectParts int                      `json:"expectParts"`
	Text        string                   `json:"text"`
	Parts       []*vo.UnifiedContentPart `json:"parts"`
	ToolName    string                   `json:"tool_name"`
	ToolDesc    string                   `json:"tool_description"`
}

func loadUnifiedContentCases(t *testing.T) []unifiedContentCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []unifiedContentCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// TestUnifiedContent_JSONRoundtrip 验证 UnifiedContent 在 string/array 形态下的
// 反序列化与再序列化能保留字节级形态（Text->string / Parts->array）
func TestUnifiedContent_JSONRoundtrip(t *testing.T) {
	cases := loadUnifiedContentCases(t)
	for _, tc := range cases {
		if tc.JSON == "" {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			var content vo.UnifiedContent
			if err := sonic.UnmarshalString(tc.JSON, &content); err != nil {
				t.Fatalf("unmarshal failed for %q: %v", tc.JSON, err)
			}

			if content.Text != tc.ExpectText {
				t.Errorf("Text = %q, want %q", content.Text, tc.ExpectText)
			}
			if len(content.Parts) != tc.ExpectParts {
				t.Errorf("len(Parts) = %d, want %d", len(content.Parts), tc.ExpectParts)
			}

			encoded, err := sonic.Marshal(&content)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var roundtrip vo.UnifiedContent
			if err := sonic.Unmarshal(encoded, &roundtrip); err != nil {
				t.Fatalf("roundtrip unmarshal failed: %v", err)
			}
			if roundtrip.Text != content.Text {
				t.Errorf("roundtrip Text = %q, want %q", roundtrip.Text, content.Text)
			}
			if len(roundtrip.Parts) != len(content.Parts) {
				t.Errorf("roundtrip len(Parts) = %d, want %d", len(roundtrip.Parts), len(content.Parts))
			}
		})
	}
}

// TestUnifiedContent_MarshalParts_Priority Parts 非空时应输出数组而非字符串
//
// 用例数据来自 fixtures/cases.json 中的 marshal_parts_priority
func TestUnifiedContent_MarshalParts_Priority(t *testing.T) {
	allCases := loadUnifiedContentCases(t)
	var tc *unifiedContentCase
	for i := range allCases {
		if allCases[i].Name == "marshal_parts_priority" {
			tc = &allCases[i]
			break
		}
	}
	if tc == nil {
		t.Fatal("test case 'marshal_parts_priority' not found in fixtures")
	}

	content := vo.UnifiedContent{
		Text:  tc.Text,
		Parts: tc.Parts,
	}
	encoded, err := sonic.MarshalString(&content)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if encoded[0] != '[' {
		t.Errorf("expected array output when Parts non-empty, got %s", encoded)
	}
}

// TestComputeToolChecksum_StableOutput 同一 tool 多次计算应得到完全相同的 checksum
//
// 用例数据来自 fixtures/cases.json 中的 compute_tool_checksum_stable
func TestComputeToolChecksum_StableOutput(t *testing.T) {
	allCases := loadUnifiedContentCases(t)
	var tc *unifiedContentCase
	for i := range allCases {
		if allCases[i].Name == "compute_tool_checksum_stable" {
			tc = &allCases[i]
			break
		}
	}
	if tc == nil {
		t.Fatal("test case 'compute_tool_checksum_stable' not found in fixtures")
	}

	tool := &vo.UnifiedTool{
		Name:        tc.ToolName,
		Description: tc.ToolDesc,
	}

	first := vo.ComputeToolChecksum(tool)
	second := vo.ComputeToolChecksum(tool)

	if first != second {
		t.Errorf("ComputeToolChecksum not stable: first=%s second=%s", first, second)
	}
	if len(first) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d (%q)", len(first), first)
	}
}
