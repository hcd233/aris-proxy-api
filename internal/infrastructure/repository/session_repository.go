package repository

import (
	"context"
	"errors"

	"github.com/samber/lo"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// sessionListFields Session 列表查询字段集（对齐原 service.ListSessions）
var sessionListFields = []string{"id", "created_at", "updated_at", "summary", "message_ids", "tool_ids"}

// sessionDetailFields Session 详情查询字段集（对齐原 service.GetSession）
var sessionDetailFields = []string{"id", "api_key_name", "created_at", "updated_at",
	"message_ids", "tool_ids", "metadata", "summary", "summarize_error",
	"coherence_score", "depth_score", "value_score", "total_score",
	"score_version", "scored_at", "score_error"}

// sessionRepository SessionRepository 的 GORM 实现
type sessionRepository struct {
	dao *dao.SessionDAO
}

// NewSessionRepository 构造 SessionRepository
//
//	@return session.SessionRepository
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewSessionRepository() session.SessionRepository {
	return &sessionRepository{dao: dao.GetSessionDAO()}
}

// Save 持久化 Session 聚合（首次 Save 回填 ID；已有 ID 执行 Update）
//
//	@receiver r *sessionRepository
//	@param ctx context.Context
//	@param s *aggregate.Session
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *sessionRepository) Save(ctx context.Context, s *aggregate.Session) error {
	db := database.GetDBInstance(ctx)

	if s.AggregateID() == 0 {
		record := &dbmodel.Session{
			APIKeyName: s.Owner().String(),
			MessageIDs: s.MessageIDs(),
			ToolIDs:    s.ToolIDs(),
			Metadata:   s.Metadata(),
		}
		applySummary(record, s.Summary())
		applyScore(record, s.Score())
		if err := r.dao.Create(db, record); err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create session")
		}
		s.SetID(record.ID)
		return nil
	}

	updates := map[string]any{
		"message_ids": s.MessageIDs(),
		"tool_ids":    s.ToolIDs(),
	}
	if summary := s.Summary(); !summary.IsEmpty() || summary.Failed() {
		updates["summary"] = summary.Text()
		updates["summarize_error"] = summary.Error()
	}
	if score := s.Score(); !score.IsEmpty() {
		updates["coherence_score"] = score.Coherence()
		updates["depth_score"] = score.Depth()
		updates["value_score"] = score.Value()
		updates["total_score"] = score.Total()
		updates["score_version"] = score.Version()
		updates["score_error"] = score.Error()
		if at := score.At(); at != nil {
			updates["scored_at"] = *at
		}
	}
	if err := r.dao.Update(db, &dbmodel.Session{ID: s.AggregateID()}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update session")
	}
	return nil
}

// applySummary 将摘要值对象写入 GORM 模型
func applySummary(record *dbmodel.Session, summary vo.SessionSummary) {
	record.Summary = summary.Text()
	record.SummarizeError = summary.Error()
}

// applyScore 将评分值对象写入 GORM 模型
func applyScore(record *dbmodel.Session, score vo.SessionScore) {
	record.CoherenceScore = score.Coherence()
	record.DepthScore = score.Depth()
	record.ValueScore = score.Value()
	record.TotalScore = score.Total()
	record.ScoreVersion = score.Version()
	record.ScoreError = score.Error()
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
	db := database.GetDBInstance(ctx)
	record, err := r.dao.Get(db, &dbmodel.Session{ID: id}, sessionDetailFields)
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
	db := database.GetDBInstance(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Session{APIKeyName: owner},
		sessionListFields,
		&dao.CommonParam{
			PageParam: dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			SortParam: dao.SortParam{Sort: enum.SortAsc, SortField: "id"},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate sessions")
	}
	out := make([]*aggregate.Session, 0, len(records))
	for _, rec := range records {
		out = append(out, toSessionAggregate(rec))
	}
	return out, pageInfo, nil
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
	db := database.GetDBInstance(ctx)
	if err := r.dao.Delete(db, &dbmodel.Session{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete session")
	}
	return nil
}

// ==================== CQRS 读模型实现 ====================

// sessionReadRepository SessionReadRepository 的 GORM 实现
type sessionReadRepository struct {
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
	toolDAO    *dao.ToolDAO
}

// NewSessionReadRepository 构造只读仓储
//
//	@return session.SessionReadRepository
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func NewSessionReadRepository() session.SessionReadRepository {
	return &sessionReadRepository{
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
	}
}

// sessionListReadFields Session 列表投影字段
var sessionListReadFields = []string{"id", "created_at", "updated_at", "summary", "message_ids", "tool_ids"}

// sessionDetailReadFields Session 详情投影字段
var sessionDetailReadFields = []string{"id", "api_key_name", "created_at", "updated_at", "message_ids", "tool_ids", "metadata"}

// ListSessions 分页查询 Session 列表投影
func (r *sessionReadRepository) ListSessions(ctx context.Context, owner string, page, pageSize int) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	db := database.GetDBInstance(ctx)
	records, pageInfo, err := r.sessionDAO.Paginate(
		db,
		&dbmodel.Session{APIKeyName: owner},
		sessionListReadFields,
		&dao.CommonParam{
			PageParam: dao.PageParam{Page: page, PageSize: pageSize},
			SortParam: dao.SortParam{Sort: enum.SortAsc, SortField: "id"},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate session read")
	}
	out := make([]*session.SessionSummaryProjection, 0, len(records))
	for _, s := range records {
		out = append(out, &session.SessionSummaryProjection{
			ID:         s.ID,
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
			Summary:    s.Summary,
			MessageIDs: s.MessageIDs,
			ToolIDs:    s.ToolIDs,
		})
	}
	return out, pageInfo, nil
}

// GetSessionDetail 查询 Session 详情（含 Message/Tool 投影）
func (r *sessionReadRepository) GetSessionDetail(ctx context.Context, id uint, owner string) (*session.SessionDetailProjection, error) {
	db := database.GetDBInstance(ctx)

	sessionRecord, err := r.sessionDAO.Get(db, &dbmodel.Session{ID: id}, sessionDetailReadFields)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get session detail")
	}

	if sessionRecord.APIKeyName != owner {
		return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
	}

	uniqMsgIDs := lo.Uniq(sessionRecord.MessageIDs)
	uniqToolIDs := lo.Uniq(sessionRecord.ToolIDs)

	messages, err := r.messageDAO.BatchGetByField(db, "id", uniqMsgIDs, []string{"id", "model", "message", "created_at"})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages")
	}

	tools, err := r.toolDAO.BatchGetByField(db, "id", uniqToolIDs, []string{"id", "tool", "created_at"})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools")
	}

	detail := &session.SessionDetailProjection{
		ID:         sessionRecord.ID,
		APIKeyName: sessionRecord.APIKeyName,
		CreatedAt:  sessionRecord.CreatedAt,
		UpdatedAt:  sessionRecord.UpdatedAt,
		Metadata:   sessionRecord.Metadata,
		MessageIDs: sessionRecord.MessageIDs,
		ToolIDs:    sessionRecord.ToolIDs,
		Messages:   BuildOrderedMessageProjections(sessionRecord.MessageIDs, messages),
		Tools:      BuildOrderedToolProjections(sessionRecord.ToolIDs, tools),
	}
	return detail, nil
}

// BuildOrderedMessageProjections 按 ids 顺序投影消息列表，跳过缺失 ID。
//
// 导出供测试断言内部排序逻辑（通过 GetSessionDetail 间接覆盖）。
func BuildOrderedMessageProjections(ids []uint, records []*dbmodel.Message) []*session.MessageDetailProjection {
	msgMap := lo.SliceToMap(records, func(m *dbmodel.Message) (uint, *dbmodel.Message) { return m.ID, m })
	items := make([]*session.MessageDetailProjection, 0, len(ids))
	for _, id := range ids {
		m, ok := msgMap[id]
		if !ok {
			continue
		}
		items = append(items, &session.MessageDetailProjection{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		})
	}
	return items
}

// BuildOrderedToolProjections 按 ids 顺序投影工具列表，跳过缺失 ID。
//
// 导出供测试断言内部排序逻辑（通过 GetSessionDetail 间接覆盖）。
func BuildOrderedToolProjections(ids []uint, records []*dbmodel.Tool) []*session.ToolDetailProjection {
	toolMap := lo.SliceToMap(records, func(t *dbmodel.Tool) (uint, *dbmodel.Tool) { return t.ID, t })
	items := make([]*session.ToolDetailProjection, 0, len(ids))
	for _, id := range ids {
		t, ok := toolMap[id]
		if !ok {
			continue
		}
		items = append(items, &session.ToolDetailProjection{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		})
	}
	return items
}

// toSessionAggregate 将 GORM 模型映射为 Session 聚合根
func toSessionAggregate(m *dbmodel.Session) *aggregate.Session {
	summary := vo.NewSessionSummary(m.Summary, m.SummarizeError)
	score := vo.RestoreSessionScore(
		m.CoherenceScore,
		m.DepthScore,
		m.ValueScore,
		m.TotalScore,
		m.ScoreVersion,
		m.ScoredAt,
		m.ScoreError,
	)
	return aggregate.RestoreSession(
		m.ID,
		vo.APIKeyOwner(m.APIKeyName),
		m.MessageIDs,
		m.ToolIDs,
		m.Metadata,
		summary,
		score,
		m.CreatedAt,
		m.UpdatedAt,
	)
}
