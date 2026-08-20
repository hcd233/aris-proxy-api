// Package config 验证熔断/bulkhead 配置默认值与环境变量覆盖。
package config

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/config"
)

// TestUpstreamCircuitEnv 验证环境变量可覆盖熔断/bulkhead 配置（子进程方式，与 HTTPClientTimeout 测试同模式）。
func TestUpstreamCircuitEnv(t *testing.T) {
	t.Parallel()
	if os.Getenv("_UPSTREAM_CIRCUIT_CHECK") == "1" {
		config.InitEnvironment()
		if config.UpstreamCircuitWindow != 10*time.Second {
			t.Fatalf("UpstreamCircuitWindow = %v, want 10s", config.UpstreamCircuitWindow)
		}
		if config.UpstreamCircuitMinRequests != 5 {
			t.Fatalf("UpstreamCircuitMinRequests = %d, want 5", config.UpstreamCircuitMinRequests)
		}
		if config.UpstreamBulkheadMaxConcurrent != 16 {
			t.Fatalf("UpstreamBulkheadMaxConcurrent = %d, want 16", config.UpstreamBulkheadMaxConcurrent)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestUpstreamCircuitEnv")
	cmd.Env = append(os.Environ(),
		"UPSTREAM_CIRCUIT_WINDOW=10s",
		"UPSTREAM_CIRCUIT_MIN_REQUESTS=5",
		"UPSTREAM_BULKHEAD_MAX_CONCURRENT=16",
		"_UPSTREAM_CIRCUIT_CHECK=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestUpstreamCircuitDefaults 验证未配置时使用 spec 默认值。
func TestUpstreamCircuitDefaults(t *testing.T) {
	t.Parallel()
	config.InitEnvironment()
	if !config.UpstreamCircuitEnabled {
		t.Fatal("UpstreamCircuitEnabled = false, want true")
	}
	if config.UpstreamCircuitWindow != 60*time.Second {
		t.Fatalf("UpstreamCircuitWindow = %v, want 60s", config.UpstreamCircuitWindow)
	}
	if config.UpstreamCircuitMinRequests != 10 {
		t.Fatalf("UpstreamCircuitMinRequests = %d, want 10", config.UpstreamCircuitMinRequests)
	}
	if config.UpstreamCircuitErrorThreshold != 0.5 {
		t.Fatalf("UpstreamCircuitErrorThreshold = %v, want 0.5", config.UpstreamCircuitErrorThreshold)
	}
	if config.UpstreamCircuitOpenTimeout != 30*time.Second {
		t.Fatalf("UpstreamCircuitOpenTimeout = %v, want 30s", config.UpstreamCircuitOpenTimeout)
	}
	if config.UpstreamCircuitHalfOpenMaxRequests != 1 {
		t.Fatalf("UpstreamCircuitHalfOpenMaxRequests = %d, want 1", config.UpstreamCircuitHalfOpenMaxRequests)
	}
	if !config.UpstreamBulkheadEnabled {
		t.Fatal("UpstreamBulkheadEnabled = false, want true")
	}
	if config.UpstreamBulkheadMaxConcurrent != 32 {
		t.Fatalf("UpstreamBulkheadMaxConcurrent = %d, want 32", config.UpstreamBulkheadMaxConcurrent)
	}
	if config.UpstreamBulkheadAcquireTimeout != time.Second {
		t.Fatalf("UpstreamBulkheadAcquireTimeout = %v, want 1s", config.UpstreamBulkheadAcquireTimeout)
	}
}
