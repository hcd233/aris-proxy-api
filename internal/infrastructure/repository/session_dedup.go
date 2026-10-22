// Package repository Session 前缀去重算法与写回
//
// 自 internal/cron/session_dedup.go 提取（2026-08-29）：前缀去重在 session 插入
// 事务提交后实时执行（internal/infrastructure/pool/store_pool.go），cron 仅承担
// terminal 终态清理。
//
//	author centonhuang
//	update 2026-08-29 10:00:00
package repository

import (
	"cmp"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

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

// FindRedundantSessions 查找冗余 Session 并给出 ToolIDs 合并映射（纯前缀规则）
//
// 算法：
//
//  1. 按首个 message ID 分组（同一对话的快照集合），组内按 MessageIDs 长度降序、ID 升序排列
//
//  2. 组内扫描并维护 keeper 列表：成员若是某个 keeper 的前缀则判为冗余，
//     ToolIDs 并入首个匹配的 keeper；否则自身成为新 keeper（处理对话分叉）
//
//     同一对话的所有快照必然共享首个 message ID：每次请求都把完整对话历史按 checksum
//     去重后落库，历史消息复用同一行，故第 k 轮的 MessageIDs 是第 k+1 轮的前缀。
//     跨组不可能存在冗余关系，分组把两两比较从 O(N²) 降到 Σ(组内²)。
//
//     MessageIDs 为空的 session 被跳过，不参与去重。
//
//     @param sessions []*dbmodel.Session
//     @return MergeResult 包含需要删除的 Session ID 和 ToolIDs 合并映射
//     @author centonhuang
//     @update 2026-08-29 10:00:00
func FindRedundantSessions(sessions []*dbmodel.Session) MergeResult {
	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}

	groups := groupByFirstMessageID(sessions)
	for _, entries := range groups {
		resolveGroup(entries, &result)
	}

	return result
}

// groupByFirstMessageID 按首个 message ID 将 session 分组，组内按 MessageIDs 长度降序、ID 升序排列
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
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func resolveGroup(entries []sessionEntry, result *MergeResult) {
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

// FindSessionTarget 返回同组中承接本轮快照的既有会话；nil 表示没有会话可续写
//
// 只认「既有会话是新快照的前缀」这一种关系：同组内最长者优先（等长取 ID 最小），
// 保证同一段对话始终落在同一行上。新快照被既有会话包含（重试/补发旧轮次）或与
// 既有会话无前缀关系（对话分叉）时返回 nil，由 FindRedundantSessions 按既有规则处理。
//
//	@param sessions []*dbmodel.Session 同组会话（不含本轮新行）
//	@param messageIDs []uint 本轮快照的消息 ID 列表
//	@return *dbmodel.Session 承接本轮快照的会话；nil 表示保留新行
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func FindSessionTarget(sessions []*dbmodel.Session, messageIDs []uint) *dbmodel.Session {
	var target *dbmodel.Session
	for _, s := range sessions {
		if len(s.MessageIDs) == 0 || !isPrefix(s.MessageIDs, messageIDs) {
			continue
		}
		if target == nil {
			target = s
			continue
		}
		longer := len(s.MessageIDs) > len(target.MessageIDs)
		earlierTie := len(s.MessageIDs) == len(target.MessageIDs) && s.ID < target.ID
		if longer || earlierTie {
			target = s
		}
	}
	return target
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

// ApplyMergeResult 在单个事务内写回 ToolIDs 合并结果并软删冗余 Session
//
// 原子性是必需的：若 tool_ids 更新失败却仍执行删除，被删 session 的 tool 引用
// 会永久丢失。任一步骤失败即整体回滚。
//
//	@param db *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	var mergedCount int
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		mergedCount, err = applyMergeInTx(tx, sessionDAO, result)
		return err
	})
	if err != nil {
		return 0, err
	}
	return mergedCount, nil
}

// applyMergeInTx 在已开启的事务内执行合并写回，不自行开启事务，
// 供 ApplyMergeResult（自管事务）与 DeduplicateSessionGroup（复用外层事务）共用。
//
// tool_ids 列为 text 类型（GORM serializer:json），Updates(map) 不触发序列化，
// 必须显式 sonic.MarshalString 后存 JSON 字符串。
//
//	@param tx *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func applyMergeInTx(tx *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	mergedCount := 0
	for sessionID, toolIDSet := range result.MergeMapping {
		if len(toolIDSet) == 0 {
			continue
		}

		// 将集合转换为排序后的切片，保证写入内容稳定
		mergedToolIDs := lo.Keys(toolIDSet)
		slices.Sort(mergedToolIDs)

		toolIDsJSON, err := sonic.MarshalString(mergedToolIDs)
		if err != nil {
			return 0, err
		}
		if err := sessionDAO.Update(tx, &dbmodel.Session{ID: sessionID}, map[string]any{
			constant.FieldToolIDs: toolIDsJSON,
		}); err != nil {
			return 0, err
		}
		mergedCount++
	}

	if err := sessionDAO.BatchDeleteByField(tx, constant.WhereFieldID, result.RedundantIDs); err != nil {
		return 0, err
	}

	return mergedCount, nil
}

// DeduplicateSessionGroup 在单个短事务内锁定同组会话，把本轮快照合并回既有会话
//
// 由 session 插入路径在主事务提交后调用（store_pool）：组行 FOR UPDATE 串行化
// 同组并发写入。续写时把新行的快照字段前滚到既有 keeper 并软删新行，会话 ID、
// 评分与分享关系因此跨轮保持稳定；首轮与对话分叉保留新行。其余陈旧前缀行仍按
// 既有规则合并删除。空组竞态下并发首插各留一份，下次同组插入自愈。
//
// ::jsonb 为 PG 专有语法，sqlite 不可用，行为由 e2e 覆盖。
//
//	@param db *gorm.DB
//	@param session *dbmodel.Session 本轮已提交的新行
//	@return int 软删的冗余会话数
//	@return uint 承接快照的 keeper ID（0 表示新行自持）
//	@return error
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func DeduplicateSessionGroup(db *gorm.DB, session *dbmodel.Session) (deduped int, keeperID uint, err error) {
	if len(session.MessageIDs) == 0 {
		return 0, 0, nil
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		sessions, findErr := dao.GetSessionDAO().FindGroupForUpdate(tx, session.MessageIDs[0])
		if findErr != nil {
			return findErr
		}

		keeper := FindSessionTarget(excludeSession(sessions, session.ID), session.MessageIDs)
		if keeper != nil {
			if writeErr := writeBackSnapshot(tx, keeper, session); writeErr != nil {
				return writeErr
			}
			if deleteErr := dao.GetSessionDAO().BatchDeleteByField(tx, constant.WhereFieldID, []uint{session.ID}); deleteErr != nil {
				return deleteErr
			}
			keeperID = keeper.ID
			deduped++
			// 内存视图同步写回结果，供后续按 keeper 清理其它陈旧前缀行
			keeper.MessageIDs = session.MessageIDs
			keeper.ToolIDs = unionIDs(keeper.ToolIDs, session.ToolIDs)
			sessions = excludeSession(sessions, session.ID)
		}

		result := FindRedundantSessions(sessions)
		if len(result.RedundantIDs) == 0 {
			return nil
		}
		deduped += len(result.RedundantIDs)

		_, mergeErr := applyMergeInTx(tx, dao.GetSessionDAO(), result)
		return mergeErr
	})
	if err != nil {
		return 0, 0, err
	}
	return deduped, keeperID, nil
}

// writeBackSnapshot 把本轮快照写回既有会话，保留其 ID、评分与创建时间
//
// keeper 是本轮快照的前缀。长度相同说明是同一快照的重试，只合并工具引用，
// 不覆盖既有快照字段；更长则把消息/提问/模型/元数据整体前滚。
// tool_ids 等列为 text 类型（GORM serializer:json），Updates(map) 不触发序列化，
// 必须显式 sonic.MarshalString 后存 JSON 字符串。
//
//	@param tx *gorm.DB
//	@param keeper *dbmodel.Session 承接快照的既有会话
//	@param snapshot *dbmodel.Session 本轮已提交的新行
//	@return error
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func writeBackSnapshot(tx *gorm.DB, keeper, snapshot *dbmodel.Session) error {
	fields := map[string]any{
		constant.FieldToolIDs: unionIDs(keeper.ToolIDs, snapshot.ToolIDs),
	}
	if len(snapshot.MessageIDs) > len(keeper.MessageIDs) {
		fields[constant.FieldMessageIDs] = snapshot.MessageIDs
		fields[constant.FieldQuestions] = snapshot.Questions
		fields[constant.FieldModelIDs] = snapshot.ModelIDs
		fields[constant.FieldMetadata] = snapshot.Metadata
	}

	updates := make(map[string]any, len(fields))
	for column, value := range fields {
		payload, err := sonic.MarshalString(value)
		if err != nil {
			return err
		}
		updates[column] = payload
	}
	return dao.GetSessionDAO().Update(tx, &dbmodel.Session{ID: keeper.ID}, updates)
}

// unionIDs 合并多组 ID、去重并排序，保证写入内容稳定
//
//	@param lists ...[]uint
//	@return []uint
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func unionIDs(lists ...[]uint) []uint {
	merged := lo.Uniq(slices.Concat(lists...))
	slices.Sort(merged)
	return merged
}

// excludeSession 过滤掉指定会话
//
//	@param sessions []*dbmodel.Session
//	@param sessionID uint
//	@return []*dbmodel.Session
//	@author centonhuang
//	@update 2026-10-22 10:00:00
func excludeSession(sessions []*dbmodel.Session, sessionID uint) []*dbmodel.Session {
	return lo.Filter(sessions, func(s *dbmodel.Session, _ int) bool { return s.ID != sessionID })
}
