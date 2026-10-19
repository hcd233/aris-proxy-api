// Package oauth_flow 验证 OAuth 授权流的纯工具：PKCE 生成、state 生成与授权码解析。
package oauth_flow

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
)

// assertS256Challenge 独立复算 S256 challenge，防止实现里 challenge 与 verifier 不配对。
func assertS256Challenge(t *testing.T, verifier, challenge string) {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != want {
		t.Fatalf("challenge mismatch: got %q want %q", challenge, want)
	}
}

func isBase64URLNoPadding(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isUpper && !isLower && !isDigit && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func isLowerHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isHexLetter := r >= 'a' && r <= 'f'
		if !isDigit && !isHexLetter {
			return false
		}
	}
	return true
}

// TestGeneratePKCEForClaude Claude 侧 verifier 为 43 字符 base64url（32 随机字节）。
func TestGeneratePKCEForClaude(t *testing.T) {
	t.Parallel()

	verifier, challenge, err := commonutil.GeneratePKCE(enum.OAuthProviderAnthropic)
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if len(verifier) != 43 || !isBase64URLNoPadding(verifier) {
		t.Fatalf("claude verifier must be 43-char base64url, got %q (len=%d)", verifier, len(verifier))
	}
	assertS256Challenge(t, verifier, challenge)
}

// TestGeneratePKCEForOpenAI OpenAI 侧 verifier 为 128 字符 hex（64 随机字节）。
func TestGeneratePKCEForOpenAI(t *testing.T) {
	t.Parallel()

	verifier, challenge, err := commonutil.GeneratePKCE(enum.OAuthProviderOpenAI)
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if len(verifier) != 128 || !isLowerHex(verifier) {
		t.Fatalf("openai verifier must be 128-char hex, got %q (len=%d)", verifier, len(verifier))
	}
	assertS256Challenge(t, verifier, challenge)
}

// TestGeneratePKCEUnique 两次调用产物不同（随机源生效）。
func TestGeneratePKCEUnique(t *testing.T) {
	t.Parallel()

	v1, _, err := commonutil.GeneratePKCE(enum.OAuthProviderAnthropic)
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	v2, _, err := commonutil.GeneratePKCE(enum.OAuthProviderAnthropic)
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if v1 == v2 {
		t.Fatal("two PKCE verifiers must differ")
	}
}

// TestGenerateOAuthState state 为 43 字符 base64url 且两次不同。
func TestGenerateOAuthState(t *testing.T) {
	t.Parallel()

	s1, err := commonutil.GenerateOAuthState()
	if err != nil {
		t.Fatalf("GenerateOAuthState: %v", err)
	}
	if len(s1) != 43 || !isBase64URLNoPadding(s1) {
		t.Fatalf("state must be 43-char base64url, got %q", s1)
	}
	s2, err := commonutil.GenerateOAuthState()
	if err != nil {
		t.Fatalf("GenerateOAuthState: %v", err)
	}
	if s1 == s2 {
		t.Fatal("two states must differ")
	}
}

// TestParseOAuthCode 兼容三种回贴形态：完整回调 URL / code#state / 纯 code + fallback state。
func TestParseOAuthCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     string
		fallback  string
		wantCode  string
		wantState string
		wantErr   bool
	}{
		{
			name:      "full callback url",
			input:     "http://localhost:1455/auth/callback?code=ac_123&state=st_456",
			wantCode:  "ac_123",
			wantState: "st_456",
		},
		{
			name:      "callback url without state uses fallback",
			input:     "http://localhost:1455/auth/callback?code=ac_123",
			fallback:  "st_789",
			wantCode:  "ac_123",
			wantState: "st_789",
		},
		{
			name:      "claude code hash state",
			input:     "ac_123#st_456",
			wantCode:  "ac_123",
			wantState: "st_456",
		},
		{
			name:      "plain code with fallback state",
			input:     "ac_123",
			fallback:  "st_456",
			wantCode:  "ac_123",
			wantState: "st_456",
		},
		{
			name:    "plain code without any state",
			input:   "ac_123",
			wantErr: true,
		},
		{
			name:    "blank input",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "url without code",
			input:   "http://localhost:1455/auth/callback?state=st_456",
			wantErr: true,
		},
		{
			name:    "hash without code",
			input:   "#st_456",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, state, err := commonutil.ParseOAuthCode(tc.input, tc.fallback)
			if tc.wantErr {
				if !errors.Is(err, ierr.ErrValidation) {
					t.Fatalf("want ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOAuthCode: %v", err)
			}
			if code != tc.wantCode || state != tc.wantState {
				t.Fatalf("got (%q, %q), want (%q, %q)", code, state, tc.wantCode, tc.wantState)
			}
		})
	}
}
