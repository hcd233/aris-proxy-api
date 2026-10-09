package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AffinityKey 组合亲和键：会话头（x-opencode-session / X-Session-Id）优先；
// 无会话头时回退 alias + NUL + 首条 user 文本的指纹（同会话多轮稳定）；
// 两者皆空返回 ok=false（不参与亲和，纯 curl 单轮无粘滞等同现状）。
// 会话头同样取摘要：其长度与内容由客户端控制，不能直接拼进 Redis key。
//
//	@param ctx context.Context 请求上下文（读取 CtxKeyPassthroughHeaders 会话头）
//	@param alias string 模型别名
//	@param firstUserText string 首条 user 消息文本（多模态取 text parts 拼接，无 text 传空）
//	@return string 亲和键（16 位十六进制摘要）
//	@return bool false=无亲和键
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func AffinityKey(ctx context.Context, alias, firstUserText string) (string, bool) {
	for name, v := range util.GetPassthroughHeaders(ctx) {
		if (strings.EqualFold(name, constant.HTTPHeaderOpencodeSession) || strings.EqualFold(name, constant.HTTPHeaderSessionID)) && v != "" {
			return affinityDigest(v), true
		}
	}
	if firstUserText == "" {
		return "", false
	}
	return affinityDigest(alias + "\x00" + firstUserText), true
}

func affinityDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// rememberAffinity 转发成功后把亲和键映射到实际成功的端点（fallback 换端点后即改写为新端点）。
func rememberAffinity(ctx context.Context, affinity service.EndpointAffinity, userID uint, alias, key string, endpointID uint) {
	if key == "" || affinity == nil {
		return
	}
	affinity.Put(ctx, userID, alias, key, endpointID)
}

// firstUserTextOpenAIChat 提取 chat 请求首条 user 消息文本（多模态取 text parts 拼接，无 text 空串）。
func firstUserTextOpenAIChat(msgs []*dto.OpenAIChatCompletionMessageParam) string {
	for _, m := range msgs {
		if m == nil || m.Role != enum.RoleUser || m.Content == nil {
			continue
		}
		if m.Content.Text != "" {
			return m.Content.Text
		}
		var b strings.Builder
		for _, p := range m.Content.Parts {
			if p != nil && p.Type == enum.ContentPartTypeText && p.Text != nil {
				b.WriteString(*p.Text)
			}
		}
		return b.String()
	}
	return ""
}

// firstUserTextAnthropic 提取 messages 请求首条 user 消息文本（多模态取 text blocks 拼接）。
func firstUserTextAnthropic(msgs []*dto.AnthropicMessageParam) string {
	for _, m := range msgs {
		if m == nil || m.Role != enum.RoleUser || m.Content == nil {
			continue
		}
		if m.Content.Text != "" {
			return m.Content.Text
		}
		var b strings.Builder
		for _, blk := range m.Content.Blocks {
			if blk != nil && blk.Type == enum.AnthropicContentBlockTypeText && blk.Text != nil {
				b.WriteString(*blk.Text)
			}
		}
		return b.String()
	}
	return ""
}

// firstUserTextResponse 提取 response 请求首条 user 输入文本：
// input 为纯字符串时即 user 输入；为 item 数组时取首条 role=user 的浅层 text。
func firstUserTextResponse(input *dto.ResponseInput) string {
	if input == nil {
		return ""
	}
	if input.Text != "" {
		return input.Text
	}
	for _, item := range input.Items {
		if item == nil || item.Role == nil || *item.Role != enum.RoleUser || item.Content == nil {
			continue
		}
		if item.Content.Text != "" {
			return item.Content.Text
		}
		var b strings.Builder
		for _, p := range item.Content.Parts {
			if p != nil && p.Text != nil {
				b.WriteString(*p.Text)
			}
		}
		return b.String()
	}
	return ""
}
