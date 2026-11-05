package message_checksum

import (
	"testing"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
)

// goldenCase 不含托管图片的消息：checksum 必须与图片外置改动前逐字节一致
type goldenCase struct {
	name    string
	msg     *vo.UnifiedMessage
	modelID string
	want    string
}

func goldenCases() []goldenCase {
	return []goldenCase{
		{
			name: "plain text",
			msg:  &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Text: "hello"}},
			want: "7f618c3fa8a1306f348ac77251adbab6cf29f6d0dddcba308ecf35a41d670b9a",
		},
		{
			name: "external url image parts",
			msg: &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Parts: []*vo.UnifiedContentPart{
				{Type: enum.ContentPartTypeText, Text: "look"},
				{Type: enum.ContentPartTypeImageURL, ImageURL: "https://example.com/a.png", ImageDetail: "high"},
			}}},
			want: "f504f5d8144e661faac0212a8b19fa1ba35a99aadbdc6c6d6d44ebf08fae30d1",
		},
		{
			name: "tool result",
			msg:  &vo.UnifiedMessage{Role: enum.RoleTool, ToolCallID: "call_1", Content: &vo.UnifiedContent{Text: "ok"}},
			want: "ba06a9d1b3f7c6be2065dbfac5b2f9f2ce109012110accdbf52be26c13048337",
		},
		{
			name: "assistant tool calls",
			msg: &vo.UnifiedMessage{Role: enum.RoleAssistant, ToolCalls: []*vo.UnifiedToolCall{
				{ID: "call_1", Name: "search", Arguments: `{"q":"go"}`},
			}},
			modelID: "gpt-4",
			want:    "a2eb4d08906ac2ddefce1207b92dda616f2765cf4a79849209ab601952c9c46e",
		},
	}
}

func TestChecksumGoldenUnchanged(t *testing.T) {
	t.Parallel()
	for _, tc := range goldenCases() {
		got := vo.ComputeMessageChecksum(tc.msg, tc.modelID, nil)
		if got != tc.want {
			t.Errorf("%s: checksum = %q, want %q", tc.name, got, tc.want)
		}
	}
}
