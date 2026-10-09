package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AffinityKey 组合亲和键：会话头（x-opencode-session / X-Session-Id）优先；
// 无会话头时回退 sha256(alias + NUL + 首条 user 文本) 指纹（同会话多轮稳定）；
// 两者皆空返回 ok=false（不参与亲和，纯 curl 单轮无粘滞等同现状）。
//
//	@param ctx context.Context 请求上下文（读取 CtxKeyPassthroughHeaders 会话头）
//	@param alias string 模型别名
//	@param firstUserText string 首条 user 消息文本（多模态取 text parts 拼接，无 text 传空）
//	@return string 亲和键
//	@return bool false=无亲和键
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func AffinityKey(ctx context.Context, alias, firstUserText string) (string, bool) {
	headers := util.GetPassthroughHeaders(ctx)
	for name, v := range headers {
		if (strings.EqualFold(name, constant.HTTPHeaderOpencodeSession) || strings.EqualFold(name, constant.HTTPHeaderSessionID)) && v != "" {
			return v, true
		}
	}
	if firstUserText == "" {
		return "", false
	}
	sum := sha256.Sum256([]byte(alias + "\x00" + firstUserText))
	return hex.EncodeToString(sum[:8]), true
}

// AffinityStore 端点亲和映射（Redis，TTL 见 constant.AffinityTTL；读写失败 fail-open 不阻塞转发）。
type AffinityStore struct {
	rdb *redis.Client
}

// NewAffinityStore 构造亲和存储
//
//	@param rdb *redis.Client
//	@return *AffinityStore
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func NewAffinityStore(rdb *redis.Client) *AffinityStore {
	return &AffinityStore{rdb: rdb}
}

// Get 读取亲和端点 ID；不存在或 Redis 失败返回 false（fail-open）。
//
//	@receiver s *AffinityStore
//	@param ctx context.Context
//	@param userID uint
//	@param alias string
//	@param key string
//	@return uint 端点 ID
//	@return bool false=未命中
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (s *AffinityStore) Get(ctx context.Context, userID uint, alias, key string) (uint, bool) {
	val, err := s.rdb.Get(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key)).Uint64()
	if err != nil {
		return 0, false
	}
	return uint(val), true
}

// Put 写入/刷新亲和映射（TTL 随写入刷新）；失败仅告警。
//
//	@receiver s *AffinityStore
//	@param ctx context.Context
//	@param userID uint
//	@param alias string
//	@param key string
//	@param endpointID uint
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (s *AffinityStore) Put(ctx context.Context, userID uint, alias, key string, endpointID uint) {
	if err := s.rdb.Set(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key), endpointID, constant.AffinityTTL).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[AffinityStore] Put affinity failed", zap.Error(err))
	}
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
