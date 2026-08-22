// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"context"
	"maps"

	"github.com/alitto/pond/v2"
	demoaccessauditport "github.com/hcd233/aris-proxy-api/internal/application/demoaccessaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// PoolManager 全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolManager struct {
	db                  *gorm.DB
	auditRepo           modelcall.AuditRepository
	demoAccessAuditRepo demoaccessauditport.DemoAccessAuditRepository
	storePool           pond.Pool
	agentPool           pond.Pool
}

// NewPoolManager 创建协程池管理器。
//
//	@param db *gorm.DB
//	@param auditRepo modelcall.AuditRepository 审计写聚合仓储（审计落库统一经此聚合）
//	@param demoAccessAuditRepo demoaccessauditport.DemoAccessAuditRepository Demo 访问审计仓储
//	@return *PoolManager
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewPoolManager(db *gorm.DB, auditRepo modelcall.AuditRepository, demoAccessAuditRepo demoaccessauditport.DemoAccessAuditRepository) *PoolManager {
	return &PoolManager{
		db:                  db,
		auditRepo:           auditRepo,
		demoAccessAuditRepo: demoAccessAuditRepo,
		storePool:           pond.NewPool(config.Pool.Store.Workers, pond.WithQueueSize(config.Pool.Store.QueueSize)),
		agentPool:           pond.NewPool(config.Pool.Agent.Workers, pond.WithQueueSize(config.Pool.Agent.QueueSize)),
	}
}

func (pm *PoolManager) StopWithContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		pm.Stop()
	}()
	select {
	case <-done:
		logger.Logger().Info("[Pool] Pool manager stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// loadMessageIDsByChecksum 按 checksum 批量查询消息，返回 checksum → ID 映射
//
//	@param tx *gorm.DB
//	@param messageDAO *dao.MessageDAO
//	@param checksums []string
//	@return map[string]uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func loadMessageIDsByChecksum(tx *gorm.DB, messageDAO *dao.MessageDAO, checksums []string) (map[string]uint, error) {
	rows, err := messageDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.MessageRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(rows, func(m *dbmodel.Message) (string, uint) { return m.CheckSum, m.ID }), nil
}

// loadToolIDsByChecksum 按 checksum 批量查询工具，返回 checksum → ID 映射
//
//	@param tx *gorm.DB
//	@param toolDAO *dao.ToolDAO
//	@param checksums []string
//	@return map[string]uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func loadToolIDsByChecksum(tx *gorm.DB, toolDAO *dao.ToolDAO, checksums []string) (map[string]uint, error) {
	rows, err := toolDAO.BatchGetByField(tx, constant.WhereFieldCheckSum, checksums, constant.ToolRepoFieldsChecksum)
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(rows, func(t *dbmodel.Tool) (string, uint) { return t.CheckSum, t.ID }), nil
}

// checksumIDsInOrder 按输入顺序取出 ID
//
//	任一 checksum 未能映射到有效 ID 即返回错误，使事务回滚。
//	绝不能返回零值 ID——那会让 session 的 message_ids / tool_ids 写入 0 这类悬空引用。
//
//	@param checksums []string
//	@param idByChecksum map[string]uint
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func checksumIDsInOrder(checksums []string, idByChecksum map[string]uint) ([]uint, error) {
	ids := make([]uint, len(checksums))
	for i, checksum := range checksums {
		id := idByChecksum[checksum]
		if id == 0 {
			return nil, ierr.Newf(ierr.ErrDBCreate, "dedup insert left checksum %s unresolved", checksum)
		}
		ids[i] = id
	}
	return ids, nil
}

// deduplicateAndStoreMessages 批量去重并存储消息
//
//	使用 IN 查询一次性获取已存在的消息，批量创建不存在的消息，保持原始顺序返回 ID 列表。
//	插入挂 ON CONFLICT DO NOTHING：并发下冲突行被跳过（ID 保持 0），随后对这部分补查补齐映射。
//
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param messages []*dbmodel.Message
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (pm *PoolManager) deduplicateAndStoreMessages(tx *gorm.DB, messages []*dbmodel.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	messageDAO := dao.GetMessageDAO()
	checksums := lo.Map(messages, func(m *dbmodel.Message, _ int) string { return m.CheckSum })

	existingMap, err := loadMessageIDsByChecksum(tx, messageDAO, checksums)
	if err != nil {
		return nil, err
	}

	newMessages := lo.UniqBy(lo.Filter(messages, func(m *dbmodel.Message, _ int) bool {
		_, exists := existingMap[m.CheckSum]
		return !exists
	}), func(m *dbmodel.Message) string { return m.CheckSum })

	if len(newMessages) > 0 {
		if err := messageDAO.BatchCreate(tx.Clauses(messageDAO.ChecksumConflict()), newMessages); err != nil {
			return nil, err
		}
		if err := mergeCreatedMessageIDs(tx, messageDAO, existingMap, newMessages); err != nil {
			return nil, err
		}
	}

	return checksumIDsInOrder(checksums, existingMap)
}

// mergeCreatedMessageIDs 把新插入消息的 ID 合并进映射
//
//	成功插入的行由 GORM 回填 ID；被 ON CONFLICT 跳过的行 ID 仍为 0，
//	说明并发写入抢先插入了同 checksum 记录，对这部分补查一次即可。
//
//	@param tx *gorm.DB
//	@param messageDAO *dao.MessageDAO
//	@param idByChecksum map[string]uint
//	@param created []*dbmodel.Message
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func mergeCreatedMessageIDs(tx *gorm.DB, messageDAO *dao.MessageDAO, idByChecksum map[string]uint, created []*dbmodel.Message) error {
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

	refetched, err := loadMessageIDsByChecksum(tx, messageDAO, skipped)
	if err != nil {
		return err
	}
	maps.Copy(idByChecksum, refetched)
	return nil
}

// deduplicateAndStoreTools 批量去重并存储工具
//
//	使用 IN 查询一次性获取已存在的工具，批量创建不存在的工具，保持原始顺序返回 ID 列表。
//	插入挂 ON CONFLICT DO NOTHING：并发下冲突行被跳过（ID 保持 0），随后对这部分补查补齐映射。
//
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param tools []*dbmodel.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (pm *PoolManager) deduplicateAndStoreTools(tx *gorm.DB, tools []*dbmodel.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	toolDAO := dao.GetToolDAO()
	checksums := lo.Map(tools, func(t *dbmodel.Tool, _ int) string { return t.CheckSum })

	existingMap, err := loadToolIDsByChecksum(tx, toolDAO, checksums)
	if err != nil {
		return nil, err
	}

	newTools := lo.UniqBy(lo.Filter(tools, func(t *dbmodel.Tool, _ int) bool {
		_, exists := existingMap[t.CheckSum]
		return !exists
	}), func(t *dbmodel.Tool) string { return t.CheckSum })

	if len(newTools) > 0 {
		if err := toolDAO.BatchCreate(tx.Clauses(toolDAO.ChecksumConflict()), newTools); err != nil {
			return nil, err
		}
		if err := mergeCreatedToolIDs(tx, toolDAO, existingMap, newTools); err != nil {
			return nil, err
		}
	}

	return checksumIDsInOrder(checksums, existingMap)
}

// mergeCreatedToolIDs 把新插入工具的 ID 合并进映射
//
//	语义与 mergeCreatedMessageIDs 一致，见其说明。
//
//	@param tx *gorm.DB
//	@param toolDAO *dao.ToolDAO
//	@param idByChecksum map[string]uint
//	@param created []*dbmodel.Tool
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func mergeCreatedToolIDs(tx *gorm.DB, toolDAO *dao.ToolDAO, idByChecksum map[string]uint, created []*dbmodel.Tool) error {
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

	refetched, err := loadToolIDsByChecksum(tx, toolDAO, skipped)
	if err != nil {
		return err
	}
	maps.Copy(idByChecksum, refetched)
	return nil
}

// Stop 停止所有协程池
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) Stop() {
	if pm.storePool != nil {
		pm.storePool.Stop()
	}
	if pm.agentPool != nil {
		pm.agentPool.Stop()
	}
}
