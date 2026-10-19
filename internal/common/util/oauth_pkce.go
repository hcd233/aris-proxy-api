package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// GeneratePKCE 生成 PKCE verifier 与 S256 challenge。
//
// verifier 形态按 provider 区分（协议事实，见 constant/oauth.go）：
//   - Claude（anthropic）：32 随机字节 base64url 无 padding（43 字符）
//   - Codex（openai）：64 随机字节小写 hex（128 字符）
//
// challenge 统一 = base64url 无 padding(SHA256(verifier))。
func GeneratePKCE(provider enum.OAuthProvider) (verifier, challenge string, err error) {
	var size int
	switch provider {
	case enum.OAuthProviderAnthropic:
		size = constant.OAuthClaudeVerifierBytes
	case enum.OAuthProviderOpenAI:
		size = constant.OAuthOpenAIVerifierBytes
	default:
		return "", "", ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", string(provider))
	}

	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", "", ierr.Wrap(ierr.ErrInternal, err, "read pkce entropy")
	}
	switch provider {
	case enum.OAuthProviderAnthropic:
		verifier = base64.RawURLEncoding.EncodeToString(raw)
	case enum.OAuthProviderOpenAI:
		verifier = hex.EncodeToString(raw)
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// GenerateOAuthState 生成 32 随机字节的 base64url state（服务端保存、回调时校验比对，防重放）。
func GenerateOAuthState() (string, error) {
	raw := make([]byte, constant.OAuthStateBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "read state entropy")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ParseOAuthCode 解析用户回贴的授权码输入，兼容三种形态：
//
//  1. 完整回调 URL（Codex：浏览器跳 localhost 打不开，用户从地址栏复制整段）→ 取 query 的 code/state
//  2. "code#state"（Claude 授权页展示格式）→ 按首个 # 切分
//  3. 纯 code → state 取自调用方提供的 fallback（请求体 state 字段）
//
// code 与 state 均必填；任一缺失返回 ErrValidation。
func ParseOAuthCode(input, fallbackState string) (code, state string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", ierr.New(ierr.ErrValidation, "oauth authorization code is required")
	}

	state = strings.TrimSpace(fallbackState)
	switch {
	case strings.HasPrefix(trimmed, constant.OAuthURLSchemeHTTPPrefix) || strings.HasPrefix(trimmed, constant.OAuthURLSchemeHTTPSPrefix):
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", ierr.Wrap(ierr.ErrValidation, parseErr, "parse oauth callback url")
		}
		query := parsed.Query()
		code = strings.TrimSpace(query.Get(constant.OAuthCallbackParamCode))
		if queryState := strings.TrimSpace(query.Get(constant.OAuthCallbackParamState)); queryState != "" {
			state = queryState
		}
	default:
		if before, after, found := strings.Cut(trimmed, "#"); found {
			code = strings.TrimSpace(before)
			if hashState := strings.TrimSpace(after); hashState != "" {
				state = hashState
			}
		} else {
			code = trimmed
		}
	}

	if code == "" {
		return "", "", ierr.New(ierr.ErrValidation, "oauth authorization code is required")
	}
	if state == "" {
		return "", "", ierr.New(ierr.ErrValidation, "oauth state is required")
	}
	return code, state, nil
}
