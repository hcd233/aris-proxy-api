// Package handler Trace 处理器
package handler

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
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
	HandleIssueTraceClientTicket(ctx context.Context, req *dto.IssueTraceClientTicketReq) (*dto.HTTPResponse[*dto.IssueTraceClientTicketRsp], error)
	HandleDownloadTraceClient(ctx context.Context, req *dto.DownloadTraceClientReq) (*huma.StreamResponse, error)
	HandleCheckTraceClient(ctx context.Context, req *dto.CheckTraceClientReq) (*huma.StreamResponse, error)
}

// TraceDependencies TraceHandler 依赖项
type TraceDependencies struct {
	Report           port.ReportTraceEventHandler
	List             port.ListTracesHandler
	Get              port.GetTraceHandler
	Events           port.ListTraceEventsHandler
	Conversation     port.ListTraceConversationHandler
	IssueTicket      port.IssueTraceClientTicketHandler
	ArtifactResolver port.TraceClientArtifactResolver
}

type traceHandler struct {
	report           port.ReportTraceEventHandler
	list             port.ListTracesHandler
	get              port.GetTraceHandler
	events           port.ListTraceEventsHandler
	conversation     port.ListTraceConversationHandler
	issueTicket      port.IssueTraceClientTicketHandler
	artifactResolver port.TraceClientArtifactResolver
}

// NewTraceHandler 构造 TraceHandler
func NewTraceHandler(deps TraceDependencies) TraceHandler {
	return &traceHandler{
		report:           deps.Report,
		list:             deps.List,
		get:              deps.Get,
		events:           deps.Events,
		conversation:     deps.Conversation,
		issueTicket:      deps.IssueTicket,
		artifactResolver: deps.ArtifactResolver,
	}
}

// HandleIssueTraceClientTicket 签发短期单次客户端下载票据。
func (h *traceHandler) HandleIssueTraceClientTicket(
	ctx context.Context,
	_ *dto.IssueTraceClientTicketReq,
) (*dto.HTTPResponse[*dto.IssueTraceClientTicketRsp], error) {
	rsp := &dto.IssueTraceClientTicketRsp{}
	view, err := h.issueTicket.Handle(ctx, port.IssueTraceClientTicketCommand{
		UserID: util.CtxValueUint(ctx, constant.CtxKeyUserID),
	})
	if err != nil {
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Ticket = view.Ticket
	rsp.ExpiresAt = view.ExpiresAt
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleDownloadTraceClient 下载白名单平台的客户端二进制。
func (h *traceHandler) HandleDownloadTraceClient(
	ctx context.Context,
	req *dto.DownloadTraceClientReq,
) (*huma.StreamResponse, error) {
	artifact, err := h.artifactResolver.Resolve(req.OS, req.Arch)
	if err != nil {
		return nil, err
	}
	log := logger.WithCtx(ctx)
	return &huma.StreamResponse{Body: func(humaCtx huma.Context) {
		file, openErr := os.Open(artifact.Path)
		if openErr != nil {
			log.Error("[TraceHandler] Failed to open trace client artifact", zap.Error(openErr))
			_ = apiutil.WriteErrorHTTPResponse( //nolint:errcheck // already in error path
				humaCtx,
				fiber.StatusInternalServerError,
				ierr.ErrInternal.BizError(),
			)
			return
		}
		defer func() { _ = file.Close() }() //nolint:errcheck // best-effort close

		humaCtx.SetStatus(fiber.StatusOK)
		humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.MIMETypeOctetStream)
		humaCtx.SetHeader(
			constant.HTTPHeaderContentDisposition,
			fmt.Sprintf(constant.HTTPAttachmentFilenameTemplate, artifact.Filename),
		)
		humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
		humaCtx.SetHeader(
			constant.HTTPHeaderContentLength,
			strconv.FormatInt(artifact.Size, constant.DecimalBase),
		)
		if _, copyErr := io.Copy(humaCtx.BodyWriter(), file); copyErr != nil {
			log.Error("[TraceHandler] Failed to stream trace client artifact", zap.Error(copyErr))
		}
	}}, nil
}

// HandleCheckTraceClient validates the API key through middleware.
func (h *traceHandler) HandleCheckTraceClient(
	_ context.Context,
	_ *dto.CheckTraceClientReq,
) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(ctx huma.Context) {
		ctx.SetStatus(fiber.StatusNoContent)
	}}, nil
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
func (h *traceHandler) HandleReportTraceEvent(
	ctx context.Context,
	req *dto.ReportTraceEventReq,
) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error) {
	rsp := &dto.ReportTraceEventRsp{}
	if req.Body == nil || (len(req.Body.Records) == 0 && req.Body.HookEventName == "") {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	cmd := port.ReportTraceEventCommand{
		HookEventName: req.Body.HookEventName,
		SessionID:     req.Body.SessionID,
		Model:         req.Body.Model,
		CWD:           req.Body.CWD,
		Source:        req.Body.Source,
		TurnID:        req.Body.TurnID,
		APIKeyName:    util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		UserID:        util.CtxValueUint(ctx, constant.CtxKeyUserID),
	}
	if len(req.Body.Records) > 0 {
		cmd.Records = lo.Map(req.Body.Records, func(
			record *dto.ReportTraceRecordReq,
			_ int,
		) port.ReportTraceRecord {
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
		cmd.RawPayload = req.Body.Raw
		if len(cmd.RawPayload) == 0 {
			rawPayload, err := sonic.Marshal(req.Body)
			if err != nil {
				logger.WithCtx(ctx).Error("[TraceHandler] Marshal report body failed", zap.Error(err))
				rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
				return apiutil.WrapHTTPResponse(rsp, nil)
			}
			cmd.RawPayload = rawPayload
		}
	}

	results, err := h.report.Handle(ctx, cmd)
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] Report event failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Results = lo.Map(results, func(
		result port.ReportTraceRecordResult,
		_ int,
	) *dto.ReportTraceRecordResult {
		return &dto.ReportTraceRecordResult{
			DedupKey: result.DedupKey,
			Status:   result.Status,
			Message:  result.Message,
		}
	})
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
