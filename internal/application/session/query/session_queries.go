// Package query Session 域只读查询处理器
//
// 遵循 CQRS 读模型原则：通过 SessionReadRepository 接口获取只读投影，
// 并在 application 层映射为内部视图类型，保持层次隔离。
package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// SessionSummaryView Session 列表单项视图（application 层只读投影）
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type SessionSummaryView struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	MessageIDs []uint
	ToolIDs    []uint
}

// MessageView 消息视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type MessageView struct {
	ID        uint
	Model     string
	Message   *vo.UnifiedMessage
	CreatedAt time.Time
}

// ToolView 工具视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type ToolView struct {
	ID        uint
	Tool      *vo.UnifiedTool
	CreatedAt time.Time
}

// SessionDetailView Session 详情视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type SessionDetailView struct {
	ID         uint
	APIKeyName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Metadata   map[string]string
	MessageIDs []uint
	ToolIDs    []uint
	Messages   []*MessageView
	Tools      []*ToolView
}

// ListSessionsQuery 分页查询命令
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type ListSessionsQuery struct {
	// OwnerAPIKeyName 会话所有者（通常为 API Key name）
	OwnerAPIKeyName string
	Page            int
	PageSize        int
}

// ListSessionsHandler 列表查询处理器
type ListSessionsHandler interface {
	Handle(ctx context.Context, q ListSessionsQuery) ([]*SessionSummaryView, *model.PageInfo, error)
}

// GetSessionQuery 详情查询命令
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type GetSessionQuery struct {
	SessionID       uint
	OwnerAPIKeyName string
}

// GetSessionHandler 详情查询处理器
type GetSessionHandler interface {
	Handle(ctx context.Context, q GetSessionQuery) (*SessionDetailView, error)
}

type listSessionsHandler struct {
	readRepo session.SessionReadRepository
}

// NewListSessionsHandler 构造列表处理器
//
//	@param readRepo session.SessionReadRepository
//	@return ListSessionsHandler
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewListSessionsHandler(readRepo session.SessionReadRepository) ListSessionsHandler {
	return &listSessionsHandler{readRepo: readRepo}
}

// Handle 执行分页查询
//
//	@receiver h *listSessionsHandler
//	@param ctx context.Context
//	@param q ListSessionsQuery
//	@return []*SessionSummaryView
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (h *listSessionsHandler) Handle(ctx context.Context, q ListSessionsQuery) ([]*SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	projections, pageInfo, err := h.readRepo.ListSessions(ctx, q.OwnerAPIKeyName, q.Page, q.PageSize)
	if err != nil {
		log.Error("[SessionQuery] Failed to list sessions", zap.Error(err))
		return nil, nil, err
	}

	views := make([]*SessionSummaryView, 0, len(projections))
	for _, p := range projections {
		views = append(views, &SessionSummaryView{
			ID:         p.ID,
			CreatedAt:  p.CreatedAt,
			UpdatedAt:  p.UpdatedAt,
			Summary:    p.Summary,
			MessageIDs: p.MessageIDs,
			ToolIDs:    p.ToolIDs,
		})
	}
	return views, pageInfo, nil
}

type getSessionHandler struct {
	readRepo session.SessionReadRepository
}

// NewGetSessionHandler 构造详情处理器
//
//	@param readRepo session.SessionReadRepository
//	@return GetSessionHandler
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewGetSessionHandler(readRepo session.SessionReadRepository) GetSessionHandler {
	return &getSessionHandler{readRepo: readRepo}
}

// Handle 执行详情查询（含权限校验）
//
//	@receiver h *getSessionHandler
//	@param ctx context.Context
//	@param q GetSessionQuery
//	@return *SessionDetailView
//	@return error
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (h *getSessionHandler) Handle(ctx context.Context, q GetSessionQuery) (*SessionDetailView, error) {
	log := logger.WithCtx(ctx)

	detail, err := h.readRepo.GetSessionDetail(ctx, q.SessionID, q.OwnerAPIKeyName)
	if err != nil {
		log.Error("[SessionQuery] Failed to get session detail", zap.Error(err),
			zap.Uint("sessionID", q.SessionID))
		return nil, err
	}
	if detail == nil {
		log.Warn("[SessionQuery] Session not found", zap.Uint("sessionID", q.SessionID))
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
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
