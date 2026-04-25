package repository

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// messageRepoFieldsChecksum 去重查询的最小字段集
var messageRepoFieldsChecksum = []string{"id", "check_sum"}

// messageRepoFieldsFull 详情查询的完整字段集（与原 SessionService 一致）
var messageRepoFieldsFull = []string{"id", "model", "message", "check_sum", "created_at"}

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
}

// NewMessageRepository 构造
//
//	@return conversation.MessageRepository
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewMessageRepository() conversation.MessageRepository {
	return &messageRepository{dao: dao.GetMessageDAO()}
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

	db := database.GetDBInstance(ctx)

	checksums := make([]string, len(messages))
	for i, m := range messages {
		checksums[i] = m.Checksum()
	}

	existing, err := r.dao.BatchGetByField(db, "check_sum", checksums, messageRepoFieldsChecksum)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by checksum")
	}

	existingMap := make(map[string]uint, len(existing))
	for _, m := range existing {
		existingMap[m.CheckSum] = m.ID
	}

	newRecords := make([]*dbmodel.Message, 0, len(messages))
	for _, m := range messages {
		if _, ok := existingMap[m.Checksum()]; ok {
			continue
		}
		newRecords = append(newRecords, &dbmodel.Message{
			Model:    m.Model(),
			Message:  m.Content(),
			CheckSum: m.Checksum(),
		})
	}

	if len(newRecords) > 0 {
		if err := r.dao.BatchCreate(db, newRecords); err != nil {
			return nil, ierr.Wrap(ierr.ErrDBCreate, err, "batch create messages")
		}
		for _, nm := range newRecords {
			existingMap[nm.CheckSum] = nm.ID
		}
	}

	ids := make([]uint, len(messages))
	for i, m := range messages {
		id := existingMap[m.Checksum()]
		ids[i] = id
		// 回填聚合 ID（便于后续事件发布或引用）
		m.SetID(id)
	}
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
	db := database.GetDBInstance(ctx)
	records, err := r.dao.BatchGetByField(db, "id", ids, messageRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get messages by id")
	}
	out := make([]*aggregate.Message, 0, len(records))
	for _, m := range records {
		out = append(out, aggregate.RestoreMessage(m.ID, m.Message, m.Model, m.CheckSum))
	}
	return out, nil
}
