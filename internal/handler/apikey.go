// Package handler API Key 处理器
package handler

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	"github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// APIKeyHandler API Key 处理器
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type APIKeyHandler interface {
	HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error)
	HandleListAPIKeys(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeysRsp], error)
	HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type apiKeyHandler struct {
	issue  command.IssueAPIKeyHandler
	revoke command.RevokeAPIKeyHandler
	list   query.ListAPIKeysHandler
}

// NewAPIKeyHandler 创建 API Key 处理器
//
//	@return APIKeyHandler
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func NewAPIKeyHandler() APIKeyHandler {
	apiKeyRepo := repository.NewAPIKeyRepository()
	userRepo := repository.NewUserRepository()
	generator := apikeyservice.NewAPIKeyGenerator()
	userExistsCh := command.NewUserExistenceChecker(userRepo)

	return &apiKeyHandler{
		issue:  command.NewIssueAPIKeyHandler(apiKeyRepo, generator, userExistsCh),
		revoke: command.NewRevokeAPIKeyHandler(apiKeyRepo),
		list:   query.NewListAPIKeysHandler(apiKeyRepo),
	}
}

// HandleCreateAPIKey 创建 API Key
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.CreateAPIKeyReq
//	@return *dto.HTTPResponse[*dto.CreateAPIKeyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *apiKeyHandler) HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error) {
	rsp := &dto.CreateAPIKeyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.issue.Handle(ctx, command.IssueAPIKeyCommand{
		UserID: userID,
		Name:   req.Body.Name,
	})
	if err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.Key = &dto.APIKeyDetail{
		ID:        result.KeyID,
		Name:      result.Name,
		Key:       result.Secret,
		CreatedAt: result.CreatedAt,
	}
	return util.WrapHTTPResponse(rsp, nil)
}

// HandleListAPIKeys 列出当前用户的 API Keys（admin 可见全量）
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.ListAPIKeysRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *apiKeyHandler) HandleListAPIKeys(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeysRsp], error) {
	rsp := &dto.ListAPIKeysRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	views, err := h.list.Handle(ctx, query.ListAPIKeysQuery{
		RequesterID:         userID,
		RequesterPermission: permission,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[APIKeyHandler] List api keys failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}

	rsp.Keys = make([]*dto.APIKeyItem, 0, len(views))
	for _, v := range views {
		rsp.Keys = append(rsp.Keys, &dto.APIKeyItem{
			ID:        v.ID,
			Name:      v.Name,
			Key:       v.MaskedKey,
			CreatedAt: v.CreatedAt,
		})
	}
	return util.WrapHTTPResponse(rsp, nil)
}

// HandleDeleteAPIKey 删除指定 API Key
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.DeleteAPIKeyReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (h *apiKeyHandler) HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	err := h.revoke.Handle(ctx, command.RevokeAPIKeyCommand{
		KeyID:               req.ID,
		RequesterID:         userID,
		RequesterPermission: permission,
	})
	if err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}
	return util.WrapHTTPResponse(rsp, nil)
}
