package config

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestHTTPClientTimeoutEnv 验证 HTTP_CLIENT_TIMEOUT 环境变量可覆盖上游客户端超时。
// 由于 initEnvironment 在包 init 时执行，通过子进程设置环境变量后重新初始化验证。
func TestHTTPClientTimeoutEnv(t *testing.T) {
	if os.Getenv("_HTTP_CLIENT_TIMEOUT_CHECK") == "1" {
		initEnvironment()
		if HTTPClientTimeout != 30*time.Minute {
			t.Fatalf("HTTPClientTimeout = %v, want 30m", HTTPClientTimeout)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestHTTPClientTimeoutEnv")
	cmd.Env = append(os.Environ(), "HTTP_CLIENT_TIMEOUT=30m", "_HTTP_CLIENT_TIMEOUT_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestHTTPClientTimeoutDefault 验证未配置时使用默认 5min。
func TestHTTPClientTimeoutDefault(t *testing.T) {
	initEnvironment()
	if HTTPClientTimeout != 5*time.Minute {
		t.Fatalf("HTTPClientTimeout = %v, want 5m", HTTPClientTimeout)
	}
}
