package repository

import (
	"context"
	"maps"

	"github.com/samber/lo"
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
//	插入挂 ON CONFLICT DO NOTHING：并发下冲突行被跳过（ID 保持 0），随后对这部分补查补齐映射。
//	任一 checksum 最终仍无有效 ID 则返回错误，避免把零值 ID 写入 session 的 tool_ids。
//
//	@receiver r *toolRepository
//	@param ctx context.Context
//	@param tools []*aggregate.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *toolRepository) BatchSaveDedup(ctx context.Context, tools []*aggregate.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	db := r.db.WithContext(ctx)

	checksums := lo.Map(tools, func(t *aggregate.Tool, _ int) string { return t.Checksum() })

	existingMap, err := r.loadIDsByChecksum(db, checksums)
	if err != nil {
		return nil, err
	}

	newRecords := lo.UniqBy(lo.FilterMap(tools, func(t *aggregate.Tool, _ int) (*dbmodel.Tool, bool) {
		if _, ok := existingMap[t.Checksum()]; ok {
			return nil, false
		}
		return &dbmodel.Tool{
			Tool:     t.Content(),
			CheckSum: t.Checksum(),
		}, true
	}), func(t *dbmodel.Tool) string { return t.CheckSum })

	if len(newRecords) > 0 {
		if err := r.dao.BatchCreate(db.Clauses(r.dao.ChecksumConflict()), newRecords); err != nil {
			return nil, ierr.Wrap(ierr.ErrDBCreate, err, "batch create tools")
		}
		if err := r.mergeCreatedIDs(db, existingMap, newRecords); err != nil {
			return nil, err
		}
	}

	ids := make([]uint, len(tools))
	for i, t := range tools {
		id := existingMap[t.Checksum()]
		if id == 0 {
			return nil, ierr.Newf(ierr.ErrDBCreate, "dedup insert left tool checksum %s unresolved", t.Checksum())
		}
		ids[i] = id
		t.SetID(id)
	}
	return ids, nil
}

// loadIDsByChecksum 按 checksum 批量查询工具，返回 checksum → ID 映射
//
//	@receiver r *toolRepository
//	@param db *gorm.DB
//	@param checksums []string
//	@return map[string]uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *toolRepository) loadIDsByChecksum(db *gorm.DB, checksums []string) (map[string]uint, error) {
	rows, err := r.dao.BatchGetByField(db, constant.WhereFieldCheckSum, checksums, toolRepoFieldsChecksum)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get tools by checksum")
	}
	return lo.SliceToMap(rows, func(t *dbmodel.Tool) (string, uint) { return t.CheckSum, t.ID }), nil
}

// mergeCreatedIDs 把新插入工具的 ID 合并进映射
//
//	成功插入的行由 GORM 回填 ID；被 ON CONFLICT 跳过的行 ID 仍为 0，
//	说明并发写入抢先插入了同 checksum 记录，对这部分补查一次即可。
//
//	@receiver r *toolRepository
//	@param db *gorm.DB
//	@param idByChecksum map[string]uint
//	@param created []*dbmodel.Tool
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (r *toolRepository) mergeCreatedIDs(db *gorm.DB, idByChecksum map[string]uint, created []*dbmodel.Tool) error {
	var skipped []string
	for _, t := range created {
		if t.ID == 0 {
			skipped = append(skipped, t.CheckSum)
			continue
		}
		idByChecksum[t.CheckSum] = t.ID
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
	return lo.Map(records, func(t *dbmodel.Tool, _ int) *aggregate.Tool {
		return aggregate.RestoreTool(t.ID, t.Tool, t.CheckSum)
	}), nil
}
