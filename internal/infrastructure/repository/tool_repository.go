package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// toolRepoFieldsChecksum 去重查询的最小字段集
var toolRepoFieldsChecksum = constant.ToolRepoFieldsChecksum

// toolRepoFieldsFull 详情查询的完整字段集
var toolRepoFieldsFull = constant.ToolRepoFieldsFull

// toolRepository ToolRepository 的 GORM 实现
//
// 去重算法与 pool.deduplicateAndStoreTools 字节级一致。
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type toolRepository struct {
	dao *dao.ToolDAO
	db  *gorm.DB
}

// NewToolRepository 构造
//
//	@return conversation.ToolRepository
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewToolRepository(db *gorm.DB) conversation.ToolRepository {
	return &toolRepository{dao: dao.GetToolDAO(), db: db}
}

// BatchSaveDedup 批量去重保存工具
//
//	@receiver r *toolRepository
//	@param ctx context.Context
//	@param tools []*aggregate.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *toolRepository) BatchSaveDedup(ctx context.Context, tools []*aggregate.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	db := r.db.WithContext(ctx)

	checksums := make([]string, len(tools))
	for i, t := range tools {
		checksums[i] = t.Checksum()
	}

	existing, err := r.dao.BatchGetByField(db, constant.WhereFieldCheckSum, checksums, toolRepoFieldsChecksum)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools by checksum")
	}

	existingMap := make(map[string]uint, len(existing))
	for _, t := range existing {
		existingMap[t.CheckSum] = t.ID
	}

	newRecords := make([]*dbmodel.Tool, 0, len(tools))
	for _, t := range tools {
		if _, ok := existingMap[t.Checksum()]; ok {
			continue
		}
		newRecords = append(newRecords, &dbmodel.Tool{
			Tool:     t.Content(),
			CheckSum: t.Checksum(),
		})
	}

	if len(newRecords) > 0 {
		if err := r.dao.BatchCreate(db, newRecords); err != nil {
			return nil, ierr.Wrap(ierr.ErrDBCreate, err, "batch create tools")
		}
		for _, nt := range newRecords {
			existingMap[nt.CheckSum] = nt.ID
		}
	}

	ids := make([]uint, len(tools))
	for i, t := range tools {
		id := existingMap[t.Checksum()]
		ids[i] = id
		t.SetID(id)
	}
	return ids, nil
}

// FindByIDs 按 ID 批量查询工具
//
//	@receiver r *toolRepository
//	@param ctx context.Context
//	@param ids []uint
//	@return []*aggregate.Tool
//	@return error
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (r *toolRepository) FindByIDs(ctx context.Context, ids []uint) ([]*aggregate.Tool, error) {
	if len(ids) == 0 {
		return []*aggregate.Tool{}, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.dao.BatchGetByField(db, constant.WhereFieldID, ids, toolRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools by id")
	}
	out := make([]*aggregate.Tool, 0, len(records))
	for _, t := range records {
		out = append(out, aggregate.RestoreTool(t.ID, t.Tool, t.CheckSum))
	}
	return out, nil
}
