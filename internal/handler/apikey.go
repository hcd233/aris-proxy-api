// Package handler API Key 处理器
package handler

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	"github.com/hcd233/aris-proxy-api/internal/application/apikey/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
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

// APIKeyDependencies APIKeyHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type APIKeyDependencies struct {
	Issue  command.IssueAPIKeyHandler
	Revoke command.RevokeAPIKeyHandler
	List   query.ListAPIKeysHandler
}

type apiKeyHandler struct {
	issue  command.IssueAPIKeyHandler
	revoke command.RevokeAPIKeyHandler
	list   query.ListAPIKeysHandler
}

// NewAPIKeyHandler 创建 API Key 处理器
//
//	@param deps APIKeyDependencies 依赖项（由调用方注入，避免 handler 直接实例化 infrastructure）
//	@return APIKeyHandler
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewAPIKeyHandler(deps APIKeyDependencies) APIKeyHandler {
	return &apiKeyHandler{
		issue:  deps.Issue,
		revoke: deps.Revoke,
		list:   deps.List,
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

	// DTO 级别输入校验
	if strings.TrimSpace(req.Body.Name) == "" {
		logger.WithCtx(ctx).Warn("[APIKeyHandler] Validation failed: empty api key name")
		rsp.Error = ierr.ErrValidation.BizError()
		return util.WrapHTTPResponse(rsp, nil)
	}

	result, err := h.issue.Handle(ctx, command.IssueAPIKeyCommand{
		UserID: userID,
		Name:   req.Body.Name,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[APIKeyHandler] Create api key failed", zap.Error(err))
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

	// DTO 级别输入校验
	if req.ID == 0 {
		logger.WithCtx(ctx).Warn("[APIKeyHandler] Validation failed: invalid api key id")
		rsp.Error = ierr.ErrValidation.BizError()
		return util.WrapHTTPResponse(rsp, nil)
	}

	err := h.revoke.Handle(ctx, command.RevokeAPIKeyCommand{
		KeyID:               req.ID,
		RequesterID:         userID,
		RequesterPermission: permission,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[APIKeyHandler] Delete api key failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return util.WrapHTTPResponse(rsp, nil)
	}
	return util.WrapHTTPResponse(rsp, nil)
}
