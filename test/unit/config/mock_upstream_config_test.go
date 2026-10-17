package config

import (
	"os"
	"os/exec"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/config"
)

// TestMockUpstreamDisabledByDefault 验证 mock 开关默认关闭，
// 保证生产上不会误配 mock 模型后拿到假响应。
//
// 不调 InitEnvironment 重置：默认值在包 init 时已加载，直接读即可；在父进程
// 重置会写全局 config，与并行测试的读取构成 data race（-race 下必现）。
// 需要验证环境变量覆盖的用例统一走子进程模式（见下方 TestMockUpstreamEnvOverride）。
func TestMockUpstreamDisabledByDefault(t *testing.T) {
	t.Parallel()
	if config.MockUpstreamEnabled {
		t.Fatal("MockUpstreamEnabled must default to false")
	}
}

// TestMockUpstreamEnvOverride 验证 MOCK_UPSTREAM_ENABLED=true 时开关生效
// （子进程方式，与 TestHTTPBodyLimitEnv 同模式，避免污染并行测试）。
func TestMockUpstreamEnvOverride(t *testing.T) {
	t.Parallel()
	if os.Getenv("_MOCK_UPSTREAM_CHECK") == "1" {
		config.InitEnvironment()
		if !config.MockUpstreamEnabled {
			t.Fatal("MockUpstreamEnabled must be true when MOCK_UPSTREAM_ENABLED=true")
		}
		return
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestMockUpstreamEnvOverride$")
	cmd.Env = append(os.Environ(), "MOCK_UPSTREAM_ENABLED=true", "_MOCK_UPSTREAM_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}
