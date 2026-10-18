package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// 本文件承载订阅账号（OAuth）端点的上游请求特化：headers 补丁与 body 最小改写。
//
// 设计原则（见设计文档 §4.3）：全部字面量取 constant/oauth.go；只做「最小必要集」，
// 被上游拒绝时按差异清单增量补——特化逻辑集中本文件，便于实测迭代。

// ──────────────────────────── Claude（oauth_anthropic）────────────────────────────

// applyAnthropicOAuthUpstream 应用 Claude OAuth 端点的请求头特化。
//
// 必须在 applyPassthroughRequestHeaders 之后调用（本函数负责覆盖/删除透传值）：
//  1. Del x-api-key——中间件是黑名单式透传，客户端带来的 x-api-key（aris 侧密钥）
//     必须在转发前删除，否则会把 aris 的 key 泄漏给上游（安全项）；
//  2. Authorization 覆盖为 Bearer <access token>；
//  3. anthropic-beta 合并（客户端透传值 ∪ oauth 必需值）——覆盖会丢掉客户端的
//     1M context 等 beta；
//  4. UA 覆盖为 Claude CLI 形态、x-app: cli（协议事实）。
func applyAnthropicOAuthUpstream(header http.Header, ep vo.UpstreamEndpoint) {
	header.Del(constant.HTTPHeaderAPIKey)
	header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)
	mergeCommaHeader(header, constant.HTTPHeaderAnthropicBeta, constant.OAuthClaudeBetaHeader)
	header.Set(constant.HTTPHeaderUserAgent, constant.OAuthClaudeUserAgent)
	header.Set(constant.HTTPHeaderXApp, constant.OAuthClaudeXApp)
}

// mergeCommaHeader 合并逗号分隔头值（已包含目标值时保持原样）
func mergeCommaHeader(header http.Header, name, value string) {
	existing := header.Get(name)
	if existing == "" {
		header.Set(name, value)
		return
	}
	parts := strings.Split(existing, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if slices.Contains(parts, value) {
		return
	}
	header.Set(name, existing+", "+value)
}

// transformAnthropicOAuthBody 应用 Claude OAuth 端点的 body 特化（最小必要集）：
//  1. system 前置 Claude Code 身份块（string/数组/缺省三态归一为数组）；
//  2. metadata.user_id 补稳定派生值（账号 + 首条 user 文本，同会话多轮稳定）。
//
// body 由 aris 从 dto 序列化而来，故用 dto 反序列化操作再重序列化（字段覆盖度即 DTO 覆盖度）。
func transformAnthropicOAuthBody(body []byte, ep vo.UpstreamEndpoint) ([]byte, error) {
	var req dto.AnthropicCreateMessageReq
	if err := sonic.Unmarshal(body, &req); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode anthropic body for oauth transform")
	}
	req.System = prependClaudeIdentityBlock(req.System)
	if req.Metadata == nil {
		req.Metadata = &dto.AnthropicMetadata{}
	}
	userID := buildClaudeMetadataUserID(ep.OAuthAccountID, firstUserText(req.Messages))
	req.Metadata.UserID = &userID
	return sonic.Marshal(&req)
}

// prependClaudeIdentityBlock 在 system 首位插入 Claude Code 身份块。
//
// 上游按 system 首块判定调用端身份（协议事实）；原 system 内容不作迁移，仅前置。
func prependClaudeIdentityBlock(content *dto.AnthropicMessageContent) *dto.AnthropicMessageContent {
	identityText := constant.OAuthClaudeSystemBlock
	identity := &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: &identityText}
	if content == nil {
		return &dto.AnthropicMessageContent{Blocks: []*dto.AnthropicContentBlock{identity}}
	}
	blocks := make([]*dto.AnthropicContentBlock, 0, len(content.Blocks)+2)
	blocks = append(blocks, identity)
	if len(content.Blocks) > 0 {
		blocks = append(blocks, content.Blocks...)
	} else if content.Text != "" {
		originalText := content.Text
		blocks = append(blocks, &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: &originalText})
	}
	return &dto.AnthropicMessageContent{Blocks: blocks}
}

// firstUserText 提取首条 user 消息的文本（string 或首个 text 块）；缺失返回空串。
func firstUserText(messages []*dto.AnthropicMessageParam) string {
	for _, message := range messages {
		if message == nil || message.Content == nil || message.Role != enum.RoleUser {
			continue
		}
		if message.Content.Text != "" {
			return message.Content.Text
		}
		for _, block := range message.Content.Blocks {
			if block != nil && block.Text != nil && *block.Text != "" {
				return *block.Text
			}
		}
	}
	return ""
}

// claudeMetadataUserIDToken Claude metadata.user_id 载荷（协议事实：上游据此做账号/会话归因）
type claudeMetadataUserIDToken struct {
	DeviceID    string `json:"device_id"`
	AccountUUID string `json:"account_uuid"`
	SessionID   string `json:"session_id"`
}

// buildClaudeMetadataUserID 生成稳定派生的 user_id JSON 串（同输入恒同输出，序列化失败返回空串不阻断请求）。
func buildClaudeMetadataUserID(accountRef, firstUser string) string {
	payload, err := sonic.Marshal(claudeMetadataUserIDToken{
		DeviceID:    stableHexDigest(accountRef),
		AccountUUID: accountRef,
		SessionID:   stableHexDigest(accountRef + firstUser),
	})
	if err != nil {
		return ""
	}
	return string(payload)
}

// stableHexDigest 生成稳定的 16 位十六进制摘要（不涉密：仅作上游会话归因的派生标识）
func stableHexDigest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:8])
}
