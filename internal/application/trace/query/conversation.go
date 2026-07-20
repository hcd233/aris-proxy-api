package query

import (
	"context"

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
	conversation := trace.BuildConversation(events)
	return &port.TraceConversationView{
		TraceID:   item.ID,
		SessionID: item.SessionID,
		Turns:     mapConversationTurns(conversation),
	}, nil
}

func tracePageParam() (param model.CommonParam) {
	param.Page = 1
	param.PageSize = constant.TraceConversationPageSize
	return param
}

func mapConversationTurns(conversation *trace.Conversation) []*port.TraceConversationTurnView {
	turns := make([]*port.TraceConversationTurnView, 0, len(conversation.Turns))
	for _, turn := range conversation.Turns {
		items := make([]*port.TraceConversationItemView, 0, len(turn.Items))
		for _, item := range turn.Items {
			items = append(items, &port.TraceConversationItemView{
				Kind:      item.Kind,
				Role:      item.Role,
				Content:   item.Content,
				ToolName:  item.ToolName,
				CallID:    item.CallID,
				Arguments: item.Arguments,
				Output:    item.Output,
				Source:    item.Source,
				RecordIDs: item.RecordIDs,
			})
		}
		turns = append(turns, &port.TraceConversationTurnView{
			TurnID: turn.TurnID,
			Items:  items,
		})
	}
	return turns
}
