package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// GetSessionMetaByUserQuery 获取 session 元数据查询参数
type GetSessionMetaByUserQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
}

// SessionMetaView session 元数据视图（含 IDs 数组，仅在 application 层内部使用）
type SessionMetaView struct {
	ID           uint
	APIKeyName   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Metadata     map[string]string
	MessageIDs   []uint
	ToolIDs      []uint
	MessageCount int
	ToolCount    int
}

// GetSessionMetaByUserHandler 元数据查询 handler 接口
type GetSessionMetaByUserHandler interface {
	Handle(ctx context.Context, q GetSessionMetaByUserQuery) (*SessionMetaView, error)
}

type getSessionMetaByUserHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo apikey.APIKeyRepository
	cache      sessionport.SessionDetailCache
}

// NewGetSessionMetaByUserHandler 构造 handler
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewGetSessionMetaByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository, detailCache sessionport.SessionDetailCache) GetSessionMetaByUserHandler {
	return &getSessionMetaByUserHandler{
		readRepo:   readRepo,
		apiKeyRepo: apiKeyRepo,
		cache:      detailCache,
	}
}

// Handle 流程见 spec §3.3.1：
//  1. 校验 SessionID
//  2. 拿 user 的 ownerNames（admin 跳过）
//  3. 缓存命中检查
//  4. SQL 取 session 行（缓存未命中时）
//  5. 写缓存
//  6. 权限比对
func (h *getSessionMetaByUserHandler) Handle(ctx context.Context, q GetSessionMetaByUserQuery) (*SessionMetaView, error) {
	log := logger.WithCtx(ctx)

	if q.SessionID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "sessionID must be greater than 0")
	}

	var ownerNames []string
	if !q.IsAdmin {
		names, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, lookupErr
		}
		ownerNames = names
	}

	record, cacheErr := h.cache.GetSessionMeta(ctx, q.SessionID)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetSessionMeta cache failed, fallback to DB",
			zap.Uint("sessionID", q.SessionID), zap.Error(cacheErr))
		record = nil
	}

	if record == nil {
		projection, sqlErr := h.readRepo.GetSessionMeta(ctx, q.SessionID)
		if sqlErr != nil {
			log.Error("[SessionQuery] GetSessionMeta failed",
				zap.Uint("sessionID", q.SessionID), zap.Error(sqlErr))
			return nil, sqlErr
		}
		if projection == nil {
			return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
		}

		record = &sessionport.SessionMetaCacheRecord{
			ID:         projection.ID,
			APIKeyName: projection.APIKeyName,
			CreatedAt:  projection.CreatedAt,
			UpdatedAt:  projection.UpdatedAt,
			Metadata:   projection.Metadata,
			MessageIDs: projection.MessageIDs,
			ToolIDs:    projection.ToolIDs,
		}

		if setErr := h.cache.SetSessionMeta(ctx, record); setErr != nil {
			log.Warn("[SessionQuery] SetSessionMeta cache failed",
				zap.Uint("sessionID", q.SessionID), zap.Error(setErr))
		}
	}

	if !q.IsAdmin {
		allowed := false
		for _, name := range ownerNames {
			if record.APIKeyName == name {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.String("owner", record.APIKeyName),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}

	return &SessionMetaView{
		ID:           record.ID,
		APIKeyName:   record.APIKeyName,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Metadata:     record.Metadata,
		MessageIDs:   record.MessageIDs,
		ToolIDs:      record.ToolIDs,
		MessageCount: len(record.MessageIDs),
		ToolCount:    len(record.ToolIDs),
	}, nil
}
