package query

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-mass/internal/application/session/port"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/domain/session"
	"github.com/hcd233/aris-mass/internal/logger"
)

type listSessionToolsHandler struct {
	readRepo  session.SessionReadRepository
	metaQuery sessionport.GetSessionMetaByUserHandler
	cache     sessionport.SessionDetailCache
}

// NewListSessionToolsHandler 构造
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListSessionToolsHandler(readRepo session.SessionReadRepository, metaQuery sessionport.GetSessionMetaByUserHandler, detailCache sessionport.SessionDetailCache) sessionport.ListSessionToolsHandler {
	return &listSessionToolsHandler{
		readRepo:  readRepo,
		metaQuery: metaQuery,
		cache:     detailCache,
	}
}

// Handle 处理 tools 分页查询
func (h *listSessionToolsHandler) Handle(ctx context.Context, q sessionport.ListSessionToolsQuery) (*sessionport.ListSessionToolsResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, sessionport.GetSessionMetaByUserQuery{
		UserID:            q.UserID,
		IsAdmin:           q.IsAdmin,
		SessionID:         q.SessionID,
		IsDemo:            q.IsDemo,
		AllowedSessionIDs: q.AllowedSessionIDs,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.ToolIDs))
	if total == 0 {
		return &sessionport.ListSessionToolsResult{Tools: []*sessionport.ToolView{}, Total: 0}, nil
	}

	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		return h.searchByName(ctx, meta.ToolIDs, keyword, q.Page, q.PageSize)
	}

	start := (q.Page - 1) * q.PageSize
	start = min(start, len(meta.ToolIDs))
	end := start + q.PageSize
	end = min(end, len(meta.ToolIDs))
	pageIDs := meta.ToolIDs[start:end]
	if len(pageIDs) == 0 {
		return &sessionport.ListSessionToolsResult{Tools: []*sessionport.ToolView{}, Total: total}, nil
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

		fetched := lo.Map(records, func(t *session.ToolDetailProjection, _ int) *sessionport.ToolCacheRecord {
			return &sessionport.ToolCacheRecord{
				ID:        t.ID,
				Tool:      t.Tool,
				CreatedAt: t.CreatedAt,
			}
		})
		hits = lo.Assign(hits, lo.SliceToMap(fetched, func(rec *sessionport.ToolCacheRecord) (uint, *sessionport.ToolCacheRecord) {
			return rec.ID, rec
		}))
		if setErr := h.cache.SetTools(ctx, fetched); setErr != nil {
			log.Warn("[SessionQuery] SetTools cache failed", zap.Error(setErr))
		}
	}

	views := lo.FilterMap(pageIDs, func(id uint, _ int) (*sessionport.ToolView, bool) {
		rec, ok := hits[id]
		if !ok {
			return nil, false
		}
		return &sessionport.ToolView{
			ID:        rec.ID,
			Tool:      rec.Tool,
			CreatedAt: rec.CreatedAt,
		}, true
	})
	return &sessionport.ListSessionToolsResult{Tools: views, Total: total}, nil
}

// searchByName 按工具名搜索：过滤与分页都在数据库完成，不经工具缓存
func (h *listSessionToolsHandler) searchByName(ctx context.Context, toolIDs []uint, keyword string, page, pageSize int) (*sessionport.ListSessionToolsResult, error) {
	records, total, err := h.readRepo.SearchToolsByName(ctx, toolIDs, keyword, page, pageSize)
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionQuery] SearchToolsByName failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "search tools by name")
	}
	views := lo.Map(records, func(t *session.ToolDetailProjection, _ int) *sessionport.ToolView {
		return &sessionport.ToolView{ID: t.ID, Tool: t.Tool, CreatedAt: t.CreatedAt}
	})
	return &sessionport.ListSessionToolsResult{Tools: views, Total: total}, nil
}
