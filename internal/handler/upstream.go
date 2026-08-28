package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	upstreamport "github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// UpstreamHandler Upstream 分组视图 HTTP 处理器
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamHandler interface {
	HandleListUpstream(ctx context.Context, req *dto.ListUpstreamReq) (*dto.HTTPResponse[*dto.ListUpstreamRsp], error)
}

// UpstreamDependencies Upstream 处理器依赖
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamDependencies struct {
	List upstreamport.ListUpstreamHandler
}

type upstreamHandler struct {
	list upstreamport.ListUpstreamHandler
}

// NewUpstreamHandler 构造 Upstream HTTP 处理器
func NewUpstreamHandler(deps UpstreamDependencies) UpstreamHandler {
	return &upstreamHandler{list: deps.List}
}

// HandleListUpstream 处理分组列表查询
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
func (h *upstreamHandler) HandleListUpstream(ctx context.Context, req *dto.ListUpstreamReq) (*dto.HTTPResponse[*dto.ListUpstreamRsp], error) {
	rsp := &dto.ListUpstreamRsp{}

	perm := util.CtxValuePermission(ctx)
	isGlobalScope := perm == enum.PermissionAdmin
	groups, modelTotal, pageInfo, err := h.list.Handle(ctx, upstreamport.ListUpstreamQuery{
		CommonParam: req.CommonParam,
		IsDemo:      perm == enum.PermissionDemo,
		ScopeUserID: lo.Ternary(isGlobalScope, 0, util.CtxValueUint(ctx, constant.CtxKeyUserID)),
		Username:    req.Username,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[UpstreamHandler] List upstream failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Groups = lo.Map(groups, func(g *upstreamport.UpstreamGroupView, _ int) *dto.UpstreamGroupItem {
		return &dto.UpstreamGroupItem{
			Endpoint:        toUpstreamEndpointItem(g.Endpoint),
			Models:          lo.Map(g.Models, func(m *upstreamport.UpstreamModelView, _ int) *dto.UpstreamModelItem { return toUpstreamModelItem(m) }),
			ModelCount:      g.ModelCount,
			TotalModelCount: g.TotalModelCount,
			Truncated:       g.Truncated,
		}
	})
	rsp.ModelTotal = modelTotal
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func toUpstreamUserItem(u *upstreamport.UpstreamUserView) *dto.UpstreamUserItem {
	if u == nil {
		return nil
	}
	return &dto.UpstreamUserItem{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
}

func toUpstreamEndpointItem(v *upstreamport.UpstreamEndpointView) *dto.UpstreamEndpointItem {
	if v == nil {
		return nil
	}
	return &dto.UpstreamEndpointItem{
		ID:                          v.ID,
		User:                        toUpstreamUserItem(v.User),
		Name:                        v.Name,
		OpenaiBaseURL:               v.OpenaiBaseURL,
		AnthropicBaseURL:            v.AnthropicBaseURL,
		MaskedAPIKey:                v.MaskedAPIKey,
		SupportOpenAIChatCompletion: v.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       v.SupportOpenAIResponse,
		SupportAnthropicMessage:     v.SupportAnthropicMessage,
		CreatedAt:                   v.CreatedAt,
		UpdatedAt:                   v.UpdatedAt,
	}
}

func toUpstreamModelItem(v *upstreamport.UpstreamModelView) *dto.UpstreamModelItem {
	return &dto.UpstreamModelItem{
		ID:              v.ID,
		User:            toUpstreamUserItem(v.User),
		Alias:           v.Alias,
		ModelID:         v.ModelID,
		UpstreamModel:   v.UpstreamModel,
		Enabled:         v.Enabled,
		ContextLength:   v.ContextLength,
		MaxOutputTokens: v.MaxOutputTokens,
		Capabilities:    v.Capabilities,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}
