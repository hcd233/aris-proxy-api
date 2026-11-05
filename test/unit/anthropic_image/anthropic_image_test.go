package anthropic_image

import (
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/dto"
)

const (
	pngData    = "iVBORw0KGgo="
	pngDataURI = "data:image/png;base64," + pngData
	imageURL   = "https://example.com/a.png"
)

func base64Image() *dto.AnthropicContentBlock {
	return &dto.AnthropicContentBlock{
		Type: enum.AnthropicContentBlockTypeImage,
		Source: &dto.AnthropicContentSource{
			Type:      enum.ImageSourceTypeBase64,
			MediaType: lo.ToPtr("image/png"),
			Data:      lo.ToPtr(pngData),
		},
	}
}

func urlImage() *dto.AnthropicContentBlock {
	return &dto.AnthropicContentBlock{
		Type:   enum.AnthropicContentBlockTypeImage,
		Source: &dto.AnthropicContentSource{Type: enum.ImageSourceTypeURL, URL: lo.ToPtr(imageURL)},
	}
}

func textBlock(s string) *dto.AnthropicContentBlock {
	return &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: lo.ToPtr(s)}
}

func toolResult(blocks ...*dto.AnthropicContentBlock) *dto.AnthropicContentBlock {
	return &dto.AnthropicContentBlock{
		Type:      enum.AnthropicContentBlockTypeToolResult,
		ToolUseID: lo.ToPtr("toolu_1"),
		Content:   &dto.AnthropicToolResultContent{Blocks: blocks},
	}
}

func convert(t *testing.T, blocks ...*dto.AnthropicContentBlock) *vo.UnifiedMessage {
	t.Helper()
	um, err := dto.FromAnthropicMessage(&dto.AnthropicMessageParam{
		Role:    enum.RoleUser,
		Content: &dto.AnthropicMessageContent{Blocks: blocks},
	})
	if err != nil {
		t.Fatalf("FromAnthropicMessage: %v", err)
	}
	return um
}

func requireParts(t *testing.T, um *vo.UnifiedMessage, want ...*vo.UnifiedContentPart) {
	t.Helper()
	if um.Content == nil {
		t.Fatal("content is nil")
	}
	if um.Content.Text != "" {
		t.Fatalf("expected parts form, got text %q", um.Content.Text)
	}
	if len(um.Content.Parts) != len(want) {
		t.Fatalf("parts len = %d, want %d: %+v", len(um.Content.Parts), len(want), um.Content.Parts)
	}
	for i, w := range want {
		got := um.Content.Parts[i]
		if got.Type != w.Type || got.Text != w.Text || got.ImageURL != w.ImageURL {
			t.Errorf("parts[%d] = %+v, want %+v", i, got, w)
		}
	}
}

func textPart(s string) *vo.UnifiedContentPart {
	return &vo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: s}
}

func imagePart(url string) *vo.UnifiedContentPart {
	return &vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: url}
}

func TestTopLevelBase64Image(t *testing.T) {
	t.Parallel()
	requireParts(t, convert(t, textBlock("看这张图"), base64Image()), textPart("看这张图"), imagePart(pngDataURI))
}

func TestTopLevelURLImage(t *testing.T) {
	t.Parallel()
	requireParts(t, convert(t, textBlock("看"), urlImage()), textPart("看"), imagePart(imageURL))
}

func TestImageOnlyMessage(t *testing.T) {
	t.Parallel()
	requireParts(t, convert(t, base64Image()), imagePart(pngDataURI))
}

func TestToolResultWithImage(t *testing.T) {
	t.Parallel()
	um := convert(t, toolResult(textBlock("截图如下"), base64Image()))
	if um.ToolCallID != "toolu_1" {
		t.Fatalf("ToolCallID = %q", um.ToolCallID)
	}
	requireParts(t, um, textPart("截图如下"), imagePart(pngDataURI))
}

func TestToolResultImageOnly(t *testing.T) {
	t.Parallel()
	requireParts(t, convert(t, toolResult(base64Image())), imagePart(pngDataURI))
}

func TestToolResultPlusTopLevelImage(t *testing.T) {
	t.Parallel()
	requireParts(t, convert(t, toolResult(textBlock("ok")), urlImage()), textPart("ok"), imagePart(imageURL))
}

func TestTextOnlyKeepsTextForm(t *testing.T) {
	t.Parallel()
	for name, um := range map[string]*vo.UnifiedMessage{
		"message":     convert(t, textBlock("a"), textBlock("b")),
		"tool result": convert(t, toolResult(textBlock("ok"))),
	} {
		if um.Content == nil || um.Content.Text == "" || len(um.Content.Parts) != 0 {
			t.Errorf("%s: expected text form, got %+v", name, um.Content)
		}
	}
}

func TestImageWithNilSourceSkipped(t *testing.T) {
	t.Parallel()
	um := convert(t, textBlock("hi"), &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeImage})
	if um.Content == nil || um.Content.Text != "hi" || len(um.Content.Parts) != 0 {
		t.Fatalf("expected text form without image, got %+v", um.Content)
	}
}

func TestResponseImageBlock(t *testing.T) {
	t.Parallel()
	um, err := dto.FromAnthropicResponse(&dto.AnthropicMessage{
		Role:    enum.RoleAssistant,
		Content: []*dto.AnthropicContentBlock{textBlock("done"), base64Image()},
	})
	if err != nil {
		t.Fatalf("FromAnthropicResponse: %v", err)
	}
	requireParts(t, um, textPart("done"), imagePart(pngDataURI))
}
