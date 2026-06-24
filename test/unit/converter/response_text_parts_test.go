package converter

import (
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestResponseProtocolConverter_FromResponseRequest_TextPartsNotJoined 验证
// 当 Response API message item 的 content 含多个纯文本 part（无图片）时，
// 转换为 ChatCompletion 后应保留 Parts 数组，而非用 "\n" 拼接为单个 Text 字符串。
func TestResponseProtocolConverter_FromResponseRequest_TextPartsNotJoined(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}

	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("gpt-5.4"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role: lo.ToPtr(enum.RoleUser),
					Content: &dto.ResponseInputMessageContent{
						Parts: []*dto.ResponseInputContent{
							{
								Type: enum.ResponseContentTypeInputText,
								Text: lo.ToPtr("first paragraph"),
							},
							{
								Type: enum.ResponseContentTypeInputText,
								Text: lo.ToPtr("second paragraph"),
							},
						},
					},
				},
			},
		},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	if len(chatReq.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(chatReq.Messages))
	}

	msg := chatReq.Messages[0]
	if msg.Content == nil {
		t.Fatal("Content should not be nil")
	}

	if len(msg.Content.Parts) != 2 {
		t.Fatalf("len(Parts) = %d, want 2 (parts should be preserved, not joined)", len(msg.Content.Parts))
	}

	if msg.Content.Parts[0].Type != enum.ContentPartTypeText || msg.Content.Parts[0].Text == nil || *msg.Content.Parts[0].Text != "first paragraph" {
		t.Errorf("Parts[0] mismatch: type=%q text=%v", msg.Content.Parts[0].Type, msg.Content.Parts[0].Text)
	}
	if msg.Content.Parts[1].Type != enum.ContentPartTypeText || msg.Content.Parts[1].Text == nil || *msg.Content.Parts[1].Text != "second paragraph" {
		t.Errorf("Parts[1] mismatch: type=%q text=%v", msg.Content.Parts[1].Type, msg.Content.Parts[1].Text)
	}

	if msg.Content.Text != "" {
		t.Errorf("Text should be empty when Parts are preserved, got %q", msg.Content.Text)
	}
}
