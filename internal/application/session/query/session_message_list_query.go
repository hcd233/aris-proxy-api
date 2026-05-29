package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListSessionMessagesQuery 分页获取 session messages 查询参数
type ListSessionMessagesQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Offset    int
	Limit     int
}

// ListSessionMessagesResult 分页结果
type ListSessionMessagesResult struct {
	Messages []*MessageView
	Total    int64
}

// ListSessionMessagesHandler 分页获取 messages handler 接口
type ListSessionMessagesHandler interface {
	Handle(ctx context.Context, q ListSessionMessagesQuery) (*ListSessionMessagesResult, error)
}

type listSessionMessagesHandler struct {
	readRepo  session.SessionReadRepository
	metaQuery GetSessionMetaByUserHandler
	cache     sessionport.SessionDetailCache
}

// NewListSessionMessagesHandler 构造
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListSessionMessagesHandler(readRepo session.SessionReadRepository, metaQuery GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) ListSessionMessagesHandler {
	return &listSessionMessagesHandler{
		readRepo:  readRepo,
		metaQuery: metaQuery,
		cache:     detailCache,
	}
}

// Handle 复用 metaQuery 完成"权限校验+元数据获取"，再在内存中切片+缓存批读
func (h *listSessionMessagesHandler) Handle(ctx context.Context, q ListSessionMessagesQuery) (*ListSessionMessagesResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, GetSessionMetaByUserQuery{
		UserID:    q.UserID,
		IsAdmin:   q.IsAdmin,
		SessionID: q.SessionID,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.MessageIDs))
	if total == 0 {
		return &ListSessionMessagesResult{Messages: []*MessageView{}, Total: 0}, nil
	}

	offset := q.Offset
	if offset > len(meta.MessageIDs) {
		offset = len(meta.MessageIDs)
	}
	end := offset + q.Limit
	if end > len(meta.MessageIDs) {
		end = len(meta.MessageIDs)
	}
	pageIDs := meta.MessageIDs[offset:end]
	if len(pageIDs) == 0 {
		return &ListSessionMessagesResult{Messages: []*MessageView{}, Total: total}, nil
	}

	hits, missing, cacheErr := h.cache.GetMessages(ctx, pageIDs)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetMessages cache failed, fallback to DB",
			zap.Error(cacheErr), zap.Int("idsLen", len(pageIDs)))
		hits = map[uint]*sessionport.MessageCacheRecord{}
		missing = pageIDs
	}

	if len(missing) > 0 {
		records, repoErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(missing))
		if repoErr != nil {
			log.Error("[SessionQuery] FindMessagesByIDs failed", zap.Error(repoErr))
			return nil, ierr.Wrap(ierr.ErrDBQuery, repoErr, "find messages by ids")
		}

		fetched := make([]*sessionport.MessageCacheRecord, 0, len(records))
		for _, m := range records {
			rec := &sessionport.MessageCacheRecord{
				ID:        m.ID,
				Model:     m.Model,
				Message:   m.Message,
				CreatedAt: m.CreatedAt,
			}
			hits[m.ID] = rec
			fetched = append(fetched, rec)
		}
		if setErr := h.cache.SetMessages(ctx, fetched); setErr != nil {
			log.Warn("[SessionQuery] SetMessages cache failed", zap.Error(setErr))
		}
	}

	views := make([]*MessageView, 0, len(pageIDs))
	for _, id := range pageIDs {
		rec, ok := hits[id]
		if !ok {
			continue
		}
		views = append(views, &MessageView{
			ID:        rec.ID,
			Model:     rec.Model,
			Message:   rec.Message,
			CreatedAt: rec.CreatedAt,
		})
	}
	return &ListSessionMessagesResult{Messages: views, Total: total}, nil
}
