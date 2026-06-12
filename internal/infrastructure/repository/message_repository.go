package repository

import (
	"context"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// messageRepoFieldsChecksum 去重查询的最小字段集
var messageRepoFieldsChecksum = constant.MessageRepoFieldsChecksum

// messageRepoFieldsFull 详情查询的完整字段集（与原 SessionService 一致）
var messageRepoFieldsFull = constant.MessageRepoFieldsFull

// messageRepository MessageRepository 的 GORM 实现
//
// 去重算法与 pool.deduplicateAndStoreMessages 字节级一致：
//
//  1. 按 Checksum 批量 IN 查询已存在条目
//
//  2. 过滤掉已存在的，BatchCreate 剩余新消息
//
//  3. 按输入顺序返回 ID 列表（含复用 ID）
//
//     @author centonhuang
//     @update 2026-04-22 19:30:00
type messageRepository struct {
	dao *dao.MessageDAO
	db  *gorm.DB
}

// NewMessageRepository 构造
//
//	@return conversation.MessageRepository
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewMessageRepository(db *gorm.DB) conversation.MessageRepository {
	return &messageRepository{dao: dao.GetMessageDAO(), db: db}
}

func NewThinkExtractRepository(db *gorm.DB) conversation.ThinkExtractRepository {
	return &messageRepository{dao: dao.GetMessageDAO(), db: db}
}

// BatchSaveDedup 批量去重保存消息
//
//	@receiver r *messageRepository
//	@param ctx context.Context
//	@param messages []*aggregate.Message
//	@return []uint 与 messages 顺序对齐的 ID 列表
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *messageRepository) BatchSaveDedup(ctx context.Context, messages []*aggregate.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	db := r.db.WithContext(ctx)

	checksums := lo.Map(messages, func(m *aggregate.Message, _ int) string { return m.Checksum() })

	existing, err := r.dao.BatchGetByField(db, constant.WhereFieldCheckSum, checksums, messageRepoFieldsChecksum)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by checksum")
	}

	existingMap := lo.SliceToMap(existing, func(m *dbmodel.Message) (string, uint) { return m.CheckSum, m.ID })

	newRecords := lo.FilterMap(messages, func(m *aggregate.Message, _ int) (*dbmodel.Message, bool) {
		if _, ok := existingMap[m.Checksum()]; ok {
			return nil, false
		}
		return &dbmodel.Message{
			Model:    m.Model(),
			Message:  m.Content(),
			CheckSum: m.Checksum(),
		}, true
	})

	if len(newRecords) > 0 {
		if err := r.dao.BatchCreate(db, newRecords); err != nil {
			return nil, ierr.Wrap(ierr.ErrDBCreate, err, "batch create messages")
		}
		for _, nm := range newRecords {
			existingMap[nm.CheckSum] = nm.ID
		}
	}

	ids := lo.Map(messages, func(m *aggregate.Message, _ int) uint { return existingMap[m.Checksum()] })
	lo.ForEach(messages, func(m *aggregate.Message, _ int) { m.SetID(existingMap[m.Checksum()]) })
	return ids, nil
}

// FindByIDs 按 ID 批量查询消息
//
//	@receiver r *messageRepository
//	@param ctx context.Context
//	@param ids []uint
//	@return []*aggregate.Message
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *messageRepository) FindByIDs(ctx context.Context, ids []uint) ([]*aggregate.Message, error) {
	if len(ids) == 0 {
		return []*aggregate.Message{}, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.dao.BatchGetByField(db, constant.WhereFieldID, ids, messageRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by id")
	}
	out := lo.Map(records, func(m *dbmodel.Message, _ int) *aggregate.Message {
		return aggregate.RestoreMessage(m.ID, m.Message, m.Model, m.CheckSum)
	})
	return out, nil
}

func (r *messageRepository) FindThinkExtractCandidates(ctx context.Context, afterID uint, startTime, endTime time.Time, limit int) ([]*conversation.ThinkExtractMessage, error) {
	if limit < 1 {
		limit = 1
	}
	db := r.db.WithContext(ctx)
	var records []*dbmodel.Message
	query := db.Model(&dbmodel.Message{}).
		Select([]string{constant.FieldID, constant.FieldMessage}).
		Where(constant.DBConditionIDGreaterThan, afterID).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.DBJSONConditionAssistantRole).
		Where(constant.DBJSONConditionHasThinkTag).
		Where(constant.DBJSONConditionReasoningEmpty)
	if !startTime.IsZero() {
		query = query.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.FieldCreatedAt+" < ?", endTime)
	}
	if err := query.Order(constant.DBOrderByID).Limit(limit).Find(&records).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query think extract messages")
	}
	out := lo.Map(records, func(record *dbmodel.Message, _ int) *conversation.ThinkExtractMessage {
		return &conversation.ThinkExtractMessage{ID: record.ID, Message: record.Message}
	})
	return out, nil
}

func (r *messageRepository) UpdateMessageContent(ctx context.Context, id uint, message *vo.UnifiedMessage) error {
	db := r.db.WithContext(ctx)
	updates := map[string]any{
		constant.FieldMessage:   message,
		constant.FieldUpdatedAt: time.Now().UTC(),
	}
	if err := db.Model(&dbmodel.Message{ID: id}).Select([]string{constant.FieldMessage, constant.FieldUpdatedAt}).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update message content")
	}
	return nil
}
