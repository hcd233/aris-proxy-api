// Package cron Session去重定时任务
//
//	author centonhuang
//	update 2026-03-19 10:00:00
package cron

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionDeduplicateCron Session去重定时任务，清理MessageIDs被其他Session包含的冗余Session
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type SessionDeduplicateCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     *lock.RedisLocker
	lockKey    string
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionDeduplicateCron 创建Session去重定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-06-01 10:00:00
func NewSessionDeduplicateCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SessionDeduplicateCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionDeduplicate)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}

// Stop 停止Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

// Start 启动Session去重定时任务
//
//	@receiver c *SessionDeduplicateCron
//	@param spec string cron 表达式
//	@return error
//	@author centonhuang
//	@update 2026-06-17 10:00:00
func (c *SessionDeduplicateCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionDeduplicate)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[SessionDeduplicateCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionDeduplicateCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// Trigger 手动触发一次 Session 去重
//
//	@receiver c *SessionDeduplicateCron
//	@return bool
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func (c *SessionDeduplicateCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate)
}

// deduplicate 执行Session去重逻辑
//
//	@receiver c *SessionDeduplicateCron
//	@author centonhuang
//	@update 2026-06-24 10:00:00
func (c *SessionDeduplicateCron) deduplicate(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.BatchGet(db, &dbmodel.Session{}, constant.SessionRepoFieldsDedup)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load sessions", zap.Error(err))
		return nil, err
	}

	checkedCount := int64(len(sessions))

	if len(sessions) < 2 {
		log.Info("[SessionDeduplicateCron] Skip deduplication, not enough sessions", zap.Int("count", len(sessions)))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	// 终端 tool_call 判定下推 SQL：只取末条消息为 assistant+tool_calls 的 message ID。
	// 不再需要 exclude 列表来缩小候选集（下推后单次查询约 1 KB），
	// merge target 的保护统一由 FindRedundantSessions 内部的 absorbed 集合表达。
	terminalMsgIDs, err := c.loadTerminalToolCallMsgIDs(db, sessions, nil)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load terminal tool call message ids", zap.Error(err))
		// 不return，退化为仅执行前缀去重
	}

	mergeResult := FindRedundantSessions(sessions, terminalMsgIDs)

	if len(mergeResult.RedundantIDs) == 0 {
		log.Info("[SessionDeduplicateCron] No redundant sessions found", zap.Int("total", len(sessions)))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	mergedCount, err := ApplyMergeResult(db, c.sessionDAO, mergeResult)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to apply deduplication", zap.Error(err))
		return nil, err
	}

	log.Info("[SessionDeduplicateCron] Deduplication completed",
		zap.Int("total", len(sessions)),
		zap.Int("deleted", len(mergeResult.RedundantIDs)),
		zap.Int("merged", mergedCount))

	return &commonmodel.CronCallAuditMetadata{
		CheckedSessions: checkedCount,
		DedupedSessions: int64(len(mergeResult.RedundantIDs)),
	}, nil
}

// ApplyMergeResult 在单个事务内写回 ToolIDs 合并结果并软删冗余 Session
//
// 原子性是必需的：若 tool_ids 更新失败却仍执行删除，被删 session 的 tool 引用
// 会永久丢失。任一步骤失败即整体回滚，任务幂等，下个整点重跑。
//
//	导出以便外部测试包验证写回行为。
//
//	@param db *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	mergedCount := 0

	err := db.Transaction(func(tx *gorm.DB) error {
		mergedCount = 0
		for sessionID, toolIDSet := range result.MergeMapping {
			if len(toolIDSet) == 0 {
				continue
			}

			// 将集合转换为排序后的切片，保证写入内容稳定
			mergedToolIDs := lo.Keys(toolIDSet)
			slices.Sort(mergedToolIDs)

			// tool_ids列为text类型(GORM serializer:json)，直接存JSON字符串
			toolIDsJSON, err := sonic.MarshalString(mergedToolIDs)
			if err != nil {
				return err
			}
			if err := sessionDAO.Update(tx, &dbmodel.Session{ID: sessionID}, map[string]any{
				constant.FieldToolIDs: toolIDsJSON,
			}); err != nil {
				return err
			}
			mergedCount++
		}

		return sessionDAO.BatchDeleteByField(tx, constant.WhereFieldID, result.RedundantIDs)
	})
	if err != nil {
		return 0, err
	}

	return mergedCount, nil
}

// loadTerminalToolCallMsgIDs 取出候选 session 的末条 message ID，并下推 SQL 筛出终端 tool_call 消息
//
//	lo.Uniq 后的 ID 仅用于 WHERE IN 查询，调用方按集合语义使用返回值，不依赖顺序。
//
//	@receiver c *SessionDeduplicateCron
//	@param db *gorm.DB
//	@param sessions []*dbmodel.Session
//	@param excludeIDs []uint
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func (c *SessionDeduplicateCron) loadTerminalToolCallMsgIDs(db *gorm.DB, sessions []*dbmodel.Session, excludeIDs []uint) ([]uint, error) {
	excludeSet := lo.SliceToMap(excludeIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })

	lastMsgIDs := lo.FilterMap(sessions, func(s *dbmodel.Session, _ int) (uint, bool) {
		if _, excluded := excludeSet[s.ID]; excluded {
			return 0, false
		}
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		return s.MessageIDs[len(s.MessageIDs)-1], true
	})

	if len(lastMsgIDs) == 0 {
		return nil, nil
	}

	return c.messageDAO.FilterTerminalToolCallIDs(db, lo.Uniq(lastMsgIDs))
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

// sessionEntry 用于表示Session在去重过程中的内部数据结构
//
//	@author centonhuang
//	@update 2026-06-04 10:00:00
type sessionEntry struct {
	id         uint
	messageIDs []uint
	toolIDs    []uint
}

// FindRedundantSessions 查找冗余 Session 并给出 ToolIDs 合并映射
//
// 算法：
//
//  1. 按首个 message ID 分组（同一对话的快照集合），组内按 MessageIDs 长度降序、ID 升序排列
//
//  2. 组内扫描并维护 keeper 列表：成员若是某个 keeper 的前缀则判为冗余，
//     ToolIDs 并入首个匹配的 keeper；否则自身成为新 keeper（处理对话分叉）
//
//  3. 对未吸收任何冗余成员的 session（含分叉 keeper 与单例组成员）应用终端规则：
//     末条 message 属于 terminalMsgIDs（assistant 且 tool_calls 非空，即对话在
//     工具调用处中断）则判为冗余。吸收过冗余成员的 keeper 是 merge target，
//     受保护不被删除，否则并入它的 ToolIDs 会随之丢失
//
//     @param sessions []*dbmodel.Session
//     @param terminalMsgIDs []uint 已判定为 assistant+tool_calls 的 message ID
//     @return MergeResult 包含需要删除的 Session ID 和 ToolIDs 合并映射
//     @author centonhuang
//     @update 2026-08-19 10:00:00
func FindRedundantSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) MergeResult {
	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}
	absorbed := make(map[uint]struct{})

	groups := groupByFirstMessageID(sessions)
	for _, entries := range groups {
		resolveGroup(entries, &result, absorbed)
	}

	applyTerminalRule(groups, terminalMsgIDs, absorbed, &result)

	return result
}

// applyTerminalRule 对未吸收冗余成员的 session 应用终端 tool_call 规则
//
//	保护条件是「是否吸收过冗余成员」而非「是否有 tool_ids」：merge target 被删会让
//	并入它的 ToolIDs 一起丢失。旧实现用 MergeMapping 的 key 作代理，而双方 tool_ids
//	全空时该 key 不会创建，于是无 tool 的 merge target 失去保护、连同整组被删。
//
//	@param groups map[uint][]sessionEntry
//	@param terminalMsgIDs []uint
//	@param absorbed map[uint]struct{} 已吸收冗余成员的 merge target，受保护
//	@param result *MergeResult
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func applyTerminalRule(groups map[uint][]sessionEntry, terminalMsgIDs []uint, absorbed map[uint]struct{}, result *MergeResult) {
	if len(terminalMsgIDs) == 0 {
		return
	}

	terminalSet := lo.SliceToMap(terminalMsgIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	redundantSet := lo.SliceToMap(result.RedundantIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })

	for _, entries := range groups {
		for _, e := range entries {
			if _, protected := absorbed[e.id]; protected {
				continue
			}
			if _, already := redundantSet[e.id]; already {
				continue
			}
			if _, terminal := terminalSet[e.messageIDs[len(e.messageIDs)-1]]; !terminal {
				continue
			}
			result.RedundantIDs = append(result.RedundantIDs, e.id)
		}
	}
}

// groupByFirstMessageID 按首个 message ID 将 session 分组，组内按 MessageIDs 长度降序、ID 升序排列
//
// 同一对话的所有快照必然共享首个 message ID：每次请求都把完整对话历史按 checksum
// 去重后落库，历史消息复用同一行，故第 k 轮的 MessageIDs 是第 k+1 轮的前缀。
// 跨组不可能存在冗余关系，分组把两两比较从 O(N²) 降到 Σ(组内²)。
//
// 生产实测（2026-08-19）：2551 个 session → 913 组、603 个单例组，
// 比较次数 6,507,601 → 145,503（44.7 倍）。
//
//	MessageIDs 为空的 session 被跳过，不参与去重。
//
//	@param sessions []*dbmodel.Session
//	@return map[uint][]sessionEntry key 为首个 message ID
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func groupByFirstMessageID(sessions []*dbmodel.Session) map[uint][]sessionEntry {
	groups := make(map[uint][]sessionEntry)

	for _, s := range sessions {
		if len(s.MessageIDs) == 0 {
			continue
		}
		firstMsgID := s.MessageIDs[0]
		groups[firstMsgID] = append(groups[firstMsgID], sessionEntry{
			id:         s.ID,
			messageIDs: s.MessageIDs,
			toolIDs:    s.ToolIDs,
		})
	}

	for _, entries := range groups {
		slices.SortFunc(entries, func(a, b sessionEntry) int {
			if len(a.messageIDs) != len(b.messageIDs) {
				return len(b.messageIDs) - len(a.messageIDs)
			}
			return cmp.Compare(a.id, b.id)
		})
	}

	return groups
}

// resolveGroup 处理单个对话组：前缀成员判为冗余并把 ToolIDs 并入首个匹配的 keeper
//
//	keepers 按加入顺序（即长度降序、ID 升序）遍历，故「首个匹配的 keeper」
//	是最长且 ID 最小的容器，与保留较早 Session 的既有语义一致。
//
//	@param entries []sessionEntry 组内条目，已按长度降序、ID 升序排列
//	@param result *MergeResult
//	@param absorbed map[uint]struct{} 记录吸收过冗余成员的 keeper（merge target）
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func resolveGroup(entries []sessionEntry, result *MergeResult, absorbed map[uint]struct{}) {
	keepers := make([]sessionEntry, 0, len(entries))

	for _, e := range entries {
		container, found := lo.Find(keepers, func(k sessionEntry) bool {
			return isPrefix(e.messageIDs, k.messageIDs)
		})
		if !found {
			keepers = append(keepers, e)
			continue
		}

		result.RedundantIDs = append(result.RedundantIDs, e.id)
		mergeToolIDsIntoMapping(result.MergeMapping, container.id, container.toolIDs, e.toolIDs)
		// 记录「吸收过冗余成员」而非「落进 MergeMapping」：双方 tool_ids 全空时
		// mergeToolIDsIntoMapping 不会建条目，用它作保护代理会漏掉这类 merge target
		absorbed[container.id] = struct{}{}
	}
}

// isPrefix 判断 short 是否是 long 的前缀
//
//	长度比较先做 O(1) 剪枝，避免无谓的逐元素比较。
//
//	@param short []uint
//	@param long []uint
//	@return bool
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func isPrefix(short, long []uint) bool {
	if len(short) > len(long) {
		return false
	}
	return slices.Equal(long[:len(short)], short)
}

// mergeToolIDsIntoMapping 将 target 和 source 的 ToolIDs 合并到 mapping 中指定 targetID 的条目
//
//	@param mapping map[uint]map[uint]struct{}
//	@param targetID uint
//	@param targetToolIDs []uint
//	@param sourceToolIDs []uint
//	@author centonhuang
//	@update 2026-06-04 10:00:00
func mergeToolIDsIntoMapping(mapping map[uint]map[uint]struct{}, targetID uint, targetToolIDs, sourceToolIDs []uint) {
	if len(targetToolIDs) == 0 && len(sourceToolIDs) == 0 {
		return
	}
	if mapping[targetID] == nil {
		mapping[targetID] = make(map[uint]struct{})
	}
	for _, tid := range targetToolIDs {
		mapping[targetID][tid] = struct{}{}
	}
	for _, tid := range sourceToolIDs {
		mapping[targetID][tid] = struct{}{}
	}
}
