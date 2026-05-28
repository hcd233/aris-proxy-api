package query

import (
	"context"
	"unicode/utf8"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type ListSessionsByUserQuery struct {
	UserID   uint
	IsAdmin  bool
	Page     int
	PageSize int
}

type ListSessionsByUserHandler interface {
	Handle(ctx context.Context, q ListSessionsByUserQuery) ([]*SessionSummaryView, *model.PageInfo, error)
}

type GetSessionByUserQuery struct {
	UserID             uint
	IsAdmin            bool
	SkipOwnershipCheck bool
	SessionID          uint
}

type GetSessionByUserHandler interface {
	Handle(ctx context.Context, q GetSessionByUserQuery) (*SessionDetailView, error)
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

func (h *listSessionsByUserHandler) Handle(ctx context.Context, q ListSessionsByUserQuery) ([]*SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	var projections []*session.SessionSummaryProjection
	var pageInfo *model.PageInfo
	var err error

	if q.IsAdmin {
		projections, pageInfo, err = h.readRepo.ListAllSessions(ctx, q.Page, q.PageSize)
	} else {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, nil, lookupErr
		}
		if len(ownerNames) == 0 {
			return []*SessionSummaryView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
		}
		projections, pageInfo, err = h.readRepo.ListSessionsByOwnerNames(ctx, ownerNames, q.Page, q.PageSize)
	}

	if err != nil {
		log.Error("[SessionQuery] Failed to list sessions by user", zap.Error(err), zap.Uint("userID", q.UserID))
		return nil, nil, err
	}

	views := make([]*SessionSummaryView, 0, len(projections))

	var emptySummaryMsgIDs []uint
	for _, p := range projections {
		if p.Summary == "" {
			emptySummaryMsgIDs = append(emptySummaryMsgIDs, p.MessageIDs...)
		}
	}

	var msgByID map[uint]*session.MessageDetailProjection
	if len(emptySummaryMsgIDs) > 0 {
		messages, batchErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(emptySummaryMsgIDs))
		if batchErr != nil {
			log.Error("[SessionQuery] Failed to batch load messages for empty summary", zap.Error(batchErr))
		} else {
			msgByID = lo.SliceToMap(messages, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
				return m.ID, m
			})
		}
	}

	for _, p := range projections {
		summary := p.Summary
		if summary == "" {
			summary = firstUserMessageContent(p.MessageIDs, msgByID)
		}

		views = append(views, &SessionSummaryView{
			ID:        p.ID,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
			Summary:   summary,
		})
	}
	return views, pageInfo, nil
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

func (h *getSessionByUserHandler) Handle(ctx context.Context, q GetSessionByUserQuery) (*SessionDetailView, error) {
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
		allowed := false
		for _, name := range ownerNames {
			if detail.APIKeyName == name {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.String("owner", detail.APIKeyName),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}

	messages := make([]*MessageView, 0, len(detail.Messages))
	for _, m := range detail.Messages {
		messages = append(messages, &MessageView{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		})
	}

	tools := make([]*ToolView, 0, len(detail.Tools))
	for _, t := range detail.Tools {
		tools = append(tools, &ToolView{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		})
	}

	return &SessionDetailView{
		ID:         detail.ID,
		APIKeyName: detail.APIKeyName,
		CreatedAt:  detail.CreatedAt,
		UpdatedAt:  detail.UpdatedAt,
		Metadata:   detail.Metadata,
		MessageIDs: detail.MessageIDs,
		ToolIDs:    detail.ToolIDs,
		Messages:   messages,
		Tools:      tools,
	}, nil
}
