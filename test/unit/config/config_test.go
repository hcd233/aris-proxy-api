// Package config 验证 HTTP_CLIENT_TIMEOUT 环境变量覆盖与默认值。
package config

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/config"
)

// TestHTTPClientTimeoutEnv 验证 HTTP_CLIENT_TIMEOUT 环境变量可覆盖上游客户端超时。
// 由于 InitEnvironment 在包 init 时执行，通过子进程设置环境变量后重新初始化验证。
func TestHTTPClientTimeoutEnv(t *testing.T) {
	t.Parallel()
	if os.Getenv("_HTTP_CLIENT_TIMEOUT_CHECK") == "1" {
		config.InitEnvironment()
		if config.HTTPClientTimeout != 30*time.Minute {
			t.Fatalf("HTTPClientTimeout = %v, want 30m", config.HTTPClientTimeout)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestHTTPClientTimeoutEnv")
	cmd.Env = append(os.Environ(), "HTTP_CLIENT_TIMEOUT=30m", "_HTTP_CLIENT_TIMEOUT_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestHTTPClientTimeoutDefault 验证未配置时使用默认 5min。
func TestHTTPClientTimeoutDefault(t *testing.T) {
	t.Parallel()
	config.InitEnvironment()
	if config.HTTPClientTimeout != 5*time.Minute {
		t.Fatalf("HTTPClientTimeout = %v, want 5m", config.HTTPClientTimeout)
	}
}
