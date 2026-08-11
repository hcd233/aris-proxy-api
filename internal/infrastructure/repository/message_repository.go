package repository

import (
	"context"
	"maps"
	"time"

	"github.com/bytedance/sonic"
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
//	插入挂 ON CONFLICT DO NOTHING：并发下冲突行被跳过（ID 保持 0），随后对这部分补查补齐映射。
//	任一 checksum 最终仍无有效 ID 则返回错误，避免把零值 ID 写入 session 的 message_ids。
//
//	@receiver r *messageRepository
//	@param ctx context.Context
//	@param messages []*aggregate.Message
//	@return []uint 与 messages 顺序对齐的 ID 列表
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *messageRepository) BatchSaveDedup(ctx context.Context, messages []*aggregate.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	db := r.db.WithContext(ctx)

	checksums := lo.Map(messages, func(m *aggregate.Message, _ int) string { return m.Checksum() })

	existingMap, err := r.loadIDsByChecksum(db, checksums)
	if err != nil {
		return nil, err
	}

	newRecords := lo.UniqBy(lo.FilterMap(messages, func(m *aggregate.Message, _ int) (*dbmodel.Message, bool) {
		if _, ok := existingMap[m.Checksum()]; ok {
			return nil, false
		}
		return &dbmodel.Message{
			ModelID:  m.ModelID(),
			Message:  m.Content(),
			CheckSum: m.Checksum(),
		}, true
	}), func(m *dbmodel.Message) string { return m.CheckSum })

	if len(newRecords) > 0 {
		if err := r.dao.BatchCreate(db.Clauses(r.dao.ChecksumConflict()), newRecords); err != nil {
			return nil, ierr.Wrap(ierr.ErrDBCreate, err, "batch create messages")
		}
		if err := r.mergeCreatedIDs(db, existingMap, newRecords); err != nil {
			return nil, err
		}
	}

	ids := make([]uint, len(messages))
	for i, m := range messages {
		id := existingMap[m.Checksum()]
		if id == 0 {
			return nil, ierr.Newf(ierr.ErrDBCreate, "dedup insert left message checksum %s unresolved", m.Checksum())
		}
		ids[i] = id
		m.SetID(id)
	}
	return ids, nil
}

// loadIDsByChecksum 按 checksum 批量查询消息，返回 checksum → ID 映射
//
//	@receiver r *messageRepository
//	@param db *gorm.DB
//	@param checksums []string
//	@return map[string]uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *messageRepository) loadIDsByChecksum(db *gorm.DB, checksums []string) (map[string]uint, error) {
	rows, err := r.dao.BatchGetByField(db, constant.WhereFieldCheckSum, checksums, messageRepoFieldsChecksum)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by checksum")
	}
	return lo.SliceToMap(rows, func(m *dbmodel.Message) (string, uint) { return m.CheckSum, m.ID }), nil
}

// mergeCreatedIDs 把新插入消息的 ID 合并进映射
//
//	成功插入的行由 GORM 回填 ID；被 ON CONFLICT 跳过的行 ID 仍为 0，
//	说明并发写入抢先插入了同 checksum 记录，对这部分补查一次即可。
//
//	@receiver r *messageRepository
//	@param db *gorm.DB
//	@param idByChecksum map[string]uint
//	@param created []*dbmodel.Message
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *messageRepository) mergeCreatedIDs(db *gorm.DB, idByChecksum map[string]uint, created []*dbmodel.Message) error {
	var skipped []string
	for _, m := range created {
		if m.ID == 0 {
			skipped = append(skipped, m.CheckSum)
			continue
		}
		idByChecksum[m.CheckSum] = m.ID
	}
	if len(skipped) == 0 {
		return nil
	}

	refetched, err := r.loadIDsByChecksum(db, skipped)
	if err != nil {
		return err
	}
	maps.Copy(idByChecksum, refetched)
	return nil
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
		return aggregate.RestoreMessage(m.ID, m.Message, m.ModelID, m.CheckSum)
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
	messageJSON, err := sonic.Marshal(message)
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "marshal message content")
	}
	updates := map[string]any{
		constant.FieldMessage:   string(messageJSON),
		constant.FieldUpdatedAt: time.Now().UTC(),
	}
	if err := db.Model(&dbmodel.Message{ID: id}).Select([]string{constant.FieldMessage, constant.FieldUpdatedAt}).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update message content")
	}
	return nil
}
