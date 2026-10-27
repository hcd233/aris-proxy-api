package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// sessionRepository SessionRepository 的 GORM 实现
type sessionRepository struct {
	dao *dao.SessionDAO
	db  *gorm.DB
}

// NewSessionRepository 构造 SessionRepository
//
//	@return session.SessionRepository
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewSessionRepository(db *gorm.DB) session.SessionRepository {
	return &sessionRepository{dao: dao.GetSessionDAO(), db: db}
}

// Save 持久化 Session 聚合（首次 Save 回填 ID；已有 ID 执行 Update）
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param s *aggregate.Session
//	@return error
//	@author centonhuang
//	@update 2026-06-09 17:10:00
func (r *sessionRepository) Save(ctx context.Context, s *aggregate.Session) error {
	db := r.db.WithContext(ctx)

	if s.AggregateID() == 0 {
		record := &dbmodel.Session{
			APIKeyName: s.OwnerName(),
			APIKeyID:   s.OwnerID().Uint(),
			MessageIDs: s.MessageIDs(),
			ToolIDs:    s.ToolIDs(),
			Metadata:   s.Metadata(),
		}
		applyScore(record, s.Score())
		if err := r.dao.Create(db, record); err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create session")
		}
		s.SetID(record.ID)
		return nil
	}

	updates := map[string]any{
		constant.FieldMessageIDs: s.MessageIDs(),
		constant.FieldToolIDs:    s.ToolIDs(),
		constant.FieldMetadata:   s.Metadata(),
	}
	if score := s.Score(); !score.IsEmpty() {
		updates[constant.FieldScore] = *score.Score()
		if at := score.At(); at != nil {
			updates[constant.FieldScoredAt] = *at
		}
	}
	if err := r.dao.Update(db, &dbmodel.Session{ID: s.AggregateID()}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update session")
	}

	return nil
}

// applyScore 将评分值对象写入 GORM 模型
func applyScore(record *dbmodel.Session, score vo.SessionScore) {
	record.Score = score.Score()
	record.ScoredAt = score.At()
}

// FindByID 按 ID 查询 Session 聚合
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param id uint
//	@return *aggregate.Session 未找到返回 nil
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *sessionRepository) FindByID(ctx context.Context, id uint) (*aggregate.Session, error) {
	db := r.db.WithContext(ctx)
	record, err := r.dao.Get(db, &dbmodel.Session{ID: id}, constant.SessionRepoFieldsDetail)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get session by id")
	}
	return toSessionAggregate(record), nil
}

// Delete 软删除
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param id uint
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *sessionRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	if err := r.dao.Delete(db, &dbmodel.Session{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete session")
	}
	return nil
}

// UpdateScore 更新会话评分
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param id uint
//	@param score vo.SessionScore
//	@return error
//	@author centonhuang
//	@update 2026-04-26 14:00:00
func (r *sessionRepository) UpdateScore(ctx context.Context, id uint, score vo.SessionScore) error {
	db := r.db.WithContext(ctx)
	updates := map[string]any{
		constant.FieldScore:    nil,
		constant.FieldScoredAt: nil,
	}
	if !score.IsEmpty() {
		updates[constant.FieldScore] = *score.Score()
		if at := score.At(); at != nil {
			updates[constant.FieldScoredAt] = *at
		}
	}
	if err := r.dao.Update(db, &dbmodel.Session{ID: id}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update session score")
	}
	return nil
}

func (r *sessionRepository) DeleteScore(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	updates := map[string]any{
		constant.FieldScore:    nil,
		constant.FieldScoredAt: nil,
	}
	if err := r.dao.Update(db, &dbmodel.Session{ID: id}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "delete session score")
	}
	return nil
}

// ==================== CQRS 读模型实现 ====================

// sessionReadRepository SessionReadRepository 的 GORM 实现
type sessionReadRepository struct {
	db         *gorm.DB
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
	toolDAO    *dao.ToolDAO
	cache      *cache.ReadCache
}

// NewSessionReadRepository 构造只读仓储
//
//	@param db *gorm.DB
//	@param rc *cache.ReadCache keyword 命中集读缓存；nil 时退化为无缓存直查
//	@return session.SessionReadRepository
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func NewSessionReadRepository(db *gorm.DB, rc *cache.ReadCache) session.SessionReadRepository {
	return &sessionReadRepository{
		db:         db,
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
		cache:      rc,
	}
}

// sessionSummaryRow session 列表查询的扁平行模型。
//
// MessageCount / ToolCount 从 message_ids / tool_ids 实时计算（不物化冗余列）。
// total 由独立 COUNT(*) 查询产出，行模型不再携带窗口函数别名
// （perf/session-list-split-count-2026-10-23）。
type sessionSummaryRow struct {
	ID           uint      `gorm:"column:id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
	Score        *int      `gorm:"column:score"`
	MessageCount int       `gorm:"column:message_count"`
	ToolCount    int       `gorm:"column:tool_count"`
	Questions    []uint    `gorm:"column:questions;serializer:json"`
	ModelIDs     []string  `gorm:"column:model_ids;serializer:json"`
	ClientUA     string    `gorm:"column:client_ua"`
}

// FormatBigIntArray 把 id 集合序列化为 PG bigint[] 字面量（"{1,2,3}"）
//
// 纯数字拼接，无 SQL 注入面；供 SessionKeywordFilterSQL 的 = ANY(?::bigint[]) 使用。
//
// ponytail: 无命中上限保护，user 消息超数十万行时再考虑分页/降级/FTS，
// upgrade path: 分区表 + tsvector 或命中集分页传递。
func FormatBigIntArray(ids []uint) string {
	return "{" + strings.Join(lo.Map(ids, func(id uint, _ int) string {
		return strconv.FormatUint(uint64(id), 10)
	}), ",") + "}"
}

// keywordMessageIDs 两段式第一段：keyword → 用户消息 id 集合（带读缓存）
//
// 命中为空时返回 nil（GetOrLoad 的 mo.None 空值标记防穿透，短 TTL 自愈）。
func (r *sessionReadRepository) keywordMessageIDs(ctx context.Context, keyword string) ([]uint, error) {
	key := fmt.Sprintf(constant.ReadCacheKeywordIDsKeyTemplate, util.KeywordCacheKeyFragment(keyword))
	opt, err := cache.GetOrLoadWithTTL(ctx, r.cache, key,
		constant.ReadCacheKeywordIDsTTL, constant.ReadCacheKeywordIDsTTLJitter,
		func(loadCtx context.Context) (mo.Option[[]uint], error) {
			var ids []uint
			pattern := util.KeywordLikePattern(keyword)
			// 两个占位符同值：粗筛（content JSON 文本，走 trigram 索引）+ 精筛（parts 的 text 字段）
			if err := r.db.WithContext(loadCtx).Raw(constant.SessionKeywordMatchIDsSQL, pattern, pattern).Scan(&ids).Error; err != nil {
				return mo.None[[]uint](), ierr.Wrap(ierr.ErrDBQuery, err, "match keyword message ids")
			}
			if len(ids) == 0 {
				return mo.None[[]uint](), nil
			}
			return mo.Some(ids), nil
		})
	if err != nil {
		return nil, err
	}
	ids, _ := opt.Get()
	return ids, nil
}

// applyKeywordFilter 注入 keyword 过滤条件
//
// ok=false 表示关键词命中为空，调用方直接返回空页（短路，不执行第二段查询）。
//
// 安全前提：第一段命中集（keywordMessageIDs）跨租户共享——缓存 key 无 scope 维度，
// 且 SQL 不带 owner 条件扫全表 messages。数据隔离完全由第二段所在查询自身的
// owner/权限条件收敛（如 ListSessionsByOwnerIDs 的 api_key_id IN ?）。
// 新增调用方必须自带 owner 过滤，否则会把他人 session 泄漏进结果集。
func (r *sessionReadRepository) applyKeywordFilter(ctx context.Context, sql *gorm.DB, keyword string) (*gorm.DB, bool, error) {
	ids, err := r.keywordMessageIDs(ctx, keyword)
	if err != nil {
		return nil, false, err
	}
	if len(ids) == 0 {
		return nil, false, nil
	}
	return sql.Where(constant.SessionKeywordFilterSQL, FormatBigIntArray(ids)), true, nil
}

func (r *sessionReadRepository) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	base := db.Model(&dbmodel.Session{}).Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		base = base.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		base = base.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}
	if param.Sort != "" && param.SortField != "" {
		param.SortField = util.SafeSortField(param.SortField)
	}
	if keyword != "" {
		filtered, ok, err := r.applyKeywordFilter(ctx, base, keyword)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return []*session.SessionSummaryProjection{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
		}
		base = filtered
	}

	// 注入 filter 条件
	if criteria != nil && len(criteria.Filters) > 0 {
		filterSQL, filterArgs, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
		if err != nil {
			return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "build filter SQL")
		}
		if filterSQL != "" {
			base = base.Where(filterSQL, filterArgs...)
		}
	}

	return r.paginateSessions(base, param)
}

func (r *sessionReadRepository) ListSessionsByOwnerIDs(ctx context.Context, ownerIDs []uint, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	base := db.Model(&dbmodel.Session{}).Where(constant.DBConditionDeletedAtZero)
	base = base.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), ownerIDs)

	if param.Sort != "" && param.SortField != "" {
		param.SortField = util.SafeSortField(param.SortField)
	}

	base, ok, err := r.applySessionListFilters(ctx, base, startTime, endTime, keyword, criteria)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return []*session.SessionSummaryProjection{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
	}

	return r.paginateSessions(base, param)
}

// applySessionListFilters 叠加列表筛选（时间范围 / keyword / filter DSL）。
//
// 作用域条件（owner IDs 或 demo 抽样 IDs）由调用方先行注入；ok=false 表示
// keyword 命中为空集，调用方应直接返回空页，避免退化成全量查询。
func (r *sessionReadRepository) applySessionListFilters(ctx context.Context, base *gorm.DB, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) (*gorm.DB, bool, error) {
	if !startTime.IsZero() {
		base = base.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		base = base.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}
	if keyword != "" {
		filtered, ok, err := r.applyKeywordFilter(ctx, base, keyword)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		base = filtered
	}

	if criteria != nil && len(criteria.Filters) > 0 {
		filterSQL, filterArgs, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
		if err != nil {
			// 用户输入导致的筛选错误（如 harness 传了多值）必须保留 400 语义，
			// 包装成 ErrDBQuery 会让前端看到 10000 Internal Error。
			if errors.Is(err, ierr.ErrValidation) {
				return nil, false, err
			}
			return nil, false, ierr.Wrap(ierr.ErrDBQuery, err, "build filter SQL")
		}
		if filterSQL != "" {
			base = base.Where(filterSQL, filterArgs...)
		}
	}
	return base, true, nil
}

func (r *sessionReadRepository) ListSessionsByIDs(ctx context.Context, ids []uint, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	if len(ids) == 0 {
		return []*session.SessionSummaryProjection{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: 0}, nil
	}
	db := r.db.WithContext(ctx)
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	base := db.Model(&dbmodel.Session{}).Where(constant.DBConditionDeletedAtZero)
	base = base.Where(constant.FieldID+" IN ?", ids)

	if param.Sort != "" && param.SortField != "" {
		param.SortField = util.SafeSortField(param.SortField)
	}

	base, ok, err := r.applySessionListFilters(ctx, base, startTime, endTime, keyword, criteria)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return []*session.SessionSummaryProjection{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
	}

	return r.paginateSessions(base, param)
}

// paginateSessions 独立 COUNT(*) + 分页 SELECT。
//
// 必须拆成两条 SQL：COUNT(*) OVER () 会让 PG 在 LIMIT 之前对全部命中行求值
// jsonb_array_length（每行一次 TOAST 解压）再 top-N 排序——11K 会话表上实测
// 119.6ms；拆分后 COUNT 1.0ms（部分索引 index-only scan）+ 分页 4.1ms。
// 见 BENCHMARK §3.6 的 EXPLAIN 与 test/unit/session_list_split。
//
// base 必须已带全部 WHERE（含租户 scope / keyword / filter），且不得带
// Select / Order / Limit——排序与分页只作用于分页查询。
func (r *sessionReadRepository) paginateSessions(base *gorm.DB, param model.CommonParam) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count sessions")
	}

	pageQuery := base.Session(&gorm.Session{}).Select(constant.SessionSummarySelect)
	if param.Sort != "" && param.SortField != "" {
		pageQuery = pageQuery.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var rows []sessionSummaryRow
	if err := pageQuery.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}

	out := lo.Map(rows, func(row sessionSummaryRow, _ int) *session.SessionSummaryProjection {
		return &session.SessionSummaryProjection{
			ID:           row.ID,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Questions:    row.Questions,
			ModelIDs:     row.ModelIDs,
			Score:        row.Score,
			MessageCount: row.MessageCount,
			ToolCount:    row.ToolCount,
			ClientUA:     row.ClientUA,
		}
	})
	return out, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: total}, nil
}

// GetSessionDetail 查询 Session 详情（含 Message/Tool 投影）
func (r *sessionReadRepository) GetSessionDetail(ctx context.Context, id uint) (*session.SessionDetailProjection, error) {
	db := r.db.WithContext(ctx)

	sessionRecord, err := r.sessionDAO.Get(db, &dbmodel.Session{ID: id}, constant.SessionRepoFieldsReadDetail)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get session detail")
	}

	uniqMsgIDs := lo.Uniq(sessionRecord.MessageIDs)
	uniqToolIDs := lo.Uniq(sessionRecord.ToolIDs)

	messages, err := r.messageDAO.BatchGetByField(db, constant.WhereFieldID, uniqMsgIDs, constant.MessageRepoFieldsDetail)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages")
	}

	tools, err := r.toolDAO.BatchGetByField(db, constant.WhereFieldID, uniqToolIDs, constant.ToolRepoFieldsDetail)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools")
	}

	detail := &session.SessionDetailProjection{
		ID:         sessionRecord.ID,
		APIKeyName: sessionRecord.APIKeyName,
		APIKeyID:   sessionRecord.APIKeyID,
		CreatedAt:  sessionRecord.CreatedAt,
		UpdatedAt:  sessionRecord.UpdatedAt,
		Metadata:   sessionRecord.Metadata,
		Score:      sessionRecord.Score,
		ScoredAt:   sessionRecord.ScoredAt,
		MessageIDs: sessionRecord.MessageIDs,
		ToolIDs:    sessionRecord.ToolIDs,
		ClientUA:   sessionRecord.ClientUA,
		Messages:   BuildOrderedMessageProjections(sessionRecord.MessageIDs, messages),
		Tools:      BuildOrderedToolProjections(sessionRecord.ToolIDs, tools),
	}
	return detail, nil
}

// FindMessagesByIDs 批量查询消息投影
func (r *sessionReadRepository) FindMessagesByIDs(ctx context.Context, ids []uint) ([]*session.MessageDetailProjection, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.messageDAO.BatchGetByField(db, constant.WhereFieldID, ids, constant.MessageRepoFieldsDetail)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by ids")
	}
	out := lo.Map(records, func(m *dbmodel.Message, _ int) *session.MessageDetailProjection {
		return &session.MessageDetailProjection{
			ID:        m.ID,
			ModelID:   m.ModelID,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})
	return out, nil
}

func (r *sessionReadRepository) FindToolsByIDs(ctx context.Context, ids []uint) ([]*session.ToolDetailProjection, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.toolDAO.BatchGetByField(db, constant.WhereFieldID, ids, constant.ToolRepoFieldsDetail)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools by ids")
	}
	out := lo.Map(records, func(t *dbmodel.Tool, _ int) *session.ToolDetailProjection {
		return &session.ToolDetailProjection{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})
	return out, nil
}

func (r *sessionReadRepository) GetSessionMeta(ctx context.Context, id uint) (*session.SessionMetaProjection, error) {
	db := r.db.WithContext(ctx)
	sessionRecord, err := r.sessionDAO.Get(db, &dbmodel.Session{ID: id}, constant.SessionRepoFieldsReadDetail)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get session meta")
	}
	return &session.SessionMetaProjection{
		ID:         sessionRecord.ID,
		APIKeyName: sessionRecord.APIKeyName,
		APIKeyID:   sessionRecord.APIKeyID,
		CreatedAt:  sessionRecord.CreatedAt,
		UpdatedAt:  sessionRecord.UpdatedAt,
		Metadata:   sessionRecord.Metadata,
		Score:      sessionRecord.Score,
		ScoredAt:   sessionRecord.ScoredAt,
		MessageIDs: sessionRecord.MessageIDs,
		ToolIDs:    sessionRecord.ToolIDs,
		ClientUA:   sessionRecord.ClientUA,
	}, nil
}

// BuildOrderedMessageProjections 按 ids 顺序投影消息列表，跳过缺失 ID。
//
// 导出供测试断言内部排序逻辑（通过 GetSessionDetail 间接覆盖）。
func BuildOrderedMessageProjections(ids []uint, records []*dbmodel.Message) []*session.MessageDetailProjection {
	msgMap := lo.SliceToMap(records, func(m *dbmodel.Message) (uint, *dbmodel.Message) { return m.ID, m })
	items := lo.FilterMap(ids, func(id uint, _ int) (*session.MessageDetailProjection, bool) {
		m, ok := msgMap[id]
		if !ok {
			return nil, false
		}
		return &session.MessageDetailProjection{
			ID:        m.ID,
			ModelID:   m.ModelID,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}, true
	})
	return items
}

// BuildOrderedToolProjections 按 ids 顺序投影工具列表，跳过缺失 ID。
//
// 导出供测试断言内部排序逻辑（通过 GetSessionDetail 间接覆盖）。
func BuildOrderedToolProjections(ids []uint, records []*dbmodel.Tool) []*session.ToolDetailProjection {
	toolMap := lo.SliceToMap(records, func(t *dbmodel.Tool) (uint, *dbmodel.Tool) { return t.ID, t })
	items := lo.FilterMap(ids, func(id uint, _ int) (*session.ToolDetailProjection, bool) {
		t, ok := toolMap[id]
		if !ok {
			return nil, false
		}
		return &session.ToolDetailProjection{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}, true
	})
	return items
}

// ListDistinctScores 查询去重的评分列表。
//
// ownerIDs 为 nil 表示不过滤（admin/demo 白名单路径）；非 nil 且为空（用户名下
// 无 Key）短路返回空结果，防止越权查全量。sessionIDs 非 nil 时按白名单过滤。
func (r *sessionReadRepository) ListDistinctScores(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) ([]int, error) {
	key := readCacheKey(constant.ReadCachePurposeSessionOptionScore, cacheScopeIDs(ownerIDs), cacheTimeBucket(startTime), cacheTimeBucket(endTime), cacheScopeIDs(sessionIDs))
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]int, error) {
		return r.loadDistinctScores(loadCtx, ownerIDs, startTime, endTime, sessionIDs)
	})
}

func (r *sessionReadRepository) loadDistinctScores(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) ([]int, error) {
	if ownerIDs != nil && len(ownerIDs) == 0 {
		return []int{}, nil
	}
	db := r.db.WithContext(ctx)

	var scores []int
	query := db.Model(&dbmodel.Session{}).
		Select(constant.SessionDistinctScoreSelect).
		Where(constant.SessionDistinctScoreWhere).
		Where(constant.DBConditionDeletedAtZero)
	if ownerIDs != nil {
		query = query.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), ownerIDs)
	}
	if len(sessionIDs) > 0 {
		query = query.Where(constant.FieldID+" IN ?", sessionIDs)
	}
	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}

	if err := query.Order(constant.SessionDistinctScoreOrder).Scan(&scores).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct scores")
	}

	return scores, nil
}

// ListDistinctModels 查询去重的模型列表。
//
// ownerIDs 为 nil 表示不过滤（admin/demo 白名单路径）；非 nil 且为空（用户名下
// 无 Key）短路返回空结果，防止越权查全量。sessionIDs 非 nil 时按白名单过滤。
func (r *sessionReadRepository) ListDistinctModels(ctx context.Context, ownerIDs []uint, keyword string, startTime, endTime time.Time, sessionIDs []uint) ([]string, error) {
	key := readCacheKey(constant.ReadCachePurposeSessionOptionModel, cacheScopeIDs(ownerIDs), keyword, cacheTimeBucket(startTime), cacheTimeBucket(endTime), cacheScopeIDs(sessionIDs))
	return cachedSliceByKeyword(ctx, r.cache, key, keyword, func(loadCtx context.Context) ([]string, error) {
		return r.loadDistinctModels(loadCtx, ownerIDs, keyword, startTime, endTime, sessionIDs)
	})
}

func (r *sessionReadRepository) loadDistinctModels(ctx context.Context, ownerIDs []uint, keyword string, startTime, endTime time.Time, sessionIDs []uint) ([]string, error) {
	if ownerIDs != nil && len(ownerIDs) == 0 {
		return []string{}, nil
	}
	db := r.db.WithContext(ctx)

	var models []string
	query := db.Model(&dbmodel.Session{}).
		Select(constant.SessionDistinctModelSelect).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.SessionDistinctModelWhere)
	if ownerIDs != nil {
		query = query.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), ownerIDs)
	}
	if len(sessionIDs) > 0 {
		query = query.Where(constant.FieldID+" IN ?", sessionIDs)
	}
	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where(constant.SessionDistinctModelLike, util.KeywordLikePattern(keyword))
	}

	if err := query.Order(constant.SessionDistinctModelOrder).Limit(constant.SessionDistinctModelLimit).Scan(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct models")
	}

	return models, nil
}

// sessionMessageCountStats 消息数统计的缓存载荷（多返回值打包成可序列化结构）
type sessionMessageCountStats struct {
	MaxCount     int           `json:"maxCount"`
	BucketCounts map[int]int64 `json:"bucketCounts"`
}

// ListMessageCountStats 查询消息数统计：
//   - maxCount：当前时间范围内最大的 session 消息数（无会话时为 0）
//   - bucketCounts：各固定边界桶（0-10 / 11-50 / 51-100 / 101-200 / 201-500 / 501+）的会话数
//
// ownerIDs 为 nil 表示不过滤（admin/demo 白名单路径）；非 nil 且为空（用户名下
// 无 Key）短路返回空结果，防止越权查全量。sessionIDs 非 nil 时按白名单过滤。
func (r *sessionReadRepository) ListMessageCountStats(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) (maxCount int, bucketCounts map[int]int64, err error) {
	key := readCacheKey(constant.ReadCachePurposeSessionOptionMsgCnt,
		cacheScopeIDs(ownerIDs),
		cacheTimeBucket(startTime),
		cacheTimeBucket(endTime),
		cacheScopeIDs(sessionIDs),
	)
	cached, cacheErr := cache.GetOrLoadWithTTL(ctx, r.cache, key,
		constant.ReadCacheWindowTTL, constant.ReadCacheWindowTTLJitter,
		func(loadCtx context.Context) (mo.Option[sessionMessageCountStats], error) {
			loadMax, loadBuckets, loadErr := r.loadMessageCountStats(loadCtx, ownerIDs, startTime, endTime, sessionIDs)
			if loadErr != nil {
				return mo.None[sessionMessageCountStats](), loadErr
			}
			return mo.Some(sessionMessageCountStats{MaxCount: loadMax, BucketCounts: loadBuckets}), nil
		})
	if cacheErr != nil {
		return 0, nil, cacheErr
	}
	// 命中空值标记（mo.None）时返回空 map 而非 nil map：调用方把它直接塞进响应体，
	// nil map 会序列化成 "null"，与「空结果统一返回空集合」的约定冲突。
	stats, ok := cached.Get()
	if !ok {
		return 0, map[int]int64{}, nil
	}
	return stats.MaxCount, stats.BucketCounts, nil
}

func (r *sessionReadRepository) loadMessageCountStats(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) (maxCount int, bucketCounts map[int]int64, err error) {
	if ownerIDs != nil && len(ownerIDs) == 0 {
		return 0, map[int]int64{}, nil
	}
	db := r.db.WithContext(ctx)

	ownerFilter := func(q *gorm.DB) *gorm.DB {
		if ownerIDs != nil {
			q = q.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), ownerIDs)
		}
		if len(sessionIDs) > 0 {
			q = q.Where(constant.FieldID+" IN ?", sessionIDs)
		}
		return q
	}

	maxQuery := db.Model(&dbmodel.Session{}).
		Select(constant.SessionMessageCountMaxSelect).
		Where(constant.DBConditionDeletedAtZero)
	maxQuery = ownerFilter(maxQuery)
	if !startTime.IsZero() {
		maxQuery = maxQuery.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		maxQuery = maxQuery.Where(constant.WhereCreatedAtLTE, endTime)
	}

	if err := maxQuery.Scan(&maxCount).Error; err != nil {
		return 0, nil, ierr.Wrap(ierr.ErrDBQuery, err, "query max message count")
	}

	subQuery := db.Model(&dbmodel.Session{}).
		Select(constant.SessionMessageCountSQLExpr + " AS cnt").
		Where(constant.DBConditionDeletedAtZero)
	subQuery = ownerFilter(subQuery)
	if !startTime.IsZero() {
		subQuery = subQuery.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		subQuery = subQuery.Where(constant.WhereCreatedAtLTE, endTime)
	}

	type bucketRow struct {
		BucketIdx int
		Cnt       int64
	}
	var rows []bucketRow
	if err := db.Table(constant.SessionMessageCountSubqueryTable, subQuery).
		Select(constant.SessionMessageCountBucketCase + " AS bucket_idx, COUNT(*) AS cnt").
		Group(constant.SessionMessageCountBucketIdx).
		Scan(&rows).Error; err != nil {
		return 0, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count message count buckets")
	}

	bucketCounts = lo.SliceToMap(rows, func(row bucketRow) (int, int64) {
		return row.BucketIdx, row.Cnt
	})
	return maxCount, bucketCounts, nil
}

// toSessionAggregate 将 GORM 模型映射为 Session 聚合根
func toSessionAggregate(m *dbmodel.Session) *aggregate.Session {
	score := vo.RestoreSessionScore(m.Score, m.ScoredAt)
	return aggregate.RestoreSession(
		m.ID,
		vo.APIKeyOwnerID(m.APIKeyID),
		m.APIKeyName,
		m.MessageIDs,
		m.ToolIDs,
		m.Metadata,
		score,
		m.CreatedAt,
		m.UpdatedAt,
	)
}

// exportSessionRow 导出查询的扁平行模型
type exportSessionRow struct {
	ID         uint     `gorm:"column:id"`
	Score      *int     `gorm:"column:score"`
	MessageIDs []uint   `gorm:"column:message_ids;serializer:json"`
	ToolIDs    []uint   `gorm:"column:tool_ids;serializer:json"`
	ModelIDs   []string `gorm:"column:model_ids;serializer:json"`
}

func applyExportFilter(sql *gorm.DB, f session.ExportFilter) *gorm.DB {
	sql = sql.Where(constant.DBConditionDeletedAtZero)
	if f.MinScore > 0 {
		sql = sql.Where(constant.FieldScore+" >= ?", f.MinScore)
	}
	if len(f.ModelIDs) > 0 {
		sql = sql.Where(constant.SessionExportModelFilterSQL, string(lo.Must1(sonic.Marshal(f.ModelIDs))))
	}
	if !f.StartTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", f.StartTime)
	}
	if !f.EndTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", f.EndTime)
	}
	// OwnerIDs 为 nil 表示不过滤（admin 路径）；非 nil 且为空（用户名下无 Key）
	// 由两个导出入口短路返回空结果，此处不会收到非 nil 空列表
	if f.OwnerIDs != nil {
		sql = sql.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), f.OwnerIDs)
	}
	return sql
}

// ListSessionsForExport 按筛选条件查询导出用会话行
func (r *sessionReadRepository) ListSessionsForExport(ctx context.Context, f session.ExportFilter) ([]*session.ExportSessionRow, error) {
	// OwnerIDs 为 nil 表示不过滤（admin 路径）；非 nil 且为空（用户名下无 Key）
	// 短路返回空结果，防止空 owner 列表退化为全量导出（越权泄露全平台会话；
	// 与 audit 守卫的入口短路风格统一）
	if f.OwnerIDs != nil && len(f.OwnerIDs) == 0 {
		return []*session.ExportSessionRow{}, nil
	}
	db := r.db.WithContext(ctx)
	sql := db.Model(&dbmodel.Session{}).Select(constant.SessionExportSelect)
	sql = applyExportFilter(sql, f)
	sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: constant.FieldID}, Desc: false})

	var rows []exportSessionRow
	if err := sql.Find(&rows).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list sessions for export")
	}
	return lo.Map(rows, func(row exportSessionRow, _ int) *session.ExportSessionRow {
		return &session.ExportSessionRow{
			ID:         row.ID,
			Score:      row.Score,
			MessageIDs: row.MessageIDs,
			ToolIDs:    row.ToolIDs,
			ModelIDs:   row.ModelIDs,
		}
	}), nil
}

// exportPreviewRow 预览聚合查询的扁平行模型
type exportPreviewRow struct {
	Score   *int   `gorm:"column:score"`
	ModelID string `gorm:"column:model"`
}

func (r *sessionReadRepository) PreviewExport(ctx context.Context, f session.ExportFilter) (*session.ExportPreview, error) {
	// OwnerIDs 为 nil 表示不过滤（admin 路径）；非 nil 且为空（用户名下无 Key）
	// 短路返回空预览，防止空 owner 列表退化为全量统计（与导出主链路守卫一致）
	if f.OwnerIDs != nil && len(f.OwnerIDs) == 0 {
		return &session.ExportPreview{}, nil
	}
	db := r.db.WithContext(ctx)
	sql := db.Model(&dbmodel.Session{}).
		Select(constant.SessionExportPreviewSelect).
		Where(constant.SessionExportModelIsNotNull)

	sql = applyExportFilter(sql, f)

	var rows []exportPreviewRow
	if err := sql.Scan(&rows).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "preview export")
	}

	preview := &session.ExportPreview{
		ScoreDistribution: make(map[int]int),
		ModelDistribution: make(map[string]int),
	}

	for _, row := range rows {
		preview.TotalSessions++
		if row.Score != nil {
			preview.ScoreDistribution[*row.Score]++
		}
		if row.ModelID != "" {
			preview.ModelDistribution[row.ModelID]++
		}
	}

	return preview, nil
}
