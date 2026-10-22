package middleware

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
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
//	@update 2026-04-10 10:00:00
type GuardConfig struct {
	StrikeThreshold int           // 在观察窗口内触发封禁的违规次数阈值
	StrikeWindow    time.Duration // 违规计数的观察窗口时长
	BanDuration     time.Duration // 触发封禁后的封禁时长
	// IgnoredPaths lists paths where 404 must not increment route-scan strikes.
	// If nil, DefaultGuardIgnored404Paths is used.
	IgnoredPaths []string
	// AllowIPs lists IPs that are exempted from route-scan banning (whitelist).
	AllowIPs []string
}

// isRouteNotFound 判断 Fiber 返回的错误是否为路由未匹配
func isRouteNotFound(err error) bool {
	var fiberErr *fiber.Error
	return errors.As(err, &fiberErr) && fiberErr.Code == fiber.StatusNotFound
}

// BanCleanCache 进程内「该 IP 未被封禁」负缓存。
//
// 每请求一次的 Redis EXISTS 是固定成本，而封禁是低频事件：缓存 1s 内的干净结论，
// 把 Redis 调用摊薄到每 IP 每秒 1 次。封禁最迟 TTL 后生效；本进程自己触发封禁时
// 调 Invalidate 立即失效。仅在单个 pod 内生效，跨 pod 结论各自独立（Redis ban key 是权威源）。
//
// 命中路径在每请求热路径上，只读，因此用 RWMutex：互斥锁会把同 pod 的并发请求串行化。
type BanCleanCache struct {
	mu      sync.RWMutex
	entries map[string]time.Time
	ttl     time.Duration
	limit   int
}

// NewBanCleanCache 创建负缓存
//
//	@param ttl 结论有效期
//	@param limit 条目上限，超限淘汰最早过期的条目
//	@return *BanCleanCache
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func NewBanCleanCache(ttl time.Duration, limit int) *BanCleanCache {
	return &BanCleanCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		limit:   limit,
	}
}

// IsClean 判断该 IP 是否仍在「未封禁」结论有效期内
//
// 过期条目不在这里删除：读路径持读锁，删 map 需要写锁。过期条目由 MarkClean 的
// 淘汰逻辑回收，内存上限由 limit 保证。
//
//	@receiver c *BanCleanCache
//	@param ip string 客户端 IP
//	@return bool 是否处于「未封禁」结论有效期内
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func (c *BanCleanCache) IsClean(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expiry, ok := c.entries[ip]
	return ok && time.Now().Before(expiry)
}

// MarkClean 记录该 IP 未被封禁
//
//	@receiver c *BanCleanCache
//	@param ip string 客户端 IP
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func (c *BanCleanCache) MarkClean(ip string) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.limit {
		c.evictLocked(now)
	}
	c.entries[ip] = now.Add(c.ttl)
}

// evictLocked 在超限时回收空间，调用方必须持写锁
//
// 先清掉已过期条目，仍超限则淘汰最早过期的一条。刻意不用 clear() 整体清空：
// 源 IP 被伪造打满时整体清空会让所有结论周期性失效，把优化变成 Redis 尖峰。
//
// ponytail: 仅在超限时 O(n) 扫一遍, 上限若需上万条再换 LRU。
//
//	@receiver c *BanCleanCache
//	@param now time.Time 当前时间
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func (c *BanCleanCache) evictLocked(now time.Time) {
	for ip, expiry := range c.entries {
		if now.After(expiry) {
			delete(c.entries, ip)
		}
	}
	if len(c.entries) < c.limit {
		return
	}

	oldestIP := ""
	var oldestExpiry time.Time
	for ip, expiry := range c.entries {
		if oldestIP == "" || expiry.Before(oldestExpiry) {
			oldestIP = ip
			oldestExpiry = expiry
		}
	}
	if oldestIP != "" {
		delete(c.entries, oldestIP)
	}
}

// Invalidate 丢弃该 IP 的缓存结论（本进程触发封禁时调用）
//
//	@receiver c *BanCleanCache
//	@param ip string 客户端 IP
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func (c *BanCleanCache) Invalidate(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, ip)
}

// GuardMiddleware 路由扫描防护中间件
//
// 在 Fiber 层拦截路由扫描行为：
//
//   - 请求到达时，检查 IP 是否已被封禁（Redis GET），若封禁则直接返回 403
//
//   - 请求处理后，若 Fiber 返回路由未命中错误（Cannot GET/POST/...），
//     则通过 Lua 脚本原子地记录违规并在达到阈值时自动封禁
//
//     @param cfg GuardConfig
//     @return fiber.Handler
//     @author centonhuang
//     @update 2026-04-10 10:00:00
func GuardMiddleware(cache *redis.Client, cfg GuardConfig) fiber.Handler {
	thresholdStr := strconv.Itoa(cfg.StrikeThreshold)
	windowTTLStr := strconv.FormatInt(int64(cfg.StrikeWindow.Seconds()), 10)
	banTTLStr := strconv.FormatInt(int64(cfg.BanDuration.Seconds()), 10)
	ignoredPaths := lo.SliceToMap(cfg.IgnoredPaths, func(p string) (string, struct{}) {
		return p, struct{}{}
	})
	allowIPs := lo.SliceToMap(cfg.AllowIPs, func(p string) (string, struct{}) {
		return p, struct{}{}
	})
	cleanCache := NewBanCleanCache(constant.GuardCleanCacheTTL, constant.GuardCleanCacheLimit)
	return func(c fiber.Ctx) error {
		if cache == nil {
			logger.WithFCtx(c).Warn("[GuardMiddleware] Redis dependency is nil")
			return c.Next()
		}
		ip := c.IP()
		if _, allowed := allowIPs[ip]; allowed {
			return c.Next()
		}
		banKey := fmt.Sprintf(constant.ScannerBanKeyTemplate, ip)
		ctx := c.RequestCtx()

		// 负缓存命中则跳过 ban 查询；下方的 404 扫描计数逻辑对每个请求照常执行
		if !cleanCache.IsClean(ip) {
			banned, err := cache.Exists(ctx, banKey).Result()
			switch {
			case err != nil:
				// fail-open，但不把「检查失败」固化成「干净」结论：下次请求仍会重新问 Redis
				logger.WithFCtx(c).Warn("[GuardMiddleware] Failed to check ban status", zap.String("ip", ip), zap.Error(err))
			case banned > 0:
				return c.SendStatus(fiber.StatusForbidden)
			default:
				cleanCache.MarkClean(ip)
			}
		}

		nextErr := c.Next()

		if isRouteNotFound(nextErr) {
			// Do not treat common browser/crawler probes as route scanning.
			if _, skip := ignoredPaths[c.Path()]; skip {
				return nextErr
			}
			strikeKey := fmt.Sprintf(constant.ScannerStrikeKeyTemplate, ip)

			result, luaErr := scannerGuardLua.Run(
				ctx, cache,
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
				cleanCache.Invalidate(ip)
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
