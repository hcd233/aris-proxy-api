package lintstatic_test

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/tool/lintstatic"
)

// TestRun_MissingGolangciLintFails 回归（2026-09-09 CR）：golangci-lint 缺失时
// Run 必须返回错误而非静默跳过。移除独立 go vet / staticcheck 进程后，静默跳过
// 会让本地 lint 完全失去 govet/staticcheck 覆盖，直到 CI 才暴露。
//
//nolint:paralleltest // t.Setenv 与 t.Parallel 互斥
func TestRun_MissingGolangciLintFails(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("GOBIN", "")

	result := lintstatic.Run([]string{"./..."})
	if result.Err == nil {
		t.Fatalf("Run() Err = nil, want error when golangci-lint is missing; output=%q", result.Output)
	}
	if !strings.Contains(result.Output, "golangci-lint not found") {
		t.Fatalf("Output = %q, want golangci-lint missing hint", result.Output)
	}
}
