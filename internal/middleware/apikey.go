package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"

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

// APIKeyAuth API Key 鉴权载荷（不含密钥本身，可安全写入 Redis）
type APIKeyAuth struct {
	APIKeyID   uint   `json:"apiKeyId"`
	APIKeyName string `json:"apiKeyName"`
	UserID     uint   `json:"userId"`
}

// ResolveAPIKeyAuth 解析 API Key 归属：缓存优先，未命中回源 DB 并写回。
//
// 导出供回归测试断言「命中缓存后不再查库」（同 repository.FormatBigIntArray 的约定）。
func ResolveAPIKeyAuth(ctx context.Context, db *gorm.DB, cache *redis.Client, token string) (*APIKeyAuth, error) {
	cacheKey := apiKeyAuthCacheKey(token)
	if cached := loadAPIKeyAuthCache(ctx, cache, cacheKey); cached != nil {
		return cached, nil
	}

	record, err := dao.GetProxyAPIKeyDAO().Get(db.WithContext(ctx), &dbmodel.ProxyAPIKey{Key: token}, constant.ProxyAPIKeyRepoFieldsAuth)
	if err != nil {
		return nil, err
	}

	auth := &APIKeyAuth{APIKeyID: record.ID, APIKeyName: record.Name, UserID: record.UserID}
	saveAPIKeyAuthCache(ctx, cache, cacheKey, auth)
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
	if setErr := cache.Set(ctx, cacheKey, payload, constant.APIKeyAuthCacheTTL).Err(); setErr != nil {
		logger.WithCtx(ctx).Warn("[APIKeyMiddleware] Failed to cache api key auth", zap.Error(setErr))
	}
}

// APIKeyMiddleware API Key 验证中间件
//
// 鉴权结果（key 归属 + 用户信息）走 Redis 缓存：命中时每请求 0 次 DB 查询，
// 未命中时 2 次（key 一次、用户一次，用户部分复用 JwtMiddleware 的 jwt:user:{id} 缓存）。
//
//	@param db *gorm.DB
//	@param cache *redis.Client 可为 nil（退化为每请求直查 DB）
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-10-22 15:00:00
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
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key not found", zap.Error(err))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
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
