// Package log_middleware 日志中间件 body 记录策略回归测试。
//
// 背景：LogMiddleware 原先对每个非采样路径的请求体与响应体都做完整 JSON 解析并再次
// 编码进日志，310KB 的 session/message/list 响应因此在请求线程上多付 8~20ms，是边缘
// 大响应接口的主要 CPU 成本之一（docs/loadtest/QPS-OPTIMIZATION.md §3.1）。
//
// 现行策略是「请求体始终记、响应体只在失败现场记」：
//   - 请求体是 LLM 代理 prompt 的唯一落点（审计表不存内容），成功请求丢掉它等于
//     丢掉「这句话是谁问的」，所以不能一并按失败过滤；
//   - 响应体是日志体积与 CPU 的主要来源，只在错误或 4xx/5xx 时记录。
//
// 中间件本身依赖包级 logger（项目 lint 禁止 zap.Logger 作为字段/参数），故这里只断言
// 响应体策略函数；请求体「始终记录」由 middleware.LogMiddleware 的调用点保证（无分支），
// 超长 body 的截断由 test/unit/truncate_field 覆盖。
package log_middleware

import (
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestShouldLogResponseBody(t *testing.T) {
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

			got := middleware.ShouldLogResponseBody(tt.err, tt.status)
			if got != tt.expected {
				t.Fatalf("ShouldLogResponseBody(err=%v, status=%d) = %v, expected %v", tt.err, tt.status, got, tt.expected)
			}
		})
	}
}
