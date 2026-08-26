// Package config 验证熔断/bulkhead 配置默认值与环境变量覆盖。
package config

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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

// TestUpstreamCircuitWindowClamp 验证窗口小于桶数秒数时被 clamp 到下界（防 record 桶索引除零 panic）。
func TestUpstreamCircuitWindowClamp(t *testing.T) {
	t.Parallel()
	if os.Getenv("_UPSTREAM_CIRCUIT_CHECK") == "1" {
		config.InitEnvironment()
		if config.UpstreamCircuitWindow != constant.ResilienceMinWindow {
			t.Fatalf("UpstreamCircuitWindow = %v, want clamped to %v", config.UpstreamCircuitWindow, constant.ResilienceMinWindow)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestUpstreamCircuitWindowClamp")
	cmd.Env = append(os.Environ(),
		"UPSTREAM_CIRCUIT_WINDOW=1s",
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

// TestUpstreamResilienceClamps 验证极端配置被钳制到合法区间：
// 极端值会让组件行为静默退化（min_requests=0 单次失败即熔断、threshold≤0 阈值
// 失效、halfopen=0 行为退化、max_concurrent=0 服务自锁），不得原样透传。
func TestUpstreamResilienceClamps(t *testing.T) {
	t.Parallel()
	if os.Getenv("_UPSTREAM_RESILIENCE_CLAMP_CHECK") == "1" {
		config.InitEnvironment()
		if config.UpstreamCircuitMinRequests != constant.ResilienceMinMinRequests {
			t.Fatalf("UpstreamCircuitMinRequests = %d, want %d (clamped)", config.UpstreamCircuitMinRequests, constant.ResilienceMinMinRequests)
		}
		if config.UpstreamCircuitErrorThreshold != constant.ResilienceMinErrorThreshold {
			t.Fatalf("UpstreamCircuitErrorThreshold = %v, want %v (clamped)", config.UpstreamCircuitErrorThreshold, constant.ResilienceMinErrorThreshold)
		}
		if config.UpstreamCircuitOpenTimeout != constant.ResilienceMinOpenTimeout {
			t.Fatalf("UpstreamCircuitOpenTimeout = %v, want %v (clamped)", config.UpstreamCircuitOpenTimeout, constant.ResilienceMinOpenTimeout)
		}
		if config.UpstreamCircuitHalfOpenMaxRequests != constant.ResilienceMinHalfOpenMaxRequests {
			t.Fatalf("UpstreamCircuitHalfOpenMaxRequests = %d, want %d (clamped)", config.UpstreamCircuitHalfOpenMaxRequests, constant.ResilienceMinHalfOpenMaxRequests)
		}
		if config.UpstreamBulkheadMaxConcurrent != constant.ResilienceMinMaxConcurrent {
			t.Fatalf("UpstreamBulkheadMaxConcurrent = %d, want %d (clamped)", config.UpstreamBulkheadMaxConcurrent, constant.ResilienceMinMaxConcurrent)
		}
		if config.UpstreamBulkheadAcquireTimeout != constant.ResilienceMinAcquireTimeout {
			t.Fatalf("UpstreamBulkheadAcquireTimeout = %v, want %v (clamped)", config.UpstreamBulkheadAcquireTimeout, constant.ResilienceMinAcquireTimeout)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestUpstreamResilienceClamps")
	cmd.Env = append(os.Environ(),
		"UPSTREAM_CIRCUIT_MIN_REQUESTS=0",
		"UPSTREAM_CIRCUIT_ERROR_THRESHOLD=-1",
		"UPSTREAM_CIRCUIT_OPEN_TIMEOUT=1ms",
		"UPSTREAM_CIRCUIT_HALFOPEN_MAX_REQUESTS=-2",
		"UPSTREAM_BULKHEAD_MAX_CONCURRENT=0",
		"UPSTREAM_BULKHEAD_ACQUIRE_TIMEOUT=1ns",
		"_UPSTREAM_RESILIENCE_CLAMP_CHECK=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestUpstreamResilienceClampsUpperBound 验证越上界的错误率阈值被钳回 1。
func TestUpstreamResilienceClampsUpperBound(t *testing.T) {
	t.Parallel()
	if os.Getenv("_UPSTREAM_RESILIENCE_CLAMP_HI_CHECK") == "1" {
		config.InitEnvironment()
		if config.UpstreamCircuitErrorThreshold != constant.ResilienceMaxErrorThreshold {
			t.Fatalf("UpstreamCircuitErrorThreshold = %v, want %v (clamped to max)", config.UpstreamCircuitErrorThreshold, constant.ResilienceMaxErrorThreshold)
		}
		return
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestUpstreamResilienceClampsUpperBound")
	cmd.Env = append(os.Environ(),
		"UPSTREAM_CIRCUIT_ERROR_THRESHOLD=3.5",
		"_UPSTREAM_RESILIENCE_CLAMP_HI_CHECK=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}
