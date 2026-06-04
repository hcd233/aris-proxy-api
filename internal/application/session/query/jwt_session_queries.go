package query

import (
	"context"
	"slices"
	"unicode/utf8"

	"github.com/samber/lo"
	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var validSessionSortFields = map[string]bool{
	constant.FieldCreatedAt:    true,
	constant.FieldUpdatedAt:    true,
	constant.FieldMessageCount: true,
	constant.FieldToolCount:    true,
}

type ListSessionsByUserHandler interface {
	Handle(ctx context.Context, q sessionport.ListSessionsByUserQuery) ([]*sessionport.SessionSummaryView, *model.PageInfo, error)
}

type GetSessionByUserHandler interface {
	Handle(ctx context.Context, q sessionport.GetSessionByUserQuery) (*sessionport.SessionDetailView, error)
}

type ownerNameLookup interface {
	LookupOwnerNamesByUserID(ctx context.Context, userID uint) ([]string, error)
}

type listSessionsByUserHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

func NewListSessionsByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) ListSessionsByUserHandler {
	return &listSessionsByUserHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *listSessionsByUserHandler) Handle(ctx context.Context, q sessionport.ListSessionsByUserQuery) ([]*sessionport.SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	param, err := sanitizeSessionListParam(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	var projections []*session.SessionSummaryProjection
	var pageInfo *model.PageInfo

	if q.IsAdmin {
		projections, pageInfo, err = h.readRepo.ListAllSessions(ctx, param, q.StartTime, q.EndTime)
	} else {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, nil, lookupErr
		}
		if len(ownerNames) == 0 {
			return []*sessionport.SessionSummaryView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
		}
		projections, pageInfo, err = h.readRepo.ListSessionsByOwnerNames(ctx, ownerNames, param, q.StartTime, q.EndTime)
	}

	if err != nil {
		log.Error("[SessionQuery] Failed to list sessions by user", zap.Error(err), zap.Uint("userID", q.UserID))
		return nil, nil, err
	}

	views := make([]*sessionport.SessionSummaryView, 0, len(projections))

	var emptySummaryIDs []uint
	for _, p := range projections {
		if p.Summary == "" {
			emptySummaryIDs = append(emptySummaryIDs, p.ID)
		}
	}

	var sessionMsgIDs map[uint][]uint
	var msgByID map[uint]*session.MessageDetailProjection
	if len(emptySummaryIDs) > 0 {
		var batchErr error
		sessionMsgIDs, batchErr = h.readRepo.FindSessionMessageIDsByIDs(ctx, emptySummaryIDs)
		if batchErr != nil {
			log.Error("[SessionQuery] Failed to batch load message IDs for empty summary", zap.Error(batchErr))
		} else {
			var allMsgIDs []uint
			for _, ids := range sessionMsgIDs {
				allMsgIDs = append(allMsgIDs, ids...)
			}
			if len(allMsgIDs) > 0 {
				messages, msgErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(allMsgIDs))
				if msgErr != nil {
					log.Error("[SessionQuery] Failed to batch load messages for empty summary", zap.Error(msgErr))
				} else {
					msgByID = lo.SliceToMap(messages, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
						return m.ID, m
					})
				}
			}
		}
	}

	for _, p := range projections {
		summary := p.Summary
		if summary == "" {
			summary = firstUserMessageContent(sessionMsgIDs[p.ID], msgByID)
		}

		views = append(views, &sessionport.SessionSummaryView{
			ID:           p.ID,
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
			Summary:      summary,
			Score:        p.Score,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
		})
	}
	return views, pageInfo, nil
}

func sanitizeSessionListParam(ctx context.Context, q sessionport.ListSessionsByUserQuery) (model.CommonParam, error) {
	page := q.Page
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > constant.SessionMaxPageSize {
		pageSize = constant.SessionMaxPageSize
	}
	if page < 1 {
		page = 1
	}
	if q.SortField != "" && !validSessionSortFields[q.SortField] {
		logger.WithCtx(ctx).Warn("[SessionQuery] Invalid sort field", zap.String("sortField", q.SortField))
		return model.CommonParam{}, ierr.New(ierr.ErrValidation, "invalid sort field: "+q.SortField)
	}
	sort := q.Sort
	sortField := q.SortField
	if sort == "" {
		sort = enum.SortDesc
	}
	if sortField == "" {
		sortField = constant.FieldCreatedAt
	}
	return model.CommonParam{
		PageParam: model.PageParam{Page: page, PageSize: pageSize},
		SortParam: model.SortParam{Sort: sort, SortField: sortField},
	}, nil
}

type getSessionByUserHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

func NewGetSessionByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) GetSessionByUserHandler {
	return &getSessionByUserHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

// firstUserMessageContent 从消息 ID 列表中提取第一个用户消息的文本内容作为 summary
func firstUserMessageContent(msgIDs []uint, msgByID map[uint]*session.MessageDetailProjection) string {
	for _, id := range msgIDs {
		m, ok := msgByID[id]
		if !ok || m.Message == nil || m.Message.Role != enum.RoleUser {
			continue
		}
		return truncateSummary(extractTextContent(m.Message.Content))
	}
	return ""
}

// extractTextContent 从 UnifiedContent 中提取纯文本内容
func extractTextContent(c *vo.UnifiedContent) string {
	if c == nil {
		return ""
	}
	if c.Text != "" {
		return c.Text
	}
	for _, p := range c.Parts {
		if p.Type == enum.ContentPartTypeText && p.Text != "" {
			return p.Text
		}
	}
	return ""
}

// truncateSummary 截断 summary 到最大 rune 数
func truncateSummary(s string) string {
	if utf8.RuneCountInString(s) <= constant.MaxSummaryRunes {
		return s
	}
	return string([]rune(s)[:constant.MaxSummaryRunes])
}

func (h *getSessionByUserHandler) Handle(ctx context.Context, q sessionport.GetSessionByUserQuery) (*sessionport.SessionDetailView, error) {
	log := logger.WithCtx(ctx)

	detail, err := h.readRepo.GetSessionDetail(ctx, q.SessionID)
	if err != nil {
		log.Error("[SessionQuery] Failed to get session detail", zap.Error(err), zap.Uint("sessionID", q.SessionID))
		return nil, err
	}
	if detail == nil {
		log.Warn("[SessionQuery] Session not found", zap.Uint("sessionID", q.SessionID))
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
	}

	if !q.IsAdmin && !q.SkipOwnershipCheck {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, lookupErr
		}
		allowed := slices.Contains(ownerNames, detail.APIKeyName)
		if !allowed {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.String("owner", detail.APIKeyName),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}

	messages := make([]*sessionport.MessageView, 0, len(detail.Messages))
	for _, m := range detail.Messages {
		messages = append(messages, &sessionport.MessageView{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		})
	}

	tools := make([]*sessionport.ToolView, 0, len(detail.Tools))
	for _, t := range detail.Tools {
		tools = append(tools, &sessionport.ToolView{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		})
	}

	return &sessionport.SessionDetailView{
		ID:         detail.ID,
		APIKeyName: detail.APIKeyName,
		CreatedAt:  detail.CreatedAt,
		UpdatedAt:  detail.UpdatedAt,
		Metadata:   detail.Metadata,
		Score:      detail.Score,
		ScoredAt:   detail.ScoredAt,
		MessageIDs: detail.MessageIDs,
		ToolIDs:    detail.ToolIDs,
		Messages:   messages,
		Tools:      tools,
	}, nil
}
