// Package config 验证 HTTP_BODY_LIMIT 环境变量覆盖与默认值，以及 fiber BodyLimit 装配。
package config

import (
	"context"
	"os"
	"os/exec"
	"strconv"
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

// TestHTTPBodyLimitClamp 验证非法/过小配置：≤0 与非数字回落默认 16MB（而非 fiber 内建 4MB），
// 正数过小钳到 1MB 下限。
func TestHTTPBodyLimitClamp(t *testing.T) {
	t.Parallel()
	if want := os.Getenv("_HTTP_BODY_LIMIT_WANT"); want != "" {
		config.InitEnvironment()
		if got := strconv.Itoa(config.HTTPBodyLimit); got != want {
			t.Fatalf("HTTPBodyLimit = %s, want %s", got, want)
		}
		return
	}
	cases := []struct {
		name, env string
		want      int
	}{
		{"zero", "0", constant.MaxHTTPBodyBytes},
		{"negative", "-1", constant.MaxHTTPBodyBytes},
		{"non_numeric", "abc", constant.MaxHTTPBodyBytes},
		{"too_small", "1024", constant.MinHTTPBodyBytes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHTTPBodyLimitClamp$")
			cmd.Env = append(os.Environ(), "HTTP_BODY_LIMIT="+tc.env, "_HTTP_BODY_LIMIT_WANT="+strconv.Itoa(tc.want))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("subprocess failed: %v\n%s", err, out)
			}
		})
	}
}
