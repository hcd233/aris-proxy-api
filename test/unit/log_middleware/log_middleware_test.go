// Package log_middleware 日志中间件 body 记录策略回归测试。
//
// 背景：LogMiddleware 原先对每个非采样路径的响应体做完整 JSON 解析并再次编码进日志，
// 310KB 的 session/message/list 响应因此在请求线程上多付 8~20ms，是边缘大响应接口
// 的主要 CPU 成本之一（docs/loadtest/QPS-OPTIMIZATION.md §3.1）。本测试钉死新策略：
// 成功响应不记录 body，失败响应（错误或 4xx/5xx）保留排障现场。
//
// 中间件本身依赖包级 logger（项目 lint 禁止 zap.Logger 作为字段/参数），故这里只断言
// 策略函数；超长 body 的截断行为由 util.TruncateFieldValue 的用例覆盖（test/unit/truncate_field）。
package log_middleware

import (
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestShouldLogBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		status   int
		expected bool
	}{
		{name: "success 200", err: nil, status: fiber.StatusOK, expected: false},
		{name: "redirect 302", err: nil, status: fiber.StatusFound, expected: false},
		{name: "client error 400", err: nil, status: fiber.StatusBadRequest, expected: true},
		{name: "validation error 422", err: nil, status: fiber.StatusUnprocessableEntity, expected: true},
		{name: "server error 500", err: nil, status: fiber.StatusInternalServerError, expected: true},
		{name: "handler error with 200 status", err: ierr.New(ierr.ErrInternal, "boom"), status: fiber.StatusOK, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := middleware.ShouldLogBody(tt.err, tt.status)
			if got != tt.expected {
				t.Fatalf("ShouldLogBody(err=%v, status=%d) = %v, expected %v", tt.err, tt.status, got, tt.expected)
			}
		})
	}
}
