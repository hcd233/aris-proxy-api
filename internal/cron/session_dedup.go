// Package cron Session去重定时任务
//
//	author centonhuang
//	update 2026-03-19 10:00:00
package cron

import (
	"context"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionDeduplicateCron Session去重定时任务，清理MessageIDs被其他Session包含的冗余Session
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type SessionDeduplicateCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionDeduplicateCron 创建Session去重定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func NewSessionDeduplicateCron(db *gorm.DB) Cron {
	return &SessionDeduplicateCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionDeduplicate)),
		),
		db:         db,
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
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
	// 每小时执行一次，定期清理冗余Session
	entryID, err := c.cron.AddFunc(constant.CronSpecSessionDeduplicate, c.deduplicate)
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
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, constant.SessionRepoFieldsDedup)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load sessions", zap.Error(err))
		return
	}

	if len(sessions) < 2 {
		log.Info("[SessionDeduplicateCron] Skip deduplication, not enough sessions", zap.Int("count", len(sessions)))
		return
	}

	mergeResult := FindRedundantSessionsWithMerge(sessions)

	// 额外检查：Session最后一条消息是assistant且有tool_calls的也标记为冗余
	messages, err := c.loadLastMessagesForTerminalToolCheck(db, sessions, mergeResult.RedundantIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load last messages for terminal tool call check", zap.Error(err))
		// 不return，继续执行已有的去重结果
	}

	if len(messages) > 0 {
		terminalToolCallResult := FindTerminalToolCallSessions(sessions, messages, mergeResult.RedundantIDs)
		if len(terminalToolCallResult.RedundantIDs) > 0 {
			mergeResult.RedundantIDs = append(mergeResult.RedundantIDs, terminalToolCallResult.RedundantIDs...)

			// 合并TerminalToolCall的ToolIDs映射到主结果
			for sessionID, toolIDSet := range terminalToolCallResult.MergeMapping {
				mergeResult.MergeMapping[sessionID] = mergeToolIDs(mergeResult.MergeMapping[sessionID], toolIDSet)
			}
		}
	}

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
		slices.Sort(mergedToolIDs)

		// tool_ids列为text类型(GORM serializer:json)，直接存JSON字符串
		err := c.sessionDAO.Update(db, &dbmodel.Session{ID: sessionID}, map[string]any{
			constant.FieldToolIDs: lo.Must1(sonic.MarshalString(mergedToolIDs)),
		})
		if err != nil {
			log.Error("[SessionDeduplicateCron] Failed to update session tool_ids",
				zap.Uint("sessionID", sessionID),
				zap.Error(err))
			continue
		}
		mergedCount++
	}

	err = c.sessionDAO.BatchDeleteByField(db, constant.WhereFieldID, mergeResult.RedundantIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to delete redundant sessions", zap.Error(err))
		return
	}

	log.Info("[SessionDeduplicateCron] Deduplication completed",
		zap.Int("total", len(sessions)),
		zap.Int("deleted", len(mergeResult.RedundantIDs)),
		zap.Int("merged", mergedCount))
}

func (c *SessionDeduplicateCron) loadLastMessagesForTerminalToolCheck(db *gorm.DB, sessions []*dbmodel.Session, excludeIDs []uint) ([]*dbmodel.Message, error) {
	excludeSet := make(map[uint]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = struct{}{}
	}

	lastMsgIDs := make([]uint, 0)
	for _, s := range sessions {
		if _, excluded := excludeSet[s.ID]; excluded {
			continue
		}
		if len(s.MessageIDs) == 0 {
			continue
		}
		lastMsgIDs = append(lastMsgIDs, s.MessageIDs[len(s.MessageIDs)-1])
	}

	if len(lastMsgIDs) == 0 {
		return nil, nil
	}

	// lo.Uniq 去重保留首次出现顺序，但传入 BatchGetByField 做 WHERE IN 查询时
	// 数据库不保证返回顺序与 IN 子句一致。当前代码用 msgMap 按 ID 映射取值，
	// 因此不依赖查询结果的顺序。如需依赖顺序，须在此处手动排序。
	uniqIDs := lo.Uniq(lastMsgIDs)
	messages, err := c.messageDAO.BatchGetByField(db, constant.WhereFieldID, uniqIDs,
		[]string{constant.FieldID, constant.FieldMessage})
	if err != nil {
		return nil, err
	}

	return messages, nil
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
	slices.SortFunc(entries, func(a, b sessionEntry) int {
		if len(a.messageIDs) != len(b.messageIDs) {
			return len(b.messageIDs) - len(a.messageIDs) // 降序
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
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
	for i := range entries {
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
				if len(shorter.toolIDs) > 0 || len(longer.toolIDs) > 0 {
					if mergeMapping[longer.id] == nil {
						mergeMapping[longer.id] = make(map[uint]struct{})
					}
					// 先把长Session自己的ToolIDs加入集合
					for _, tid := range longer.toolIDs {
						mergeMapping[longer.id][tid] = struct{}{}
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

// mergeToolIDs 合并两个 ToolID 集合，返回新的集合
//
//	@param existing map[uint]struct{} 已有的 ToolID 集合（可以为 nil）
//	@param incoming map[uint]struct{} 需要合并的 ToolID 集合
//	@return map[uint]struct{} 合并后的集合
//	@author centonhuang
//	@update 2026-05-24 10:00:00
func mergeToolIDs(existing, incoming map[uint]struct{}) map[uint]struct{} {
	if len(incoming) == 0 {
		return existing
	}
	if existing == nil {
		existing = make(map[uint]struct{}, len(incoming))
	}
	for tid := range incoming {
		existing[tid] = struct{}{}
	}
	return existing
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

// FindTerminalToolCallSessions 查找最后一条消息是assistant且有tool_calls的session
//
// 这些session的对话在工具调用阶段中断，属于不完整分支。
// 标记为冗余并尝试查找parent session以合并ToolIDs。
//
//	@param sessions []*dbmodel.Session
//	@param messages []*dbmodel.Message
//	@param excludeIDs []uint 已被子数组检查标记为冗余的session ID
//	@return MergeResult
//	@author centonhuang
//	@update 2026-05-24 10:00:00
func FindTerminalToolCallSessions(sessions []*dbmodel.Session, messages []*dbmodel.Message, excludeIDs []uint) MergeResult {
	excludeSet := make(map[uint]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = struct{}{}
	}

	msgMap := make(map[uint]*dbmodel.Message, len(messages))
	for _, m := range messages {
		msgMap[m.ID] = m
	}

	sessionByID := make(map[uint]*dbmodel.Session, len(sessions))
	for _, s := range sessions {
		sessionByID[s.ID] = s
	}

	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}

	for _, s := range sessions {
		if _, excluded := excludeSet[s.ID]; excluded {
			continue
		}
		if len(s.MessageIDs) == 0 {
			continue
		}

		lastMsgID := s.MessageIDs[len(s.MessageIDs)-1]
		msg, ok := msgMap[lastMsgID]
		if !ok || msg.Message == nil {
			continue
		}

		if msg.Message.Role == enum.RoleAssistant && len(msg.Message.ToolCalls) > 0 {
			result.RedundantIDs = append(result.RedundantIDs, s.ID)

			if len(s.ToolIDs) > 0 {
				parentID := findParentSessionID(s, sessions)
				if parentID > 0 {
					parentToolIDSet := make(map[uint]struct{})
					for _, tid := range sessionByID[parentID].ToolIDs {
						parentToolIDSet[tid] = struct{}{}
					}
					for _, tid := range s.ToolIDs {
						parentToolIDSet[tid] = struct{}{}
					}
					result.MergeMapping[parentID] = parentToolIDSet
				}
			}
		}
	}

	return result
}

func findParentSessionID(target *dbmodel.Session, sessions []*dbmodel.Session) uint {
	if len(target.MessageIDs) == 0 {
		return 0
	}

	// 选择 MessageIDs 最长且 ID 最小的 session 作为 parent，保证确定性
	var parentID uint
	var parentLen int
	for _, s := range sessions {
		if s.ID == target.ID {
			continue
		}
		if len(s.MessageIDs) <= len(target.MessageIDs) {
			continue
		}
		if !IsSubArray(target.MessageIDs, s.MessageIDs) {
			continue
		}
		// 优先选 MessageIDs 最长的；长度相同选 ID 最小的
		if len(s.MessageIDs) > parentLen || (len(s.MessageIDs) == parentLen && s.ID < parentID) {
			parentID = s.ID
			parentLen = len(s.MessageIDs)
		}
	}
	return parentID
}
