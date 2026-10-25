package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
)

// CompressMiddleware 压缩中间件
//
// 用 LevelBestSpeed 而非默认档：默认档是 gzip 6 / brotli 4，在大 JSON 响应
// （session/message/list 310KB）上压缩率只比最快档好
// 10~15%，CPU 却要 2~3 倍。本服务瓶颈是 CPU 不是带宽，详见
// docs/loadtest/QPS-OPTIMIZATION.md §3.4。
//
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func CompressMiddleware() fiber.Handler {
	return compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	})
}
