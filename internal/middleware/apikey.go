package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// apiKeyAuthGroup 合并同一密钥的并发回源
//
// TTL 到期瞬间同一热 Key 的多个请求会同时 miss，没有合并就会一起打 DB；
// 键含密钥摘要，天然按密钥隔离。
var apiKeyAuthGroup singleflight.Group

// APIKeyAuth API Key 鉴权载荷（不含密钥本身，可安全写入 Redis）
//
//	@author centonhuang
//	@update 2026-10-22 20:00:00
type APIKeyAuth struct {
	APIKeyID   uint   `json:"apiKeyId"`
	APIKeyName string `json:"apiKeyName"`
	UserID     uint   `json:"userId"`
}

// ResolveAPIKeyAuth 解析 API Key 归属：缓存优先，未命中经 singleflight 回源 DB 并写回。
//
// 导出供回归测试断言「命中缓存后不再查库」（同 repository.FormatBigIntArray 的约定）。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param cache *redis.Client 可为 nil（退化为每次查库）
//	@param token string 明文 API Key
//	@return *APIKeyAuth
//	@return error
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func ResolveAPIKeyAuth(ctx context.Context, db *gorm.DB, cache *redis.Client, token string) (*APIKeyAuth, error) {
	cacheKey := apiKeyAuthCacheKey(token)
	if cached := loadAPIKeyAuthCache(ctx, cache, cacheKey); cached != nil {
		return cached, nil
	}

	shared, err, _ := apiKeyAuthGroup.Do(cacheKey, func() (any, error) {
		// 脱离请求取消：首个请求被客户端断开时，不应把同批等待者的回源一起废掉
		loadCtx := context.WithoutCancel(ctx)
		if cached := loadAPIKeyAuthCache(loadCtx, cache, cacheKey); cached != nil {
			return cached, nil
		}

		record, dbErr := dao.GetProxyAPIKeyDAO().Get(db.WithContext(loadCtx), &dbmodel.ProxyAPIKey{Key: token}, constant.ProxyAPIKeyRepoFieldsAuth)
		if dbErr != nil {
			return nil, dbErr
		}

		auth := &APIKeyAuth{APIKeyID: record.ID, APIKeyName: record.Name, UserID: record.UserID}
		saveAPIKeyAuthCache(loadCtx, cache, cacheKey, auth)
		return auth, nil
	})
	if err != nil {
		return nil, err
	}
	auth, ok := shared.(*APIKeyAuth)
	if !ok {
		return nil, ierr.New(ierr.ErrInternal, "unexpected api key auth payload")
	}
	return auth, nil
}

// apiKeyAuthCacheKey 用密钥的 SHA-256 摘要做缓存键，明文密钥不落 Redis keyspace。
func apiKeyAuthCacheKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf(constant.APIKeyAuthKeyTemplate, hex.EncodeToString(sum[:]))
}

// loadAPIKeyAuthCache 读缓存；未命中、反序列化失败或 Redis 故障一律返回 nil（fail-open 回源 DB）。
func loadAPIKeyAuthCache(ctx context.Context, cache *redis.Client, cacheKey string) *APIKeyAuth {
	if cache == nil {
		return nil
	}
	raw, err := cache.Get(ctx, cacheKey).Bytes()
	if err != nil {
		return nil
	}
	var auth APIKeyAuth
	if unmarshalErr := sonic.Unmarshal(raw, &auth); unmarshalErr != nil {
		return nil
	}
	return &auth
}

// saveAPIKeyAuthCache 写缓存；失败只影响后续命中率，不影响本次鉴权结果。
func saveAPIKeyAuthCache(ctx context.Context, cache *redis.Client, cacheKey string, auth *APIKeyAuth) {
	if cache == nil {
		return
	}
	payload, err := sonic.Marshal(auth)
	if err != nil {
		return
	}
	if setErr := cache.Set(ctx, cacheKey, payload, apiKeyAuthCacheTTL()).Err(); setErr != nil {
		logger.WithCtx(ctx).Warn("[APIKeyMiddleware] Failed to cache api key auth", zap.Error(setErr))
	}
}

// apiKeyAuthCacheTTL 缓存有效期 + 随机抖动
//
// 抖动把「同批写入的条目同时过期」摊平：进程重启预热、压测同时打同一批 Key 时，
// 固定 TTL 会让整批条目在同一秒失效并把回源压成一个尖峰。语义与
// cache.ReadCache 的 TTL 抖动一致。
//
// 抖动上限为 0 时直接返回基准值：rand.N 在 n<=0 时会 panic，而这条路径在鉴权热路径上。
func apiKeyAuthCacheTTL() time.Duration {
	if constant.APIKeyAuthCacheTTLJitter <= 0 {
		return constant.APIKeyAuthCacheTTL
	}
	return constant.APIKeyAuthCacheTTL + rand.N(constant.APIKeyAuthCacheTTLJitter) //nolint:gosec // G404 TTL 抖动无需密码学随机
}

// APIKeyAuthFailureStatus 把鉴权回源失败映射为 HTTP 状态码。
//
// Key 不存在（gorm.ErrRecordNotFound）是客户端问题 → 401；
// DB 不可用等其他错误是服务端问题 → 503，让 SDK 重试而不是误以为 Key 失效。
// 导出仅供回归测试断言（同 ResolveAPIKeyAuth 的约定）。
//
//	@param err ResolveAPIKeyAuth 返回的错误
//	@return int HTTP 状态码
//	@author centonhuang
//	@update 2026-10-23 01:30:00
func APIKeyAuthFailureStatus(err error) int {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.StatusUnauthorized
	}
	return fiber.StatusServiceUnavailable
}

// APIKeyMiddleware API Key 验证中间件
//
// 鉴权结果（key 归属 + 用户信息）走 Redis 缓存：命中时每请求 0 次 DB 查询，
// 未命中时 2 次（key 一次、用户一次，用户部分复用 JwtMiddleware 的 jwt:user:{id} 缓存）。
// TTL 带抖动、并发 miss 经 singleflight 合并，避免同批条目同时过期造成回源尖峰。
//
//	@param db *gorm.DB
//	@param cache *redis.Client 可为 nil（退化为每请求直查 DB）
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-10-22 20:00:00
func APIKeyMiddleware(db *gorm.DB, cache *redis.Client) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := ctx.Header(constant.HTTPHeaderAuthorization)
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), constant.HTTPAuthBearerPrefix))

		if tokenString == "" {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key is empty")
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		if db == nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] DB dependency is nil")
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		auth, err := ResolveAPIKeyAuth(ctx.Context(), db, cache, tokenString)
		if err != nil {
			status := APIKeyAuthFailureStatus(err)
			if status == fiber.StatusUnauthorized {
				logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key not found", zap.Error(err))
				lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, status, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
				return
			}
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] Failed to load API key auth", zap.Error(err))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, status, ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		// 无主 key（user_id=0）防御：零值 userID 会命中主键最小的用户，
		// 把请求错误认成别人的租户——多租户化后必须在查用户前显式拒绝。
		if auth.UserID == 0 {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] Orphan API key rejected (user_id=0)",
				zap.Uint("apiKeyID", auth.APIKeyID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		userName, permission, userErr := resolveJWTUser(ctx.Context(), db, cache, auth.UserID)
		if errors.Is(userErr, ierr.ErrUnauthorized) {
			// 用户已被删除：语义是未授权，与 JwtMiddleware 的映射保持一致，不能落成 500
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key owner no longer exists",
				zap.Uint("userID", auth.UserID), zap.Uint("apiKeyID", auth.APIKeyID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}
		if userErr != nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] Failed to resolve user",
				zap.Error(userErr), zap.Uint("userID", auth.UserID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		// Demo 账户的存量 API Key 一律拒绝：demo 只读受限，不允许调用 LLM 转发
		if permission == enum.PermissionDemo {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] Demo user API key rejected",
				zap.Uint("userID", auth.UserID), zap.Uint("apiKeyID", auth.APIKeyID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, auth.UserID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, userName)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, auth.APIKeyID)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyName, auth.APIKeyName)
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header(constant.HTTPHeaderUserAgent))

		next(ctx)
	}
}
