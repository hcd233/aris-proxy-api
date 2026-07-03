package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
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
			APIKeyName: s.Owner().String(),
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

// Paginate 按 owner 分页查询会话列表（对齐原 service.ListSessions 字段/排序）
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param owner string
//	@param param session.PageParam
//	@return []*aggregate.Session
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *sessionRepository) Paginate(ctx context.Context, owner string, param session.PageParam) ([]*aggregate.Session, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Session{APIKeyName: owner},
		constant.SessionRepoFieldsList,
		&dao.CommonParam{
			PageParam: dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			SortParam: dao.SortParam{Sort: enum.SortAsc, SortField: constant.FieldID},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}
	return lo.Map(records, func(rec *dbmodel.Session, _ int) *aggregate.Session {
		return toSessionAggregate(rec)
	}), pageInfo, nil
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
}

// NewSessionReadRepository 构造只读仓储
//
//	@return session.SessionReadRepository
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewSessionReadRepository(db *gorm.DB) session.SessionReadRepository {
	return &sessionReadRepository{
		db:         db,
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}

// sessionSummaryRow session 列表查询的扁平行模型。
//
// 设计要点（perf/session-list-trigram-and-windowcount-2026-06-08）：
//   - TotalCount 接 SQL 里的 COUNT(*) OVER ()，把分页 SELECT 与 COUNT 折成一条
//     语句执行，省掉一次独立 COUNT(*) 的 roundtrip 与 WHERE 评估。
//     窗口函数对所有行返回相同值，所以只需读 rows[0].TotalCount。
//   - MessageCount 和 ToolCount 从 message_ids / tool_ids 实时计算，不再物化冗余列。
type sessionSummaryRow struct {
	ID           uint      `gorm:"column:id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
	Score        *int      `gorm:"column:score"`
	MessageCount int       `gorm:"column:message_count"`
	ToolCount    int       `gorm:"column:tool_count"`
	Questions    []uint    `gorm:"column:questions;serializer:json"`
	Models       []string  `gorm:"column:models;serializer:json"`
	TotalCount   int64     `gorm:"column:total_count"`
}

func (r *sessionReadRepository) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := db.Model(&dbmodel.Session{}).Select(constant.SessionSummarySelect).Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}
	if param.Sort != "" && param.SortField != "" {
		param.SortField = util.SafeSortField(param.SortField)
	}
	if param.Sort != "" && param.SortField != "" {
		sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}
	if keyword != "" {
		sql = sql.Where(constant.SessionKeywordFilterSQL, "%"+keyword+"%")
	}

	// 注入 filter 条件
	if criteria != nil && len(criteria.Filters) > 0 {
		filterSQL, filterArgs, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
		if err != nil {
			return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "build filter SQL")
		}
		if filterSQL != "" {
			sql = sql.Where(filterSQL, filterArgs...)
		}
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var rows []sessionSummaryRow
	if err := sql.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if len(rows) > 0 {
		// COUNT(*) OVER () 对所有行返回相同 total，读首行即可。
		pageInfo.Total = rows[0].TotalCount
	}
	// 空结果时 total 保持 0（窗口函数无行可读）。
	// 这是合理 fallback：page > 1 且数据缩水的边界情况罕见，前端遇到 total=0 退化为
	// "no results" 视图，比为了准确性多打一次 COUNT 划得来。

	out := lo.Map(rows, func(row sessionSummaryRow, _ int) *session.SessionSummaryProjection {
		return &session.SessionSummaryProjection{
			ID:           row.ID,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Questions:    row.Questions,
			Models:       row.Models,
			Score:        row.Score,
			MessageCount: row.MessageCount,
			ToolCount:    row.ToolCount,
		}
	})
	return out, pageInfo, nil
}

func (r *sessionReadRepository) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := db.Model(&dbmodel.Session{}).Select(constant.SessionSummarySelect).Where(constant.DBConditionDeletedAtZero)
	sql = sql.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyName), ownerNames)

	if !startTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}
	if param.Sort != "" && param.SortField != "" {
		param.SortField = util.SafeSortField(param.SortField)
	}
	if param.Sort != "" && param.SortField != "" {
		sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}
	if keyword != "" {
		sql = sql.Where(constant.SessionKeywordFilterSQL, "%"+keyword+"%")
	}

	// 注入 filter 条件
	if criteria != nil && len(criteria.Filters) > 0 {
		filterSQL, filterArgs, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
		if err != nil {
			return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "build filter SQL")
		}
		if filterSQL != "" {
			sql = sql.Where(filterSQL, filterArgs...)
		}
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var rows []sessionSummaryRow
	if err := sql.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if len(rows) > 0 {
		pageInfo.Total = rows[0].TotalCount
	}

	out := lo.Map(rows, func(row sessionSummaryRow, _ int) *session.SessionSummaryProjection {
		return &session.SessionSummaryProjection{
			ID:           row.ID,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Questions:    row.Questions,
			Models:       row.Models,
			Score:        row.Score,
			MessageCount: row.MessageCount,
			ToolCount:    row.ToolCount,
		}
	})
	return out, pageInfo, nil
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
		CreatedAt:  sessionRecord.CreatedAt,
		UpdatedAt:  sessionRecord.UpdatedAt,
		Metadata:   sessionRecord.Metadata,
		Score:      sessionRecord.Score,
		ScoredAt:   sessionRecord.ScoredAt,
		MessageIDs: sessionRecord.MessageIDs,
		ToolIDs:    sessionRecord.ToolIDs,
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
			Model:     m.Model,
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
		CreatedAt:  sessionRecord.CreatedAt,
		UpdatedAt:  sessionRecord.UpdatedAt,
		Metadata:   sessionRecord.Metadata,
		Score:      sessionRecord.Score,
		ScoredAt:   sessionRecord.ScoredAt,
		MessageIDs: sessionRecord.MessageIDs,
		ToolIDs:    sessionRecord.ToolIDs,
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
			Model:     m.Model,
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

// ListDistinctScores 查询去重的评分列表
func (r *sessionReadRepository) ListDistinctScores(ctx context.Context, startTime, endTime time.Time) ([]int, error) {
	db := r.db.WithContext(ctx)

	var scores []int
	query := db.Model(&dbmodel.Session{}).
		Select(constant.SessionDistinctScoreSelect).
		Where(constant.SessionDistinctScoreWhere).
		Where(constant.DBConditionDeletedAtZero)

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

// ListDistinctModels 查询去重的模型列表
func (r *sessionReadRepository) ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	db := r.db.WithContext(ctx)

	var models []string
	query := db.Model(&dbmodel.Session{}).
		Select(constant.SessionDistinctModelSelect).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.SessionDistinctModelWhere)

	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where(constant.SessionDistinctModelLike, "%"+keyword+"%")
	}

	if err := query.Order(constant.SessionDistinctModelOrder).Limit(constant.SessionDistinctModelLimit).Scan(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct models")
	}

	return models, nil
}

// toSessionAggregate 将 GORM 模型映射为 Session 聚合根
func toSessionAggregate(m *dbmodel.Session) *aggregate.Session {
	score := vo.RestoreSessionScore(m.Score, m.ScoredAt)
	return aggregate.RestoreSession(
		m.ID,
		vo.APIKeyOwner(m.APIKeyName),
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
	Models     []string `gorm:"column:models;serializer:json"`
}

func applyExportFilter(sql *gorm.DB, f session.ExportFilter) *gorm.DB {
	sql = sql.Where(constant.DBConditionDeletedAtZero)
	if f.MinScore > 0 {
		sql = sql.Where(constant.FieldScore+" >= ?", f.MinScore)
	}
	if len(f.Models) > 0 {
		sql = sql.Where(constant.SessionExportModelFilterSQL, f.Models)
	}
	if !f.StartTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", f.StartTime)
	}
	if !f.EndTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", f.EndTime)
	}
	if len(f.OwnerNames) > 0 {
		sql = sql.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyName), f.OwnerNames)
	}
	return sql
}

// ListSessionsForExport 按筛选条件查询导出用会话行
func (r *sessionReadRepository) ListSessionsForExport(ctx context.Context, f session.ExportFilter) ([]*session.ExportSessionRow, error) {
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
			Models:     row.Models,
		}
	}), nil
}

// exportPreviewRow 预览聚合查询的扁平行模型
type exportPreviewRow struct {
	Score *int   `gorm:"column:score"`
	Model string `gorm:"column:model"`
}

func (r *sessionReadRepository) PreviewExport(ctx context.Context, f session.ExportFilter) (*session.ExportPreview, error) {
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
		if row.Model != "" {
			preview.ModelDistribution[row.Model]++
		}
	}

	return preview, nil
}
