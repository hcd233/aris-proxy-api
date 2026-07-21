// Package handler Trace 处理器
package handler

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"net/url"
	"text/template"

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

//go:embed install_trace_client.sh.tmpl
var installScriptTemplate string

var installScriptTmpl = template.Must(template.New(constant.TraceClientInstallScriptTmplName).Parse(installScriptTemplate))

type installScriptData struct {
	Host string
}

// TraceHandler Trace 处理器接口
type TraceHandler interface {
	HandleReportTraceEvent(ctx context.Context, req *dto.ReportTraceEventReq) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error)
	HandleListTraces(ctx context.Context, req *dto.ListTracesReq) (*dto.HTTPResponse[*dto.ListTracesRsp], error)
	HandleGetTrace(ctx context.Context, req *dto.GetTraceReq) (*dto.HTTPResponse[*dto.GetTraceRsp], error)
	HandleListTraceEvents(ctx context.Context, req *dto.ListTraceEventsReq) (*dto.HTTPResponse[*dto.ListTraceEventsRsp], error)
	HandleGetTraceConversation(ctx context.Context, req *dto.GetTraceConversationReq) (*dto.HTTPResponse[*dto.GetTraceConversationRsp], error)
	HandleCheckTraceClient(ctx context.Context, req *dto.CheckTraceClientReq) (*huma.StreamResponse, error)
	HandleInstallScript(ctx context.Context, req *dto.InstallScriptReq) (*huma.StreamResponse, error)
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
	return &traceHandler{
		report:       deps.Report,
		list:         deps.List,
		get:          deps.Get,
		events:       deps.Events,
		conversation: deps.Conversation,
	}
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

// writeInstallScriptError 返回一个将错误输出到 stderr 并退出的 bash 脚本。
// 用于 curl|bash 模式下服务端出错时给出可读提示，避免 bash 把 JSON 当命令执行。
func writeInstallScriptError(humaCtx huma.Context, message string) {
	humaCtx.SetStatus(fiber.StatusOK)
	humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeTextPlain)
	humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
	if _, writeErr := io.WriteString(humaCtx.BodyWriter(), "#!/usr/bin/env bash\necho '"+message+"' >&2\nexit 1\n"); writeErr != nil {
		logger.WithCtx(humaCtx.Context()).Warn(
			"[TraceHandler] Failed to write install error script",
			zap.Error(writeErr),
		)
	}
}

// HandleInstallScript 返回自包含的安装脚本（无票据，host 从请求头推导）。
func (h *traceHandler) HandleInstallScript(
	_ context.Context,
	_ *dto.InstallScriptReq,
) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(humaCtx huma.Context) {
		scheme := humaCtx.Header(constant.HTTPHeaderXForwardedProto)
		if scheme == "" {
			scheme = constant.HTTPSchemeHTTP
		}
		origin := scheme + "://" + humaCtx.Header(constant.HTTPHeaderHost)

		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != constant.HTTPSchemeHTTP && parsed.Scheme != constant.HTTPSchemeHTTPS) || parsed.Host == "" {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Invalid origin for install script",
				zap.String("origin", origin),
			)
			writeInstallScriptError(humaCtx, constant.TraceClientInstallOriginErrorMessage)
			return
		}

		var buf bytes.Buffer
		if err := installScriptTmpl.Execute(&buf, installScriptData{Host: origin}); err != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Failed to execute install script template",
				zap.Error(err),
			)
			writeInstallScriptError(humaCtx, constant.TraceClientInstallGenErrorMessage)
			return
		}

		humaCtx.SetStatus(fiber.StatusOK)
		humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeTextPlain)
		humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
		if _, writeErr := io.WriteString(humaCtx.BodyWriter(), buf.String()); writeErr != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Failed to write install script",
				zap.Error(writeErr),
			)
		}
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
