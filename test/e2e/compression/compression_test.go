package compression

import (
	"os"
	"testing"
)

// TestCompressionE2E 验证压缩后的请求能被上游正常接受并返回有效响应。
// 需要 BASE_URL 和 API_KEY 环境变量。
func TestCompressionE2E(t *testing.T) {
	t.Parallel()
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}

	// TODO: 构造含 tool output 的请求，发送到代理，验证响应正常
	// 具体实现取决于项目 E2E 测试框架
	t.Skip("E2E compression test requires running proxy with COMPRESSION_ENABLED=true")
}
