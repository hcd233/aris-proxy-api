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

// ListSessionMessagesHandler 分页获取 messages handler 接口
type ListSessionMessagesHandler interface {
	Handle(ctx context.Context, q sessionport.ListSessionMessagesQuery) (*sessionport.ListSessionMessagesResult, error)
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
func (h *listSessionMessagesHandler) Handle(ctx context.Context, q sessionport.ListSessionMessagesQuery) (*sessionport.ListSessionMessagesResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, sessionport.GetSessionMetaByUserQuery{
		UserID:    q.UserID,
		IsAdmin:   q.IsAdmin,
		SessionID: q.SessionID,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.MessageIDs))
	if total == 0 {
		return &sessionport.ListSessionMessagesResult{Messages: []*sessionport.MessageView{}, Total: 0}, nil
	}

	start := (q.Page - 1) * q.PageSize
	start = min(start, len(meta.MessageIDs))
	end := start + q.PageSize
	end = min(end, len(meta.MessageIDs))
	pageIDs := meta.MessageIDs[start:end]
	if len(pageIDs) == 0 {
		return &sessionport.ListSessionMessagesResult{Messages: []*sessionport.MessageView{}, Total: total}, nil
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

	views := make([]*sessionport.MessageView, 0, len(pageIDs))
	for _, id := range pageIDs {
		rec, ok := hits[id]
		if !ok {
			continue
		}
		views = append(views, &sessionport.MessageView{
			ID:        rec.ID,
			Model:     rec.Model,
			Message:   rec.Message,
			CreatedAt: rec.CreatedAt,
		})
	}
	return &sessionport.ListSessionMessagesResult{Messages: views, Total: total}, nil
}
