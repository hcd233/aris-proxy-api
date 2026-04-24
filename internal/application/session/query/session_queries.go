// Package query Session 域只读查询处理器
//
// 遵循 CQRS 读模型原则：直接走 DAO 投影并在 application 层映射为内部视图，
// 不向 handler/调用方泄漏 dbmodel 结构，保持层次隔离。
package query

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
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
	Message   *dto.UnifiedMessage
	CreatedAt time.Time
}

// ToolView 工具视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type ToolView struct {
	ID        uint
	Tool      *dto.UnifiedTool
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
	sessionDAO *dao.SessionDAO
}

// NewListSessionsHandler 构造列表处理器
//
//	@return ListSessionsHandler
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func NewListSessionsHandler() ListSessionsHandler {
	return &listSessionsHandler{sessionDAO: dao.GetSessionDAO()}
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
//	@update 2026-04-23 11:00:00
func (h *listSessionsHandler) Handle(ctx context.Context, q ListSessionsQuery) ([]*SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	sessions, pageInfo, err := h.sessionDAO.Paginate(
		db,
		&dbmodel.Session{APIKeyName: q.OwnerAPIKeyName},
		[]string{"id", "created_at", "updated_at", "summary", "message_ids", "tool_ids"},
		&dao.CommonParam{
			PageParam: dao.PageParam{
				Page:     q.Page,
				PageSize: q.PageSize,
			},
			SortParam: dao.SortParam{
				Sort:      enum.SortAsc,
				SortField: "id",
			},
		},
	)
	if err != nil {
		log.Error("[SessionQuery] Failed to paginate sessions", zap.Error(err))
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}

	views := make([]*SessionSummaryView, 0, len(sessions))
	for _, s := range sessions {
		views = append(views, &SessionSummaryView{
			ID:         s.ID,
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
			Summary:    s.Summary,
			MessageIDs: s.MessageIDs,
			ToolIDs:    s.ToolIDs,
		})
	}
	return views, pageInfo, nil
}

type getSessionHandler struct {
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
	toolDAO    *dao.ToolDAO
}

// NewGetSessionHandler 构造详情处理器
//
//	@return GetSessionHandler
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func NewGetSessionHandler() GetSessionHandler {
	return &getSessionHandler{
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}

// Handle 执行详情查询（含权限校验）
//
//	@receiver h *getSessionHandler
//	@param ctx context.Context
//	@param q GetSessionQuery
//	@return *SessionDetailView
//	@return error
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func (h *getSessionHandler) Handle(ctx context.Context, q GetSessionQuery) (*SessionDetailView, error) {
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	session, err := h.sessionDAO.Get(
		db,
		&dbmodel.Session{ID: q.SessionID},
		[]string{"id", "api_key_name", "created_at", "updated_at", "message_ids", "tool_ids", "client", "metadata"},
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("[SessionQuery] Session not found", zap.Uint("sessionID", q.SessionID))
			return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
		}
		log.Error("[SessionQuery] Failed to get session", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get session")
	}

	if session.APIKeyName != q.OwnerAPIKeyName {
		log.Warn("[SessionQuery] No permission to access session",
			zap.Uint("sessionID", q.SessionID),
			zap.String("apiKeyName", q.OwnerAPIKeyName),
			zap.String("sessionAPIKeyName", session.APIKeyName))
		return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
	}

	messages, err := h.messageDAO.BatchGetByField(db, "id", uniqUints(session.MessageIDs), []string{"id", "model", "message", "created_at"})
	if err != nil {
		log.Error("[SessionQuery] Failed to batch get messages", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages")
	}

	tools, err := h.toolDAO.BatchGetByField(db, "id", uniqUints(session.ToolIDs), []string{"id", "tool", "created_at"})
	if err != nil {
		log.Error("[SessionQuery] Failed to batch get tools", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools")
	}

	return &SessionDetailView{
		ID:         session.ID,
		APIKeyName: session.APIKeyName,
		CreatedAt:  session.CreatedAt,
		UpdatedAt:  session.UpdatedAt,
		Metadata:   session.Metadata,
		MessageIDs: session.MessageIDs,
		ToolIDs:    session.ToolIDs,
		Messages:   BuildOrderedMessages(session.MessageIDs, messages),
		Tools:      BuildOrderedTools(session.ToolIDs, tools),
	}, nil
}

// uniqUints 去重整型切片，保持首次出现顺序
//
//	@param s []uint
//	@return []uint
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func uniqUints(s []uint) []uint {
	seen := make(map[uint]struct{}, len(s))
	out := make([]uint, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
