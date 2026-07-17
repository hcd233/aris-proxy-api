package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
		UserID: m.UserID, Model: m.Model, CWD: m.CWD, Source: m.Source,
		Status: m.Status, Metadata: m.Metadata, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func toTraceRecord(t *trace.Trace) *dbmodel.Trace {
	return &dbmodel.Trace{
		Agent: t.Agent, SessionID: t.SessionID, APIKeyName: t.APIKeyName,
		UserID: t.UserID, Model: t.Model, CWD: t.CWD, Source: t.Source,
		Status: t.Status, Metadata: t.Metadata,
	}
}

func (r *traceRepository) UpsertBySessionID(ctx context.Context, t *trace.Trace) (*trace.Trace, error) {
	db := r.db.WithContext(ctx)
	rec := toTraceRecord(t)
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: constant.FieldSessionID}},
		DoUpdates: clause.AssignmentColumns([]string{
			constant.FieldModel, constant.FieldCWD, constant.FieldSource, constant.FieldStatus,
			constant.FieldUpdatedAt, constant.FieldMetadata, constant.FieldUserID, constant.FieldAPIKeyName,
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

func (r *traceRepository) MarkDone(ctx context.Context, sessionID string) error {
	db := r.db.WithContext(ctx)
	err := db.Model(&dbmodel.Trace{}).Where(constant.FieldSessionID+" = ?", sessionID).
		Updates(map[string]any{constant.FieldStatus: constant.TraceStatusDone, constant.FieldUpdatedAt: time.Now().UTC()}).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "mark trace done")
	}
	return nil
}

func (r *traceRepository) InsertEvent(ctx context.Context, e *trace.TraceEvent) error {
	db := r.db.WithContext(ctx)
	if e.DedupKey == "" {
		digest := sha256.Sum256(e.Payload)
		e.DedupKey = constant.TraceLegacyDedupPrefix + e.SessionID + ":" + e.Event + ":" + hex.EncodeToString(digest[:])
	}
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
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: constant.TraceFieldDedupKey}},
		DoNothing: true,
	}).Create(rec).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "insert trace event")
	}
	e.ID = rec.ID
	return nil
}

func (r *traceRepository) PaginateByOwners(ctx context.Context, owners []string, param model.CommonParam) ([]*trace.Trace, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	q := db.Model(&dbmodel.Trace{}).Where(constant.DBConditionDeletedAtZero)
	if len(owners) > 0 {
		q = q.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyName), owners)
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
