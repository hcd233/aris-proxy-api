// Package oauth_flow 验证订阅账号 provider 客户端的协议形态（httptest 模拟上游）。
package oauth_flow

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
)

// 全局 HTTP 客户端由应用启动时初始化；测试进程需显式初始化（config 包 init 已提供默认超时）。
func init() {
	httpclient.InitHTTPClient()
}

// TestAuthorizeURLContainsPKCEParams 授权链接必须携带 PKCE 与 state 参数。
func TestAuthorizeURLContainsPKCEParams(t *testing.T) {
	t.Parallel()

	for _, provider := range enum.OAuthProviders {
		client := oauthprovider.NewClient(provider)
		url := client.AuthorizeURL("challenge_x", "state_y")
		for _, want := range []string{"code_challenge=challenge_x", "state=state_y", "code_challenge_method=S256"} {
			if !strings.Contains(url, want) {
				t.Fatalf("%s authorize url missing %q: %s", provider, want, url)
			}
		}
	}
}

// TestClaudeExchangeAndRefresh Claude 换 token：JSON body 含 code_verifier；响应映射账号信息。
func TestClaudeExchangeAndRefresh(t *testing.T) {
	t.Parallel()

	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(raw) //nolint:errcheck // 测试辅助读取
		_ = sonic.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "at-1",
			"refresh_token": "rt-1",
			"expires_in": 3600,
			"organization": {"uuid": "org-1"},
			"account": {"email_address": "alice@example.com"}
		}`))
	}))
	defer server.Close()

	client := oauthprovider.NewClientWithURLs(enum.OAuthProviderAnthropic, server.URL+"/authorize", server.URL+"/token", server.URL+"/usage")
	set, err := client.ExchangeCode(t.Context(), "ac-1", "verifier-1", "state-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if gotBody["grant_type"] != "authorization_code" || gotBody["code_verifier"] != "verifier-1" || gotBody["code"] != "ac-1" {
		t.Fatalf("unexpected exchange body: %v", gotBody)
	}
	if set.AccessToken != "at-1" || set.RefreshToken != "rt-1" || set.AccountRef != "org-1" || set.AccountEmail != "alice@example.com" {
		t.Fatalf("unexpected token set: %+v", set)
	}
	if time.Until(set.ExpiresAt) < 50*time.Minute {
		t.Fatalf("expires_in mapping wrong: %v", set.ExpiresAt)
	}

	// 刷新：grant_type=refresh_token，refresh 空响应时保留原值由调用方处理
	gotBody = nil
	_, err = client.Refresh(t.Context(), "rt-1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if gotBody["grant_type"] != "refresh_token" || gotBody["refresh_token"] != "rt-1" {
		t.Fatalf("unexpected refresh body: %v", gotBody)
	}
}

// TestOpenAIExchangeExtractsAccountIDFromIDToken form body；id_token 解出 chatgpt_account_id/email/plan。
func TestOpenAIExchangeExtractsAccountIDFromIDToken(t *testing.T) {
	t.Parallel()

	claims := `{"https://api.openai.com/auth":{"chatgpt_account_id":"acc-1","chatgpt_plan_type":"plus"},"email":"bob@example.com"}`
	idToken := "h." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".s"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm() //nolint:errcheck // 测试辅助
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") != "verifier-2" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-2","refresh_token":"rt-2","expires_in":3600,"id_token":"` + idToken + `"}`))
	}))
	defer server.Close()

	client := oauthprovider.NewClientWithURLs(enum.OAuthProviderOpenAI, server.URL+"/authorize", server.URL+"/token", server.URL+"/usage")
	set, err := client.ExchangeCode(t.Context(), "ac-2", "verifier-2", "state-2")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if set.AccessToken != "at-2" || set.AccountRef != "acc-1" || set.AccountEmail != "bob@example.com" || set.PlanType != "plus" {
		t.Fatalf("unexpected token set: %+v", set)
	}
}

// TestProbeQuotaMapping 两 provider 的窗口映射到统一 QuotaSnapshot。
func TestProbeQuotaMapping(t *testing.T) {
	t.Parallel()

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(constant.HTTPHeaderAnthropicBeta) != constant.OAuthClaudeBetaHeader {
				t.Errorf("missing oauth beta header: %v", r.Header)
			}
			// 客户端身份头与 token 端点共用（M2：同类调用头必须一致）
			if got := r.Header.Get(constant.HTTPHeaderUserAgent); got != constant.OAuthClaudeUserAgent {
				t.Errorf("user-agent = %q, want %q", got, constant.OAuthClaudeUserAgent)
			}
			if got := r.Header.Get(constant.HTTPHeaderXApp); got != constant.OAuthClaudeXApp {
				t.Errorf("x-app = %q, want %q", got, constant.OAuthClaudeXApp)
			}
			w.Header().Set("Content-Type", "application/json")
			// 上游 utilization 已是 0~100 百分比（如 14.0 表示 14%），客户端必须原样透传
			_, _ = w.Write([]byte(`{"five_hour":{"utilization":12.5,"resets_at":"2099-01-01T00:00:00Z"},"seven_day":{"utilization":40.0,"resets_at":"2099-01-08T00:00:00Z"}}`))
		}))
		defer server.Close()

		client := oauthprovider.NewClientWithURLs(enum.OAuthProviderAnthropic, server.URL+"/a", server.URL+"/t", server.URL+"/usage")
		snapshot, err := client.ProbeQuota(t.Context(), "at", "org-1")
		if err != nil {
			t.Fatalf("ProbeQuota: %v", err)
		}
		if len(snapshot.Windows) != 2 || snapshot.Windows[0].Key != "five_hour" {
			t.Fatalf("unexpected windows: %+v", snapshot.Windows)
		}
		// 回归：utilization 是百分比，再乘 100 会让有用量的账号恒显示 100%（甚至 1250%）
		if snapshot.Windows[0].UsedPercent != 12.5 {
			t.Fatalf("used percent = %v, want 12.5 (percentage passed through as-is)", snapshot.Windows[0].UsedPercent)
		}
		if snapshot.Windows[1].UsedPercent != 40 {
			t.Fatalf("used percent = %v, want 40 (percentage passed through as-is)", snapshot.Windows[1].UsedPercent)
		}
	})

	t.Run("openai", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(constant.HTTPHeaderChatGPTAccountID) != "acc-1" {
				t.Errorf("missing account header: %v", r.Header)
			}
			// 客户端身份头与 token 端点共用（M2）
			if got := r.Header.Get(constant.HTTPHeaderUserAgent); got != constant.OAuthCodexUserAgent {
				t.Errorf("user-agent = %q, want %q", got, constant.OAuthCodexUserAgent)
			}
			if got := r.Header.Get(constant.HTTPHeaderOriginator); got != constant.OAuthCodexOriginator {
				t.Errorf("originator = %q, want %q", got, constant.OAuthCodexOriginator)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":25,"limit_window_seconds":18000,"reset_at":"2099-01-01T00:00:00Z"},"secondary_window":{"used_percent":60,"limit_window_seconds":604800,"reset_at":"2099-01-08T00:00:00Z"}}}`))
		}))
		defer server.Close()

		client := oauthprovider.NewClientWithURLs(enum.OAuthProviderOpenAI, server.URL+"/a", server.URL+"/t", server.URL+"/usage")
		snapshot, err := client.ProbeQuota(t.Context(), "at", "acc-1")
		if err != nil {
			t.Fatalf("ProbeQuota: %v", err)
		}
		if len(snapshot.Windows) != 2 || snapshot.Windows[0].Key != "primary" || snapshot.Windows[1].Key != "secondary" {
			t.Fatalf("unexpected windows: %+v", snapshot.Windows)
		}
		// Codex used_percent 已是百分比：不参与归一，原值透传
		if snapshot.Windows[0].UsedPercent != 25 || snapshot.Windows[1].UsedPercent != 60 {
			t.Fatalf("unexpected used percent: %+v", snapshot.Windows)
		}
		if snapshot.Windows[1].WindowMinutes != 10080 {
			t.Fatalf("window minutes mapping wrong: %+v", snapshot.Windows[1])
		}
	})
}
