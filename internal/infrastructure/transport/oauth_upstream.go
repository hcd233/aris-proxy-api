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
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
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

// mergeCommaHeader 合并逗号分隔头值（已包含目标值时保持原样）。
//
// 用 Values 聚合全部同名头行：客户端可能发多行 anthropic-beta，只用 Get 取首行
// 会在后续 Set 时丢掉其余行的值。
func mergeCommaHeader(header http.Header, name, value string) {
	parts := make([]string, 0, len(header.Values(name))+1)
	for _, line := range header.Values(name) {
		for part := range strings.SplitSeq(line, constant.CommaSeparator) {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" || slices.Contains(parts, trimmed) {
				continue
			}
			parts = append(parts, trimmed)
		}
	}
	if !slices.Contains(parts, value) {
		parts = append(parts, value)
	}
	header.Del(name)
	header.Set(name, strings.Join(parts, constant.CommaSpaceSeparator))
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

// ──────────────────────────── Codex（oauth_openai）────────────────────────────

// applyOpenAIOAuthUpstream 应用 Codex OAuth 端点的请求头特化。
//
// Authorization 覆盖为 Bearer <access token>；补 chatgpt-account-id（账号路由）、
// originator/UA（客户端身份协议事实）、accept: text/event-stream（Codex 仅支持流式）。
// Del x-api-key 与 Claude 分支同理：黑名单式透传会把 aris 侧密钥带给上游（安全项）。
func applyOpenAIOAuthUpstream(header http.Header, ep vo.UpstreamEndpoint) {
	header.Del(constant.HTTPHeaderAPIKey)
	header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)
	header.Set(constant.HTTPHeaderChatGPTAccountID, ep.OAuthAccountID)
	header.Set(constant.HTTPHeaderOriginator, constant.OAuthCodexOriginator)
	header.Set(constant.HTTPHeaderUserAgent, constant.OAuthCodexUserAgent)
	header.Set(constant.HTTPHeaderAccept, constant.HTTPContentTypeEventStream)
}

// transformCodexOAuthBody 应用 Codex OAuth 端点的 body 特化（最小必要集，规格 §4.3）：
//  1. 强制 store=false、stream=true（Codex 后端约束）；
//  2. 删除上游不接受的字段——在 DTO 上逐一置 nil（omitempty 后不再输出）。
//     规格 §4.3 删除清单：max_output_tokens / max_completion_tokens / temperature / top_p /
//     frequency_penalty / presence_penalty / user / metadata / prompt_cache_retention /
//     safety_identifier / stream_options / truncation / stop_sequences。
//     其中 max_completion_tokens / frequency_penalty / presence_penalty / stop_sequences
//     不在本 DTO 内，往返序列化时自然消失，所以下面只置 DTO 覆盖的 9 项
//     （评审发现：仅清 3 项会漏掉 metadata/truncation 等客户端可能携带的字段）；
//  3. reasoning 存在时补 include=["reasoning.encrypted_content"]。
//
// instructions 保持客户端原值（未带则不填）：不引入来源不明的默认文本，
// 若实测上游拒绝空 instructions，再从 Apache-2.0 许可的 openai/codex 取文案补充。
func transformCodexOAuthBody(body []byte) ([]byte, error) {
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(body, &req); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode codex body for oauth transform")
	}
	storeFalse := false
	streamTrue := true
	req.Store = &storeFalse
	req.Stream = &streamTrue

	// 删除清单中 DTO 存在的字段（其余如 frequency_penalty 不在 DTO，往返自然消失）
	req.MaxOutputTokens = nil
	req.Temperature = nil
	req.TopP = nil
	req.Metadata = nil
	req.User = nil
	req.PromptCacheRetention = nil
	req.SafetyIdentifier = nil
	req.StreamOptions = nil
	req.Truncation = nil

	if req.Reasoning != nil && !slices.Contains(req.Include, constant.OAuthCodexReasoningEncryptedInclude) {
		req.Include = append(req.Include, constant.OAuthCodexReasoningEncryptedInclude)
	}
	return sonic.Marshal(&req)
}

// upstreamCredLabel 上游凭据的日志标签（键名避开 Key/Token 等敏感词，值经 OAuth 占位符化）：OAuth 端点的 APIKey 字段承载 access token，
// 整体占位符化（MaskSecret 会暴露首尾各 4 字符，违背 token 不进日志的承诺）。
func upstreamCredLabel(ep vo.UpstreamEndpoint) string {
	if ep.AuthType.IsOAuth() {
		return constant.MaskSecretPlaceholder
	}
	return commonutil.MaskSecret(ep.APIKey)
}
