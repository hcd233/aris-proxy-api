// Package service Session 服务
package service

import (
	"context"
	"errors"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionService Session服务
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type SessionService interface {
	ListSessions(ctx context.Context, req *dto.ListSessionsReq) (*dto.ListSessionsRsp, error)
	GetSession(ctx context.Context, req *dto.GetSessionReq) (*dto.GetSessionRsp, error)
}

type sessionService struct {
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
	toolDAO    *dao.ToolDAO
}

// NewSessionService 创建Session服务
//
//	@return SessionService
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func NewSessionService() SessionService {
	return &sessionService{
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}

// ListSessions 分页获取Session列表
//
//	@receiver s *sessionService
//	@param ctx context.Context
//	@param req *dto.ListSessionsReq
//	@return *dto.ListSessionsRsp
//	@return error
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func (s *sessionService) ListSessions(ctx context.Context, req *dto.ListSessionsReq) (*dto.ListSessionsRsp, error) {
	rsp := &dto.ListSessionsRsp{}

	apiKeyName := ctx.Value(constant.CtxKeyUserName).(string)

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	sessions, pageInfo, err := s.sessionDAO.Paginate(
		db,
		&dbmodel.Session{APIKeyName: apiKeyName},
		[]string{"id", "created_at", "updated_at", "message_ids", "tool_ids"},
		&dao.CommonParam{
			PageParam: dao.PageParam{
				Page:     req.Page,
				PageSize: req.PageSize,
			},
			SortParam: dao.SortParam{
				Sort:      enum.SortAsc,
				SortField: "id",
			},
		},
	)
	if err != nil {
		logger.Error("[SessionService] failed to paginate sessions", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	rsp.Sessions = lo.Map(sessions, func(session *dbmodel.Session, _ int) *dto.SessionSummary {
		return &dto.SessionSummary{
			ID:         session.ID,
			CreatedAt:  session.CreatedAt.Format(time.DateTime),
			UpdatedAt:  session.UpdatedAt.Format(time.DateTime),
			MessageIDs: session.MessageIDs,
			ToolIDs:    session.ToolIDs,
		}
	})
	rsp.PageInfo = pageInfo

	return rsp, nil
}

// GetSession 获取Session详情
//
//	@receiver s *sessionService
//	@param ctx context.Context
//	@param req *dto.GetSessionReq
//	@return *dto.GetSessionRsp
//	@return error
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func (s *sessionService) GetSession(ctx context.Context, req *dto.GetSessionReq) (*dto.GetSessionRsp, error) {
	rsp := &dto.GetSessionRsp{}

	apiKeyName := ctx.Value(constant.CtxKeyUserName).(string)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	session, err := s.sessionDAO.Get(
		db,
		&dbmodel.Session{ID: req.SessionID},
		[]string{"id", "api_key_name", "created_at", "updated_at", "message_ids", "tool_ids"},
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("[SessionService] session not found", zap.Uint("sessionID", req.SessionID))
			rsp.Error = constant.ErrDataNotExists
			return rsp, nil
		}
		log.Error("[SessionService] failed to get session", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	if session.APIKeyName != apiKeyName {
		log.Warn("[SessionService] no permission to access session",
			zap.Uint("sessionID", req.SessionID),
			zap.String("apiKeyName", apiKeyName),
			zap.String("sessionAPIKeyName", session.APIKeyName))
		rsp.Error = constant.ErrNoPermission
		return rsp, nil
	}

	messages, err := s.messageDAO.BatchGetByField(db, "id", lo.Uniq(session.MessageIDs), []string{"id", "model", "message", "created_at"})
	if err != nil {
		log.Error("[SessionService] failed to batch get messages", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	tools, err := s.toolDAO.BatchGetByField(db, "id", lo.Uniq(session.ToolIDs), []string{"id", "tool", "created_at"})
	if err != nil {
		log.Error("[SessionService] failed to batch get tools", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	// 构建 ID -> index 映射，按 session.MessageIDs/ToolIDs 原始顺序返回
	messageItems := BuildOrderedMessages(session.MessageIDs, messages)
	toolItems := BuildOrderedTools(session.ToolIDs, tools)

	rsp.Session = &dto.SessionDetail{
		ID:         session.ID,
		APIKeyName: session.APIKeyName,
		CreatedAt:  session.CreatedAt.Format(time.DateTime),
		UpdatedAt:  session.UpdatedAt.Format(time.DateTime),
		MessageIDs: session.MessageIDs,
		ToolIDs:    session.ToolIDs,
		Messages:   messageItems,
		Tools:      toolItems,
	}

	log.Info("[SessionService] get session detail",
		zap.Uint("sessionID", req.SessionID),
		zap.String("apiKeyName", apiKeyName),
		zap.Int("messageCount", len(messageItems)),
		zap.Int("toolCount", len(toolItems)))

	return rsp, nil
}

// BuildOrderedMessages 按指定ID顺序构建消息列表
//
//	@param ids []uint 有序ID列表
//	@param messages []*dbmodel.Message 消息列表
//	@return []*dto.MessageItem
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func BuildOrderedMessages(ids []uint, messages []*dbmodel.Message) []*dto.MessageItem {
	messageMap := lo.SliceToMap(messages, func(m *dbmodel.Message) (uint, *dbmodel.Message) {
		return m.ID, m
	})

	items := make([]*dto.MessageItem, 0, len(ids))
	for _, id := range ids {
		msg, ok := messageMap[id]
		if !ok {
			continue
		}
		items = append(items, &dto.MessageItem{
			ID:        msg.ID,
			Model:     msg.Model,
			Message:   msg.Message,
			CreatedAt: msg.CreatedAt.Format(time.DateTime),
		})
	}
	return items
}

// BuildOrderedTools 按指定ID顺序构建工具列表
//
//	@param ids []uint 有序ID列表
//	@param tools []*dbmodel.Tool 工具列表
//	@return []*dto.ToolItem
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func BuildOrderedTools(ids []uint, tools []*dbmodel.Tool) []*dto.ToolItem {
	toolMap := lo.SliceToMap(tools, func(t *dbmodel.Tool) (uint, *dbmodel.Tool) {
		return t.ID, t
	})

	items := make([]*dto.ToolItem, 0, len(ids))
	for _, id := range ids {
		tool, ok := toolMap[id]
		if !ok {
			continue
		}
		items = append(items, &dto.ToolItem{
			ID:        tool.ID,
			Tool:      tool.Tool,
			CreatedAt: tool.CreatedAt.Format(time.DateTime),
		})
	}
	return items
}
