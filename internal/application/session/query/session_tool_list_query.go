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

// ListSessionToolsQuery 分页获取 session tools 查询参数
type ListSessionToolsQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Page      int
	PageSize  int
}

// ListSessionToolsResult 分页结果
type ListSessionToolsResult struct {
	Tools []*ToolView
	Total int64
}

// ListSessionToolsHandler 分页获取 tools handler 接口
type ListSessionToolsHandler interface {
	Handle(ctx context.Context, q ListSessionToolsQuery) (*ListSessionToolsResult, error)
}

type listSessionToolsHandler struct {
	readRepo  session.SessionReadRepository
	metaQuery GetSessionMetaByUserHandler
	cache     sessionport.SessionDetailCache
}

// NewListSessionToolsHandler 构造
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListSessionToolsHandler(readRepo session.SessionReadRepository, metaQuery GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) ListSessionToolsHandler {
	return &listSessionToolsHandler{
		readRepo:  readRepo,
		metaQuery: metaQuery,
		cache:     detailCache,
	}
}

// Handle 处理 tools 分页查询
func (h *listSessionToolsHandler) Handle(ctx context.Context, q ListSessionToolsQuery) (*ListSessionToolsResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, GetSessionMetaByUserQuery{
		UserID:    q.UserID,
		IsAdmin:   q.IsAdmin,
		SessionID: q.SessionID,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.ToolIDs))
	if total == 0 {
		return &ListSessionToolsResult{Tools: []*ToolView{}, Total: 0}, nil
	}

	start := (q.Page - 1) * q.PageSize
	if start > len(meta.ToolIDs) {
		start = len(meta.ToolIDs)
	}
	end := start + q.PageSize
	if end > len(meta.ToolIDs) {
		end = len(meta.ToolIDs)
	}
	pageIDs := meta.ToolIDs[start:end]
	if len(pageIDs) == 0 {
		return &ListSessionToolsResult{Tools: []*ToolView{}, Total: total}, nil
	}

	hits, missing, cacheErr := h.cache.GetTools(ctx, pageIDs)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetTools cache failed, fallback to DB",
			zap.Error(cacheErr), zap.Int("idsLen", len(pageIDs)))
		hits = map[uint]*sessionport.ToolCacheRecord{}
		missing = pageIDs
	}

	if len(missing) > 0 {
		records, repoErr := h.readRepo.FindToolsByIDs(ctx, lo.Uniq(missing))
		if repoErr != nil {
			log.Error("[SessionQuery] FindToolsByIDs failed", zap.Error(repoErr))
			return nil, ierr.Wrap(ierr.ErrDBQuery, repoErr, "find tools by ids")
		}

		fetched := make([]*sessionport.ToolCacheRecord, 0, len(records))
		for _, t := range records {
			rec := &sessionport.ToolCacheRecord{
				ID:        t.ID,
				Tool:      t.Tool,
				CreatedAt: t.CreatedAt,
			}
			hits[t.ID] = rec
			fetched = append(fetched, rec)
		}
		if setErr := h.cache.SetTools(ctx, fetched); setErr != nil {
			log.Warn("[SessionQuery] SetTools cache failed", zap.Error(setErr))
		}
	}

	views := make([]*ToolView, 0, len(pageIDs))
	for _, id := range pageIDs {
		rec, ok := hits[id]
		if !ok {
			continue
		}
		views = append(views, &ToolView{
			ID:        rec.ID,
			Tool:      rec.Tool,
			CreatedAt: rec.CreatedAt,
		})
	}
	return &ListSessionToolsResult{Tools: views, Total: total}, nil
}
