package middleware

import (
	"runtime/debug"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// RecoverMiddleware 恢复中间件
//
//	在所有请求入口捕获 panic，返回结构化 JSON 错误响应而非裸 500。
//
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-06-04 14:00:00
func RecoverMiddleware() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e any) {
			logger.WithFCtx(c).Error("[PanicRecovery] Recovered panic",
				zap.Any("error", e),
				zap.ByteString("stack", debug.Stack()))
		},
		PanicHandler: func(c fiber.Ctx, _ any) error {
			c.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
			c.Status(fiber.StatusOK).Send(
				lo.Must1(sonic.Marshal(&dto.CommonRsp{
					Error: ierr.ErrInternal.BizError(),
				})),
			)
			return nil
		},
	})
}
