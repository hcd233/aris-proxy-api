package enum

import (
	"slices"
)

// OAuthProvider 订阅账号 OAuth 提供方
type OAuthProvider string

const (
	// OAuthProviderAnthropic Anthropic 订阅账号（Claude Code OAuth）
	OAuthProviderAnthropic OAuthProvider = "anthropic"

	// OAuthProviderOpenAI OpenAI 订阅账号（Codex OAuth）
	OAuthProviderOpenAI OAuthProvider = "openai"
)

// OAuthProviders 全部合法 provider。解析/白名单判断必须基于该集合，
// 新增 provider 时同步补充此处，避免多层口径不一致。
var OAuthProviders = []OAuthProvider{OAuthProviderAnthropic, OAuthProviderOpenAI}

// IsValid 是否为合法 provider（enum 包禁止依赖 ierr：由调用方据此构造 ErrValidation）
func (p OAuthProvider) IsValid() bool {
	return slices.Contains(OAuthProviders, p)
}
