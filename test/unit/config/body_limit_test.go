// Package config 验证 HTTP_BODY_LIMIT 环境变量覆盖与默认值，以及 fiber BodyLimit 装配。
package config

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/api"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

// TestHTTPBodyLimitEnv 验证 HTTP_BODY_LIMIT 环境变量可覆盖 fiber BodyLimit
// （子进程方式，与 HTTPClientTimeout 测试同模式）。
func TestHTTPBodyLimitEnv(t *testing.T) {
	t.Parallel()
	if os.Getenv("_HTTP_BODY_LIMIT_CHECK") == "1" {
		config.InitEnvironment()
		want := 48 * 1024 * 1024
		if config.HTTPBodyLimit != want {
			t.Fatalf("HTTPBodyLimit = %d, want %d", config.HTTPBodyLimit, want)
		}
		if got := api.NewFiberApp().Config().BodyLimit; got != want {
			t.Fatalf("fiber BodyLimit = %d, want %d", got, want)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestHTTPBodyLimitEnv")
	cmd.Env = append(os.Environ(), "HTTP_BODY_LIMIT=50331648", "_HTTP_BODY_LIMIT_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestHTTPBodyLimitDefault 验证未配置时使用默认 16MB。
func TestHTTPBodyLimitDefault(t *testing.T) {
	t.Parallel()
	config.InitEnvironment()
	if config.HTTPBodyLimit != constant.MaxHTTPBodyBytes {
		t.Fatalf("HTTPBodyLimit = %d, want %d", config.HTTPBodyLimit, constant.MaxHTTPBodyBytes)
	}
}
