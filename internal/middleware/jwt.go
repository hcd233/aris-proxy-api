// Package middleware 中间件
//
//	update 2024-06-22 11:05:33
package middleware

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// jwtUserCache Redis 中缓存用户信息所用的结构体（仅包含 JWT 中间件需要的字段）
type jwtUserCache struct {
	Name       string          `json:"name"`
	Permission enum.Permission `json:"permission"`
}

// jwtUserCacheKey 构造 Redis key
func jwtUserCacheKey(userID uint) string {
	return fmt.Sprintf(constant.JWTUserCacheKeyTemplate, userID)
}

// JwtMiddleware JWT 中间件
//
// 优先从 Redis 缓存读取用户信息，未命中时查询数据库并写入缓存，缓存 TTL 与 AccessToken 过期时间一致。
//
//	@param db *gorm.DB
//	@param cache
//	@return ctx
//	@return next
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-05-13 11:44:46
func JwtMiddleware(db *gorm.DB, cache *redis.Client) func(ctx huma.Context, next func(huma.Context)) {
	accessTokenSvc := jwt.GetAccessTokenSigner()

	return func(ctx huma.Context, next func(huma.Context)) {
		log := logger.WithCtx(ctx.Context())
		if db == nil {
			log.Error("[JwtMiddleware] DB dependency is nil")
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}

		userID, err := extractToken(ctx, accessTokenSvc)
		if err != nil {
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrJWTDecode.BizError()))
			return
		}

		name, permission, err := resolveJWTUser(ctx.Context(), db, cache, userID)
		if err != nil {
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrDBQuery.BizError()))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, userID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, name)
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, permission)
		next(ctx)
	}
}

func extractToken(ctx huma.Context, accessTokenSvc jwt.TokenSigner) (uint, error) {
	tokenString := cmp.Or(ctx.Header(constant.HTTPLowerHeaderAuthorization), ctx.Header(constant.HTTPTitleHeaderAuthorization))
	tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), constant.HTTPAuthBearerPrefix))
	if tokenString == "" {
		return 0, ierr.ErrUnauthorized
	}
	return accessTokenSvc.DecodeToken(tokenString)
}

func resolveJWTUser(ctx context.Context, db *gorm.DB, cache *redis.Client, userID uint) (string, enum.Permission, error) {
	var name string
	var permission enum.Permission

	cacheKey := jwtUserCacheKey(userID)
	if cached := loadJWTUserCache(ctx, cache, cacheKey); cached != nil {
		return cached.Name, cached.Permission, nil
	}

	userDAO := dao.GetUserDAO()
	reqDB := db.WithContext(ctx)
	user, dbErr := userDAO.Get(reqDB, &model.User{ID: userID}, constant.UserRepoFieldsAuth)
	if dbErr != nil {
		return "", "", dbErr
	}
	name = user.Name
	permission = user.Permission

	saveJWTUserCache(ctx, cache, cacheKey, name, permission)

	return name, permission, nil
}

func loadJWTUserCache(ctx context.Context, cache *redis.Client, cacheKey string) *jwtUserCache {
	if cache == nil {
		return nil
	}
	raw, redisErr := cache.Get(ctx, cacheKey).Bytes()
	if redisErr != nil {
		return nil
	}
	var cached jwtUserCache
	if err := sonic.Unmarshal(raw, &cached); err != nil {
		return nil
	}
	return &cached
}

func saveJWTUserCache(ctx context.Context, cache *redis.Client, cacheKey, name string, permission enum.Permission) {
	if cache == nil {
		return
	}
	cacheVal, marshalErr := sonic.Marshal(&jwtUserCache{Name: name, Permission: permission})
	if marshalErr != nil {
		return
	}
	if setErr := cache.Set(ctx, cacheKey, cacheVal, config.JwtAccessTokenExpired).Err(); setErr != nil {
		logger.WithCtx(ctx).Warn("[JwtMiddleware] Failed to cache user info", zap.Error(setErr))
	}
}
