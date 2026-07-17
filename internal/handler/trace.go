// Package handler Trace 处理器
package handler

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TraceHandler Trace 处理器接口
type TraceHandler interface {
	HandleReportTraceEvent(ctx context.Context, req *dto.ReportTraceEventReq) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error)
	HandleListTraces(ctx context.Context, req *dto.ListTracesReq) (*dto.HTTPResponse[*dto.ListTracesRsp], error)
	HandleGetTrace(ctx context.Context, req *dto.GetTraceReq) (*dto.HTTPResponse[*dto.GetTraceRsp], error)
	HandleListTraceEvents(ctx context.Context, req *dto.ListTraceEventsReq) (*dto.HTTPResponse[*dto.ListTraceEventsRsp], error)
	HandleGetTraceConversation(ctx context.Context, req *dto.GetTraceConversationReq) (*dto.HTTPResponse[*dto.GetTraceConversationRsp], error)
}

// TraceDependencies TraceHandler 依赖项
type TraceDependencies struct {
	Report       port.ReportTraceEventHandler
	List         port.ListTracesHandler
	Get          port.GetTraceHandler
	Events       port.ListTraceEventsHandler
	Conversation port.ListTraceConversationHandler
}

type traceHandler struct {
	report       port.ReportTraceEventHandler
	list         port.ListTracesHandler
	get          port.GetTraceHandler
	events       port.ListTraceEventsHandler
	conversation port.ListTraceConversationHandler
}

// NewTraceHandler 构造 TraceHandler
func NewTraceHandler(deps TraceDependencies) TraceHandler {
	return &traceHandler{report: deps.Report, list: deps.List, get: deps.Get, events: deps.Events, conversation: deps.Conversation}
}

// HandleGetTraceConversation 获取 Trace 对话投影（JWT）。
func (h *traceHandler) HandleGetTraceConversation(ctx context.Context, req *dto.GetTraceConversationReq) (*dto.HTTPResponse[*dto.GetTraceConversationRsp], error) {
	rsp := &dto.GetTraceConversationRsp{}
	permission := util.CtxValuePermission(ctx)
	view, err := h.conversation.Handle(ctx, port.ListTraceConversationQuery{
		UserID: util.CtxValueUint(ctx, constant.CtxKeyUserID), IsAdmin: permission.Level() >= enum.PermissionAdmin.Level(), TraceID: req.TraceID,
	})
	if err != nil {
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	turns := lo.Map(view.Turns, func(turn *port.TraceConversationTurnView, _ int) *dto.TraceConversationTurn {
		return &dto.TraceConversationTurn{TurnID: turn.TurnID, Items: lo.Map(turn.Items, func(item *port.TraceConversationItemView, _ int) *dto.TraceConversationItem {
			return &dto.TraceConversationItem{Kind: item.Kind, Role: item.Role, Content: item.Content, ToolName: item.ToolName, CallID: item.CallID, Arguments: item.Arguments, Output: item.Output, Source: item.Source, RecordIDs: item.RecordIDs}
		})}
	})
	rsp.Conversation = &dto.TraceConversation{TraceID: view.TraceID, SessionID: view.SessionID, Turns: turns}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleReportTraceEvent 上报 codex hook 事件（API Key 鉴权）
func (h *traceHandler) HandleReportTraceEvent(ctx context.Context, req *dto.ReportTraceEventReq) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error) {
	rsp := &dto.ReportTraceEventRsp{}
	if req.Body == nil {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	apiKeyName := util.CtxValueString(ctx, constant.CtxKeyAPIKeyName)
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	cmd := port.ReportTraceEventCommand{
		HookEventName: req.Body.HookEventName,
		SessionID:     req.Body.SessionID,
		Model:         req.Body.Model,
		CWD:           req.Body.CWD,
		Source:        req.Body.Source,
		TurnID:        req.Body.TurnID,
		APIKeyName:    apiKeyName,
		UserID:        userID,
	}
	if len(req.Body.Records) > 0 {
		cmd.Records = lo.Map(req.Body.Records, func(record *dto.ReportTraceRecordReq, _ int) port.ReportTraceRecord {
			return port.ReportTraceRecord{
				Source:         record.Source,
				RecordType:     record.RecordType,
				HookEventName:  record.HookEventName,
				Event:          record.Event,
				TurnID:         record.TurnID,
				CallID:         record.CallID,
				TranscriptLine: record.TranscriptLine,
				ClientSequence: record.ClientSequence,
				DedupKey:       record.DedupKey,
				Payload:        record.Payload,
			}
		})
	} else {
		// Legacy single-event clients are normalized by the application handler.
		rawPayload, err := sonic.Marshal(req.Body)
		if err != nil {
			logger.WithCtx(ctx).Error("[TraceHandler] Marshal report body failed", zap.Error(err))
			rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
			return apiutil.WrapHTTPResponse(rsp, nil)
		}
		cmd.RawPayload = rawPayload
	}
	if err := h.report.Handle(ctx, cmd); err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] Report event failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListTraces 列出当前用户 traces（JWT）
func (h *traceHandler) HandleListTraces(ctx context.Context, req *dto.ListTracesReq) (*dto.HTTPResponse[*dto.ListTracesRsp], error) {
	rsp := &dto.ListTracesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	views, pageInfo, err := h.list.Handle(ctx, port.ListTracesQuery{UserID: userID, IsAdmin: isAdmin, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] List traces failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Traces = lo.Map(views, func(item *port.TraceSummaryView, _ int) *dto.TraceSummary {
		return &dto.TraceSummary{
			ID: item.ID, SessionID: item.SessionID, Agent: item.Agent, APIKeyName: item.APIKeyName,
			Model: item.Model, Source: item.Source, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetTrace 获取 trace 详情（JWT）
func (h *traceHandler) HandleGetTrace(ctx context.Context, req *dto.GetTraceReq) (*dto.HTTPResponse[*dto.GetTraceRsp], error) {
	rsp := &dto.GetTraceRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.get.Handle(ctx, port.GetTraceQuery{UserID: userID, IsAdmin: isAdmin, TraceID: req.TraceID})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] Get trace failed", zap.Uint("traceID", req.TraceID), zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Trace = &dto.TraceDetail{
		ID: view.ID, SessionID: view.SessionID, Agent: view.Agent, APIKeyName: view.APIKeyName,
		Model: view.Model, CWD: view.CWD, Source: view.Source, Status: view.Status,
		Metadata: view.Metadata, EventCount: view.EventCount, CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListTraceEvents 列出 trace 事件时间线（JWT）
func (h *traceHandler) HandleListTraceEvents(ctx context.Context, req *dto.ListTraceEventsReq) (*dto.HTTPResponse[*dto.ListTraceEventsRsp], error) {
	rsp := &dto.ListTraceEventsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	views, pageInfo, err := h.events.Handle(ctx, port.ListTraceEventsQuery{UserID: userID, IsAdmin: isAdmin, TraceID: req.TraceID, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] List trace events failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Events = lo.Map(views, func(item *port.TraceEventView, _ int) *dto.TraceEventItem {
		return &dto.TraceEventItem{
			ID:             item.ID,
			Source:         item.Source,
			RecordType:     item.RecordType,
			Event:          item.Event,
			TurnID:         item.TurnID,
			CallID:         item.CallID,
			TranscriptLine: item.TranscriptLine,
			ClientSequence: item.ClientSequence,
			DedupKey:       item.DedupKey,
			Payload:        sonic.NoCopyRawMessage(item.Payload),
			CreatedAt:      item.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}
