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
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type traceRepository struct {
	traceDAO *dao.TraceDAO
	eventDAO *dao.EventDAO
	db       *gorm.DB
}

// NewTraceRepository 构造 TraceRepository
func NewTraceRepository(db *gorm.DB) trace.TraceRepository {
	return &traceRepository{traceDAO: dao.GetTraceDAO(), eventDAO: dao.GetEventDAO(), db: db}
}

func toTraceDomain(m *dbmodel.Trace) *trace.Trace {
	return &trace.Trace{
		ID: m.ID, Agent: m.Agent, SessionID: m.SessionID, APIKeyName: m.APIKeyName,
		ParentTraceID: m.ParentTraceID, Model: m.Model, CWD: m.CWD,
		Metadata: m.Metadata, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: m.DeletedAt,
	}
}

func toTraceRecord(t *trace.Trace) *dbmodel.Trace {
	return &dbmodel.Trace{
		Agent: t.Agent, SessionID: t.SessionID, APIKeyName: t.APIKeyName,
		ParentTraceID: t.ParentTraceID, Model: t.Model, CWD: t.CWD,
		Metadata: t.Metadata,
	}
}

func (r *traceRepository) UpsertBySessionID(ctx context.Context, t *trace.Trace) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec := toTraceRecord(t)
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: constant.FieldSessionID}},
		DoUpdates: clause.AssignmentColumns([]string{
			constant.FieldModel, constant.FieldCWD,
			constant.FieldUpdatedAt, constant.FieldMetadata, constant.FieldAPIKeyName,
			constant.FieldParentTraceID,
		}),
	}).Create(rec).Error
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBCreate, err, "upsert trace")
	}
	t.ID = rec.ID
	return t, nil
}

func (r *traceRepository) FindBySessionID(ctx context.Context, sessionID string) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec, err := r.traceDAO.Get(db, &dbmodel.Trace{SessionID: sessionID}, []string{constant.DBSelectAll})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find trace by session")
	}
	return toTraceDomain(rec), nil
}

func (r *traceRepository) FindByID(ctx context.Context, id uint) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec, err := r.traceDAO.Get(db, &dbmodel.Trace{BaseModel: dbmodel.BaseModel{ID: id}}, []string{constant.DBSelectAll})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find trace by id")
	}
	return toTraceDomain(rec), nil
}

func (r *traceRepository) FindBySessionIDIncludingDeleted(ctx context.Context, sessionID string) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	var rec dbmodel.Trace
	err := db.Unscoped().Where(&dbmodel.Trace{SessionID: sessionID}).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find trace by session including deleted")
	}
	return toTraceDomain(&rec), nil
}

func (r *traceRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	now := time.Now().UTC().Unix()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dbmodel.Trace{}).Where(constant.FieldID+" = ?", id).
			Update(constant.FieldDeletedAt, now).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "soft delete trace")
		}
		if err := tx.Model(&dbmodel.TraceEvent{}).Where(constant.FieldTraceID+" = ?", id).
			Update(constant.FieldDeletedAt, now).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "soft delete trace events")
		}
		return nil
	})
}

func (r *traceRepository) InsertEvent(ctx context.Context, e *trace.TraceEvent) (bool, error) {
	db := r.db.WithContext(ctx)
	rec := &dbmodel.TraceEvent{
		TraceID:        e.TraceID,
		SessionID:      e.SessionID,
		Source:         e.Source,
		RecordType:     e.RecordType,
		Event:          e.Event,
		TurnID:         e.TurnID,
		CallID:         e.CallID,
		TranscriptLine: e.TranscriptLine,
		ClientSequence: e.ClientSequence,
		DedupKey:       e.DedupKey,
		Payload:        e.Payload,
	}
	query := db
	if e.DedupKey != "" {
		if e.RecordType == constant.TraceRecordTypeEventMsg && e.Event == constant.TraceEventTokenCount {
			// token_count 固定 dedup key（客户端 D1a）：同 key 冲突时覆盖 payload，最终保留最后一条
			// 注意：dedup_key 唯一索引是部分索引（WHERE dedup_key <> ''），
			// PG 对 ON CONFLICT 的 arbiter 推断必须给出匹配的 index_predicate，
			// 否则每条 INSERT 抛 42P10（there is no unique or exclusion constraint matching）。
			query = query.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: constant.FieldDedupKey}},
				TargetWhere: clause.Where{Exprs: []clause.Expression{
					clause.Expr{SQL: constant.DBConditionDedupKeyNotZero},
				}},
				DoUpdates: clause.AssignmentColumns([]string{
					constant.FieldTraceID, constant.FieldPayload, constant.FieldUpdatedAt,
				}),
			})
		} else {
			query = query.Clauses(clause.OnConflict{DoNothing: true})
		}
	}
	result := query.Create(rec)
	if result.Error != nil {
		return false, ierr.Wrap(ierr.ErrDBCreate, result.Error, "insert trace event")
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	e.ID = rec.ID
	return true, nil
}

func (r *traceRepository) PaginateByOwners(ctx context.Context, owners []string, param model.CommonParam) ([]*trace.Trace, *model.PageInfo, error) {
	// owners 为 nil 表示不过滤（admin 路径）；非 nil 且为空（用户名下无 Key）
	// 短路返回空结果，防止越权查全量（与 audit 守卫的入口短路风格统一）
	if owners != nil && len(owners) == 0 {
		return []*trace.Trace{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: 0}, nil
	}
	db := r.db.WithContext(ctx)
	q := db.Model(&dbmodel.Trace{}).Where(constant.DBConditionDeletedAtZero)
	if owners != nil {
		q = q.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyName), owners)
	}
	if param.Query != "" && len(param.QueryFields) > 0 {
		like := "%" + param.Query + "%"
		expressions := lo.FilterMap(param.QueryFields, func(field string, _ int) (clause.Expression, bool) {
			if field == "" {
				return nil, false
			}
			return clause.Like{Column: clause.Column{Name: field}, Value: like}, true
		})
		if len(expressions) > 0 {
			sub := db.Session(&gorm.Session{NewDB: true}).Where(expressions[0])
			for _, expr := range expressions[1:] {
				sub = sub.Or(expr)
			}
			q = q.Where(sub)
		}
	}
	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 {
		pageInfo.PageSize = constant.TraceListPageSize
	}
	if err := q.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count traces")
	}
	var recs []*dbmodel.Trace
	if err := q.Order(clause.OrderByColumn{Column: clause.Column{Name: constant.FieldID}, Desc: true}).
		Limit(pageInfo.PageSize).Offset((pageInfo.Page - 1) * pageInfo.PageSize).Find(&recs).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list traces")
	}
	return lo.Map(recs, func(item *dbmodel.Trace, _ int) *trace.Trace { return toTraceDomain(item) }), pageInfo, nil
}

func (r *traceRepository) CountEvents(ctx context.Context, traceID uint) (int64, error) {
	db := r.db.WithContext(ctx)
	var c int64
	if err := db.Model(&dbmodel.TraceEvent{}).Where(constant.FieldTraceID+" = ?", traceID).Where(constant.DBConditionDeletedAtZero).Count(&c).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBQuery, err, "count trace events")
	}
	return c, nil
}

func (r *traceRepository) ListEvents(ctx context.Context, traceID uint, param model.CommonParam) ([]*trace.TraceEvent, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 {
		pageInfo.PageSize = constant.TraceEventPageSize
	}
	q := db.Model(&dbmodel.TraceEvent{}).Where(constant.FieldTraceID+" = ?", traceID).Where(constant.DBConditionDeletedAtZero)
	if err := q.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count trace events")
	}
	var recs []*dbmodel.TraceEvent
	if err := q.Order(clause.OrderByColumn{Column: clause.Column{Name: constant.FieldID}, Desc: false}).
		Limit(pageInfo.PageSize).Offset((pageInfo.Page - 1) * pageInfo.PageSize).Find(&recs).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list trace events")
	}
	return lo.Map(recs, func(item *dbmodel.TraceEvent, _ int) *trace.TraceEvent {
		return &trace.TraceEvent{
			ID:             item.ID,
			TraceID:        item.TraceID,
			SessionID:      item.SessionID,
			Source:         item.Source,
			RecordType:     item.RecordType,
			Event:          item.Event,
			TurnID:         item.TurnID,
			CallID:         item.CallID,
			TranscriptLine: item.TranscriptLine,
			ClientSequence: item.ClientSequence,
			DedupKey:       item.DedupKey,
			Payload:        item.Payload,
			CreatedAt:      item.CreatedAt,
		}
	}), pageInfo, nil
}
