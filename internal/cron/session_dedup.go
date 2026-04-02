// Package cron Session去重定时任务
//
//	author centonhuang
//	update 2026-03-19 10:00:00
package cron

import (
	"context"
	"sort"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// SessionDeduplicateCron Session去重定时任务，清理MessageIDs被其他Session包含的冗余Session
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type SessionDeduplicateCron struct {
	cron       *cron.Cron
	sessionDAO *dao.SessionDAO
}

// NewSessionDeduplicateCron 创建Session去重定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func NewSessionDeduplicateCron() Cron {
	return &SessionDeduplicateCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter("SessionDeduplicateCron", logger.Logger())),
		),
		sessionDAO: dao.GetSessionDAO(),
	}
}

// Stop 停止Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func (c *SessionDeduplicateCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// Start 启动Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@return error
//	@author centonhuang
//	@update 2026-04-03 10:00:00
func (c *SessionDeduplicateCron) Start() error {
	// 每天凌晨1:00执行，为后续摘要和评分任务减少处理量
	entryID, err := c.cron.AddFunc("0 1 * * *", c.deduplicate)
	if err != nil {
		logger.Logger().Error("[SessionDeduplicateCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionDeduplicateCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// deduplicate 执行Session去重逻辑
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-03-30 10:00:00
func (c *SessionDeduplicateCron) deduplicate() {
	ctx := context.WithValue(context.Background(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, []string{"id", "message_ids", "tool_ids"})
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load sessions", zap.Error(err))
		return
	}

	if len(sessions) < 2 {
		log.Info("[SessionDeduplicateCron] Skip deduplication, not enough sessions", zap.Int("count", len(sessions)))
		return
	}

	mergeResult := FindRedundantSessionsWithMerge(sessions)
	if len(mergeResult.RedundantIDs) == 0 {
		log.Info("[SessionDeduplicateCron] No redundant sessions found", zap.Int("total", len(sessions)))
		return
	}

	// 合并ToolIDs到保留的Session
	mergedCount := 0
	for sessionID, toolIDSet := range mergeResult.MergeMapping {
		if len(toolIDSet) == 0 {
			continue
		}

		// 将集合转换为排序后的切片
		mergedToolIDs := make([]uint, 0, len(toolIDSet))
		for tid := range toolIDSet {
			mergedToolIDs = append(mergedToolIDs, tid)
		}
		sort.Slice(mergedToolIDs, func(i, j int) bool {
			return mergedToolIDs[i] < mergedToolIDs[j]
		})

		// tool_ids列为text类型(GORM serializer:json)，直接存JSON字符串
		err := c.sessionDAO.Update(db, &dbmodel.Session{ID: sessionID}, map[string]interface{}{
			"tool_ids": lo.Must1(sonic.MarshalString(mergedToolIDs)),
		})
		if err != nil {
			log.Error("[SessionDeduplicateCron] Failed to update session tool_ids",
				zap.Uint("sessionID", sessionID),
				zap.Error(err))
			continue
		}
		mergedCount++
	}

	err = c.sessionDAO.BatchDeleteByField(db, "id", mergeResult.RedundantIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to delete redundant sessions", zap.Error(err))
		return
	}

	log.Info("[SessionDeduplicateCron] Deduplication completed",
		zap.Int("total", len(sessions)),
		zap.Int("deleted", len(mergeResult.RedundantIDs)),
		zap.Int("merged", mergedCount))
}

// MergeResult 表示Session去重后的合并结果
//
//	@author centonhuang
//	@update 2026-03-30 10:00:00
type MergeResult struct {
	// RedundantIDs 需要删除的Session ID列表
	RedundantIDs []uint
	// MergeMapping 长Session ID -> 需要合并的ToolIDs（来自被删除的短Session）
	MergeMapping map[uint]map[uint]struct{}
}

// FindRedundantSessionsWithMerge 查找MessageIDs被其他Session完全包含（子数组）的冗余Session，并返回ToolIDs合并信息
//
// 算法：
//
//  1. 按MessageIDs长度降序排序，长度相同则按ID升序（保留较早的Session）
//
//  2. 对每个Session，构建首元素索引加速查找
//
//  3. 短Session的MessageIDs如果是某个长Session的连续子数组，则标记为冗余
//
//  4. 将被标记为冗余的Session的ToolIDs与保留的Session取并集
//
//     @param sessions []*dbmodel.Session
//     @return MergeResult 包含需要删除的Session ID和ToolIDs合并映射
//     @author centonhuang
//     @update 2026-03-30 10:00:00
func FindRedundantSessionsWithMerge(sessions []*dbmodel.Session) MergeResult {
	type sessionEntry struct {
		id         uint
		messageIDs []uint
		toolIDs    []uint
	}

	entries := lo.Map(sessions, func(s *dbmodel.Session, _ int) sessionEntry {
		return sessionEntry{id: s.ID, messageIDs: s.MessageIDs, toolIDs: s.ToolIDs}
	})

	// 按MessageIDs长度降序排序，长度相同按ID升序（保留较早的）
	sort.Slice(entries, func(i, j int) bool {
		if len(entries[i].messageIDs) != len(entries[j].messageIDs) {
			return len(entries[i].messageIDs) > len(entries[j].messageIDs)
		}
		return entries[i].id < entries[j].id
	})

	// 过滤掉空MessageIDs的Session
	entries = lo.Filter(entries, func(e sessionEntry, _ int) bool {
		return len(e.messageIDs) > 0
	})

	redundantIDs := make([]uint, 0)
	redundantSet := make(map[uint]struct{})
	// mergeMapping: 长Session ID -> 需要合并的ToolID集合
	mergeMapping := make(map[uint]map[uint]struct{})

	// 对每个Session，检查它是否是已知非冗余Session的子数组
	// 从长到短遍历，短的只需要和比它长的比较
	for i := 0; i < len(entries); i++ {
		if _, redundant := redundantSet[entries[i].id]; redundant {
			continue
		}

		for j := i + 1; j < len(entries); j++ {
			if _, redundant := redundantSet[entries[j].id]; redundant {
				continue
			}

			shorter := entries[j]
			longer := entries[i]

			isRedundant := false

			// 长度相同时检查完全相等（保留ID较小的，即entries[i]）
			if len(shorter.messageIDs) == len(longer.messageIDs) {
				if isEqualSlice(shorter.messageIDs, longer.messageIDs) {
					isRedundant = true
				}
			} else if IsSubArray(shorter.messageIDs, longer.messageIDs) {
				// shorter 比 longer 短，检查 shorter 是否是 longer 的连续子数组
				isRedundant = true
			}

			if isRedundant {
				redundantSet[shorter.id] = struct{}{}
				redundantIDs = append(redundantIDs, shorter.id)

				// 将短Session的ToolIDs合并到长Session
				if len(shorter.toolIDs) > 0 {
					if mergeMapping[longer.id] == nil {
						mergeMapping[longer.id] = make(map[uint]struct{})
						// 先把长Session自己的ToolIDs加入集合
						for _, tid := range longer.toolIDs {
							mergeMapping[longer.id][tid] = struct{}{}
						}
					}
					// 合并短Session的ToolIDs
					for _, tid := range shorter.toolIDs {
						mergeMapping[longer.id][tid] = struct{}{}
					}
				}
			}
		}
	}

	return MergeResult{
		RedundantIDs: redundantIDs,
		MergeMapping: mergeMapping,
	}
}

// FindRedundantSessions 查找MessageIDs被其他Session完全包含（子数组）的冗余Session
//
// 算法：
//
//  1. 按MessageIDs长度降序排序，长度相同则按ID升序（保留较早的Session）
//
//  2. 对每个Session，构建首元素索引加速查找
//
//  3. 短Session的MessageIDs如果是某个长Session的连续子数组，则标记为冗余
//
//     @param sessions []*dbmodel.Session
//     @return []uint 需要删除的Session ID列表
//     @author centonhuang
//     @update 2026-03-19 10:00:00
func FindRedundantSessions(sessions []*dbmodel.Session) []uint {
	result := FindRedundantSessionsWithMerge(sessions)
	return result.RedundantIDs
}

// IsSubArray 判断 sub 是否是 arr 的连续子数组
//
//	使用滑动窗口算法，时间复杂度 O(n*m)，其中 n=len(arr), m=len(sub)
//	@param sub []uint 候选子数组
//	@param arr []uint 母数组
//	@return bool
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func IsSubArray(sub, arr []uint) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(arr) {
		return false
	}

	// 滑动窗口：在 arr 中寻找与 sub 完全匹配的连续片段
	limit := len(arr) - len(sub)
	for i := 0; i <= limit; i++ {
		if arr[i] != sub[0] {
			continue
		}

		matched := true
		for j := 1; j < len(sub); j++ {
			if arr[i+j] != sub[j] {
				matched = false
				break
			}
		}

		if matched {
			return true
		}
	}

	return false
}

// isEqualSlice 判断两个 uint 切片是否完全相等
//
//	@param a []uint
//	@param b []uint
//	@return bool
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func isEqualSlice(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
