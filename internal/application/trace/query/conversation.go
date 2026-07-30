package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTraceConversationHandler struct {
	repo       trace.TraceRepository
	authorizer *traceAuthorizer
}

// NewListTraceConversationHandler 构造 Trace 对话查询 handler。
func NewListTraceConversationHandler(
	repo trace.TraceRepository,
	apiKeyRepo apikeydomain.APIKeyRepository,
) port.ListTraceConversationHandler {
	return &listTraceConversationHandler{
		repo:       repo,
		authorizer: newTraceAuthorizer(repo, apiKeyRepo),
	}
}

func (h *listTraceConversationHandler) Handle(
	ctx context.Context,
	q port.ListTraceConversationQuery,
) (*port.TraceConversationView, error) {
	item, err := h.authorizer.Find(ctx, q.UserID, q.IsAdmin, q.TraceID)
	if err != nil {
		return nil, err
	}
	events, _, err := h.repo.ListEvents(ctx, item.ID, tracePageParam())
	if err != nil {
		return nil, err
	}
	conversation, err := trace.BuildConversationFor(item.Agent, events)
	if err != nil {
		return nil, err
	}
	return &port.TraceConversationView{
		TraceID:   item.ID,
		SessionID: item.SessionID,
		Turns:     mapConversationTurns(conversation),
		Tools:     mapConversationTools(conversation),
	}, nil
}

func tracePageParam() (param model.CommonParam) {
	param.Page = 1
	param.PageSize = constant.TraceConversationPageSize
	return param
}

func mapConversationTurns(conversation *trace.Conversation) []*port.TraceConversationTurnView {
	return lo.Map(conversation.Turns, func(turn *trace.ConversationTurn, _ int) *port.TraceConversationTurnView {
		items := lo.Map(turn.Items, func(item *trace.ConversationItem, _ int) *port.TraceConversationItemView {
			return &port.TraceConversationItemView{
				Kind:      item.Kind,
				Role:      item.Role,
				Content:   item.Content,
				ToolName:  item.ToolName,
				CallID:    item.CallID,
				Arguments: item.Arguments,
				Output:    item.Output,
				Source:    item.Source,
				RecordIDs: item.RecordIDs,
			}
		})
		return &port.TraceConversationTurnView{
			TurnID: turn.TurnID,
			Items:  items,
		}
	})
}

func mapConversationTools(conversation *trace.Conversation) []*port.TraceConversationToolView {
	return lo.Map(conversation.Tools, func(tool *trace.ToolDefinition, _ int) *port.TraceConversationToolView {
		return &port.TraceConversationToolView{
			Namespace:   tool.Namespace,
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
			RecordIDs:   tool.RecordIDs,
		}
	})
}
