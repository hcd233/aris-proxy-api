// Package middleware 中间件
//
//	update 2024-06-22 11:05:33
package middleware

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
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
//	@param rdb *redis.Client
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func JwtMiddleware(db *gorm.DB, rdb *redis.Client) func(ctx huma.Context, next func(huma.Context)) {
	userDAO := dao.GetUserDAO()
	accessTokenSvc := jwt.GetAccessTokenSigner()

	return func(ctx huma.Context, next func(huma.Context)) {
		log := logger.WithCtx(ctx.Context())
		if db == nil {
			log.Error("[JwtMiddleware] DB dependency is nil")
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrInternal.BizError()))
			return
		}
		reqDB := db.WithContext(ctx.Context())

		tokenString := cmp.Or(ctx.Header(constant.HTTPLowerHeaderAuthorization), ctx.Header(constant.HTTPTitleHeaderAuthorization))
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), constant.HTTPAuthBearerPrefix))
		if tokenString == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
			return
		}
		userID, err := accessTokenSvc.DecodeToken(tokenString)
		if err != nil {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrJWTDecode.BizError()))
			return
		}

		var name string
		var permission enum.Permission
		cacheHit := false

		cacheKey := jwtUserCacheKey(userID)
		if rdb != nil {
			if raw, redisErr := rdb.Get(ctx.Context(), cacheKey).Bytes(); redisErr == nil {
				var cached jwtUserCache
				if unmarshalErr := sonic.Unmarshal(raw, &cached); unmarshalErr == nil {
					name = cached.Name
					permission = cached.Permission
					cacheHit = true
				}
			}
		}

		if !cacheHit {
			user, dbErr := userDAO.Get(reqDB, &model.User{ID: userID}, constant.UserRepoFieldsAuth)
			if dbErr != nil {
				lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrDBQuery.BizError()))
				return
			}
			name = user.Name
			permission = user.Permission

			if rdb != nil {
				if cacheVal, marshalErr := sonic.Marshal(&jwtUserCache{Name: name, Permission: permission}); marshalErr == nil {
					if setErr := rdb.Set(ctx.Context(), cacheKey, cacheVal, config.JwtAccessTokenExpired).Err(); setErr != nil {
						log.Warn("[JwtMiddleware] Failed to cache user info", zap.Uint("userID", userID), zap.Error(setErr))
					}
				}
			}
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, userID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, name)
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, permission)
		next(ctx)
	}
}
