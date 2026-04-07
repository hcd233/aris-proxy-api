package middleware

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// scannerGuardLua 路由扫描防护 Lua 脚本（原子操作）
//
// 当检测到一次路由未命中时调用：
//  1. 对 strike key 执行 INCR（违规计数 +1）
//  2. 若为首次记录，设置 strike key 的过期时间（观察窗口）
//  3. 若计数达到阈值，设置 ban key（封禁）并删除 strike key
//  4. 返回 [当前违规次数, 是否触发封禁(0/1)]
//
// KEYS[1]: strike key (scanner:strike:{ip})
// KEYS[2]: ban key    (scanner:ban:{ip})
// ARGV[1]: 封禁阈值
// ARGV[2]: 观察窗口 TTL（秒）
// ARGV[3]: 封禁时长 TTL（秒）
var scannerGuardLua = redis.NewScript(`
local strike_key = KEYS[1]
local ban_key = KEYS[2]
local threshold = tonumber(ARGV[1])
local window_ttl = tonumber(ARGV[2])
local ban_ttl = tonumber(ARGV[3])

local strikes = redis.call('INCR', strike_key)
if strikes == 1 then
    redis.call('EXPIRE', strike_key, window_ttl)
end

if strikes >= threshold then
    redis.call('SET', ban_key, '1', 'EX', ban_ttl)
    redis.call('DEL', strike_key)
    return {strikes, 1}
end

return {strikes, 0}
`)

// GuardConfig 路由扫描防护配置
//
//	@author centonhuang
//	@update 2026-04-07 10:00:00
type GuardConfig struct {
	StrikeThreshold int           // 在观察窗口内触发封禁的违规次数阈值
	StrikeWindow    time.Duration // 违规计数的观察窗口时长
	BanDuration     time.Duration // 触发封禁后的封禁时长
}

// isRouteNotFound 判断 Fiber 返回的错误是否为路由未匹配
func isRouteNotFound(err error) bool {
	var fiberErr *fiber.Error
	return errors.As(err, &fiberErr) && fiberErr.Code == fiber.StatusNotFound
}

// GuardMiddleware 路由扫描防护中间件
//
// 在 Fiber 层拦截路由扫描行为：
//   - 请求到达时，检查 IP 是否已被封禁（Redis GET），若封禁则直接返回 403
//   - 请求处理后，若 Fiber 返回路由未命中错误（Cannot GET/POST/...），
//     则通过 Lua 脚本原子地记录违规并在达到阈值时自动封禁
//
//	@param cfg GuardConfig
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-04-07 10:00:00
func GuardMiddleware(cfg GuardConfig) fiber.Handler {

	rdb := cache.GetRedisClient()
	thresholdStr := strconv.Itoa(cfg.StrikeThreshold)
	windowTTLStr := strconv.FormatInt(int64(cfg.StrikeWindow.Seconds()), 10)
	banTTLStr := strconv.FormatInt(int64(cfg.BanDuration.Seconds()), 10)

	return func(c *fiber.Ctx) error {
		ip := c.IP()
		banKey := fmt.Sprintf(constant.ScannerBanKeyTemplate, ip)
		ctx := c.Context()

		banned, err := rdb.Exists(ctx, banKey).Result()
		if err != nil {
			logger.WithFCtx(c).Warn("[GuardMiddleware] Failed to check ban status", zap.String("ip", ip), zap.Error(err))
		}
		if banned > 0 {
			return c.SendStatus(fiber.StatusForbidden)
		}

		nextErr := c.Next()

		if isRouteNotFound(nextErr) {
			strikeKey := fmt.Sprintf(constant.ScannerStrikeKeyTemplate,ip)

			result, luaErr := scannerGuardLua.Run(
				ctx, rdb,
				[]string{strikeKey, banKey},
				thresholdStr, windowTTLStr, banTTLStr,
			).Int64Slice()
			if luaErr != nil {
				logger.WithFCtx(c).Warn("[GuardMiddleware] Failed to execute strike script", zap.String("ip", ip), zap.Error(luaErr))
				return nextErr
			}

			strikes := result[0]
			wasBanned := result[1] == 1
			if wasBanned {
				logger.WithFCtx(c).Warn("[GuardMiddleware] IP banned due to route scanning",
					zap.String("ip", ip),
					zap.Int64("strikes", strikes),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()))
			}
		}

		return nextErr
	}
}
