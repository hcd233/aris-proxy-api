// Package middleware 中间件
//
//	update 2024-06-22 11:05:33
package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/jwt"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// JwtMiddleware JWT 中间件
//
//	@return ctx huma.Context
//	@return next func(huma.Context)
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2025-11-02 04:17:04
func JwtMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	dao := dao.GetUserDAO()
	accessTokenSvc := jwt.GetAccessTokenSigner()

	return func(ctx huma.Context, next func(huma.Context)) {
		db := database.GetDBInstance(ctx.Context())

		tokenString := ctx.Header("Authorization")
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), "Bearer "))
		if tokenString == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
			return
		}
		userID, err := accessTokenSvc.DecodeToken(tokenString)
		if err != nil {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrJWTDecode.BizError()))
			return
		}
		user, err := dao.Get(db, &model.User{ID: userID}, []string{"id", "name", "permission"})
		if err != nil {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrDBQuery.BizError()))
			return
		}
		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, user.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, user.Name)
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, user.Permission)
		next(ctx)
	}
}
