// Package e2eguard e2e 目标环境安全守卫。
//
// env-gated e2e 仅凭 BASE_URL 即对目标环境执行请求，其中 session_dedup 会发起
// 真实 LLM 调用（花钱），user_scope_config / trigger / apikeys 会真实增删配置。
// 本包提供共享的「拒绝指向生产」守卫：BASE_URL 命中生产域名且未设置
// E2E_ALLOW_LIVE=1 时让用例直接失败，防止误打生产。
//
// 选择 Fail 而非 Skip：BASE_URL 显式指向生产本身就是危险配置，t.Skip 会静默
// 掩盖误配置（本地看到 ok 实际什么都没验证，另一种假绿）；Fail 把危险目标变成
// 显式失败，确要对生产执行时用 E2E_ALLOW_LIVE=1 显式确认。
package e2eguard

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// AllowLiveEnv 强制放行生产目标的开关环境变量，值为 "1" 时跳过守卫
const AllowLiveEnv = "E2E_ALLOW_LIVE"

// prodHosts 生产域名标识（与 login-prod-server skill、install_origin 用例对齐）。
// 命中其一或其子域即判定为生产目标。
var prodHosts = []string{
	"api.lvlvko.top",
	"lvlvko.top",
}

// GuardLiveTarget 校验 baseURL 不指向生产环境，命中即 t.Fatal。
// 其它目标（本机、内网、预发）一律放行；每个 env-gated e2e 在读完 BASE_URL 后调用。
func GuardLiveTarget(t *testing.T, baseURL string) {
	t.Helper()
	if os.Getenv(AllowLiveEnv) == "1" {
		return
	}
	host := hostOf(baseURL)
	for _, prod := range prodHosts {
		if host == prod || strings.HasSuffix(host, "."+prod) {
			t.Fatalf("e2e refused: BASE_URL %q targets production host %q; "+
				"set %s=1 to run against production on purpose", baseURL, prod, AllowLiveEnv)
		}
	}
}

// hostOf 提取 URL 主机名（小写）；无法解析时按原文本清洗比较
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		return strings.ToLower(u.Hostname())
	}
	return strings.ToLower(strings.TrimSpace(raw))
}
