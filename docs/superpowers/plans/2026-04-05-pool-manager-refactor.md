# Pool Manager 重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构 Pool Manager 为按对象分组的双池架构（Store 池 + Agent 池），Manager 瘦身为纯分发器

**Architecture:** 按处理对象分组：Store 池处理消息/工具存储任务，Agent 池处理 AI 能力任务（总结、评分）。配置按组分，Manager 不直接持有业务依赖。

**Tech Stack:** Go, Pond v2, Viper

---

## 文件变更概览

| 操作 | 文件 | 职责 |
|-----|------|------|
| 修改 | `internal/config/config.go` | 新增 `PoolConfig` 结构体，替换标量配置 |
| 修改 | `internal/agent/summarizer.go` | 新增 `GetSummarizer()` 单例访问 |
| 修改 | `internal/agent/scorer.go` | 新增 `GetScorer()` 单例访问 |
| 重写 | `internal/infrastructure/pool/pool.go` | PoolManager 精简为分发器 |
| 新增 | `internal/infrastructure/pool/store_pool.go` | Store 池任务处理（MessageStore） |
| 新增 | `internal/infrastructure/pool/agent_pool.go` | Agent 池任务处理（Summarize、Score） |
| 修改 | `config/config.yaml.template` | 新增 store/agent 分组配置 |
| 修改 | `config/config.yaml` | 同上 |

---

## Task 1: Config 结构体重构

**Files:**
- Modify: `internal/config/config.go`

### 现有代码 (lines 162-168):
```go
// PoolWorkers int 协程池工作协程数
//	@update 2026-01-31 03:26:11
PoolWorkers int

// PoolQueueSize int 协程池任务队列大小
//	@update 2026-01-31 03:26:08
PoolQueueSize int
```

### 现有 initEnvironment 代码 (lines 196-197, 259-260):
```go
config.SetDefault("pool.workers", 8)
config.SetDefault("pool.queue.size", 64)
// ...
PoolWorkers = config.GetInt("pool.workers")
PoolQueueSize = config.GetInt("pool.queue.size")
```

- [ ] **Step 1: 添加 PoolConfig 结构体和变量**

在 `initEnvironment()` 函数之前添加：

```go
// PoolGroupConfig 协程池分组配置
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolGroupConfig struct {
	Workers   int
	QueueSize int
}

// PoolConfig 协程池配置
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolConfig struct {
	Store PoolGroupConfig
	Agent PoolGroupConfig
}

// Pool Store 池和 Agent 池的全局配置
var Pool PoolConfig
```

- [ ] **Step 2: 替换 PoolWorkers/PoolQueueSize 变量**

删除原有的 `PoolWorkers int` 和 `PoolQueueSize int` 声明（约 lines 162-168）。

- [ ] **Step 3: 更新 initEnvironment 中的默认值**

将原有的：
```go
config.SetDefault("pool.workers", 8)
config.SetDefault("pool.queue.size", 64)
```

替换为：
```go
config.SetDefault("pool.store.workers", 50)
config.SetDefault("pool.store.queue_size", 1000)
config.SetDefault("pool.agent.workers", 10)
config.SetDefault("pool.agent.queue_size", 100)
```

- [ ] **Step 4: 更新配置读取**

将原有的：
```go
PoolWorkers = config.GetInt("pool.workers")
PoolQueueSize = config.GetInt("pool.queue.size")
```

替换为：
```go
Pool = PoolConfig{
	Store: PoolGroupConfig{
		Workers:   config.GetInt("pool.store.workers"),
		QueueSize: config.GetInt("pool.store.queue_size"),
	},
	Agent: PoolGroupConfig{
		Workers:   config.GetInt("pool.agent.workers"),
		QueueSize: config.GetInt("pool.agent.queue_size"),
	},
}
```

- [ ] **Step 5: 提交**

```bash
git add internal/config/config.go
git commit -m "refactor(config): replace pool scalars with PoolConfig struct"
```

---

## Task 2: Agent 单例化

**Files:**
- Modify: `internal/agent/summarizer.go`
- Modify: `internal/agent/scorer.go`

- [ ] **Step 1: 在 summarizer.go 添加单例访问**

在 `NewSummarizer` 函数之后、`Summarize` 方法之前添加：

```go
var summarizer *Summarizer

// GetSummarizer 获取全局 Summarizer 单例
//
//	@return *Summarizer
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func GetSummarizer() *Summarizer {
	if summarizer == nil {
		summarizer = lo.Must1(NewSummarizer())
	}
	return summarizer
}
```

- [ ] **Step 2: 在 scorer.go 添加单例访问**

在 `NewScorer` 函数之后、`Score` 方法之前添加：

```go
var scorer *Scorer

// GetScorer 获取全局 Scorer 单例
//
//	@return *Scorer
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func GetScorer() *Scorer {
	if scorer == nil {
		scorer = lo.Must1(NewScorer())
	}
	return scorer
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/agent/summarizer.go internal/agent/scorer.go
git commit -m "feat(agent): add GetSummarizer and GetScorer singleton accessors"
```

---

## Task 3: 创建 Store 池

**Files:**
- Create: `internal/infrastructure/pool/store_pool.go`

- [ ] **Step 1: 创建 store_pool.go**

```go
// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// submitMessageStoreTask 提交消息存储任务到 Store 池
//
//	@param pm *PoolManager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) submitMessageStoreTask(task *dto.MessageStoreTask) error {
	logger := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)
	messageDAO := dao.GetMessageDAO()
	sessionDAO := dao.GetSessionDAO()
	toolDAO := dao.GetToolDAO()

	return pm.storePool.Go(func() {
		toolSchemas := util.ToolSchemaMap{}
		for _, t := range task.Tools {
			if t.Parameters != nil {
				toolSchemas[t.Name] = t.Parameters
			}
		}

		messages := lo.Map(task.Messages, func(m *dto.UnifiedMessage, _ int) *dbmodel.Message {
			model := ""
			tokenCount := 0
			if lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role) {
				model = task.Model
				tokenCount = task.OutputTokens
			}
			return &dbmodel.Message{
				Model:      model,
				Message:    m,
				CheckSum:   util.ComputeMessageChecksum(m, toolSchemas),
				TokenCount: tokenCount,
			}
		})

		tools := lo.Map(task.Tools, func(t *dto.UnifiedTool, _ int) *dbmodel.Tool {
			return &dbmodel.Tool{
				Tool:     t,
				CheckSum: util.ComputeToolChecksum(t),
			}
		})

		err := db.Transaction(func(tx *gorm.DB) error {
			messageIDs, err := pm.deduplicateAndStoreMessages(tx, messageDAO, messages)
			if err != nil {
				logger.Error("[StorePool] Failed to store messages", zap.Error(err))
				return err
			}

			toolIDs, err := pm.deduplicateAndStoreTools(tx, toolDAO, tools)
			if err != nil {
				logger.Error("[StorePool] Failed to store tools", zap.Error(err))
				return err
			}

			session := &dbmodel.Session{
				APIKeyName: task.APIKeyName,
				MessageIDs: messageIDs,
				ToolIDs:    toolIDs,
				Client:     task.Client,
				Metadata:   task.Metadata,
			}
			if err := sessionDAO.Create(tx, session); err != nil {
				logger.Error("[StorePool] Failed to create session", zap.Error(err))
				return err
			}
			return nil
		})
		if err != nil {
			logger.Error("[StorePool] Transaction failed", zap.Error(err))
			return
		}
		logger.Info("[StorePool] Messages stored successfully")
	})
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/infrastructure/pool/store_pool.go
git commit -m "feat(pool): add store_pool.go for message storage tasks"
```

---

## Task 4: 创建 Agent 池

**Files:**
- Create: `internal/infrastructure/pool/agent_pool.go`

- [ ] **Step 1: 创建 agent_pool.go**

```go
package pool

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/agent"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// submitSummarizeTask 提交 Session 总结任务到 Agent 池
//
//	@param pm *PoolManager
//	@param task *dto.SummarizeTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) submitSummarizeTask(task *dto.SummarizeTask) error {
	log := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)
	sessionDAO := dao.GetSessionDAO()

	return pm.agentPool.Go(func() {
		summary, err := agent.GetSummarizer().SummarizeWithRetry(task.Ctx, task.Content, constant.SummarizeMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate summary", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		if summary == "" {
			log.Error("[AgentPool] Summary is empty", zap.Uint("sessionID", task.SessionID))
			return
		}

		err = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]interface{}{
			"summary": summary,
		})
		if err != nil {
			log.Error("[AgentPool] Failed to update session summary", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		log.Info("[AgentPool] Session summarized successfully", zap.Uint("sessionID", task.SessionID), zap.String("summary", summary))
	})
}

// submitScoreTask 提交 Session 评分任务到 Agent 池
//
//	@param pm *PoolManager
//	@param task *dto.ScoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) submitScoreTask(task *dto.ScoreTask) error {
	log := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)
	sessionDAO := dao.GetSessionDAO()

	return pm.agentPool.Go(func() {
		result, err := agent.GetScorer().ScoreWithRetry(task.Ctx, task.Content, constant.ScoreMaxRetries)
		if err != nil {
			log.Error("[AgentPool] Failed to generate score", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		if result == nil {
			log.Info("[AgentPool] Skipping score for empty content", zap.Uint("sessionID", task.SessionID))
			return
		}

		err = sessionDAO.Update(db, &dbmodel.Session{ID: task.SessionID}, map[string]interface{}{
			"coherence_score": result.Coherence,
			"depth_score":     result.Depth,
			"value_score":     result.Value,
			"total_score":     result.Total(),
			"score_version":   constant.ScoreVersion,
			"scored_at":       lo.ToPtr(time.Now()),
		})
		if err != nil {
			log.Error("[AgentPool] Failed to update session score", zap.Uint("sessionID", task.SessionID), zap.Error(err))
			return
		}

		log.Info("[AgentPool] Session scored successfully",
			zap.Uint("sessionID", task.SessionID),
			zap.Float64("coherence", float64(result.Coherence)),
			zap.Float64("depth", float64(result.Depth)),
			zap.Float64("value", float64(result.Value)),
			zap.Float64("total", result.Total()))
	})
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/infrastructure/pool/agent_pool.go
git commit -m "feat(pool): add agent_pool.go for summarization and scoring tasks"
```

---

## Task 5: 重构 PoolManager（pool.go）

**Files:**
- Rewrite: `internal/infrastructure/pool/pool.go`

- [ ] **Step 1: 重写 pool.go**

完整重写为：

```go
// Package pool 协程池管理器
//
//	author centonhuang
//	update 2026-04-05 10:00:00
package pool

import (
	"github.com/alitto/pond/v2"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"
)

// PoolManager 全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolManager struct {
	storePool pond.Pool
	agentPool pond.Pool
}

var poolManager *PoolManager

// InitPoolManager 初始化全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func InitPoolManager() {
	poolManager = &PoolManager{
		storePool: pond.NewPool(config.Pool.Store.Workers, pond.WithQueueSize(config.Pool.Store.QueueSize)),
		agentPool: pond.NewPool(config.Pool.Agent.Workers, pond.WithQueueSize(config.Pool.Agent.QueueSize)),
	}
}

// GetPoolManager 获取全局协程池管理器实例
//
//	@return *PoolManager
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func GetPoolManager() *PoolManager {
	return poolManager
}

// StopPoolManager 停止全局协程池管理器
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func StopPoolManager() {
	if poolManager != nil {
		poolManager.Stop()
	}
}

// SubmitMessageStoreTask 提交消息存储任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.MessageStoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	return pm.submitMessageStoreTask(task)
}

// SubmitSummarizeTask 提交 Session 总结任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.SummarizeTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) SubmitSummarizeTask(task *dto.SummarizeTask) error {
	return pm.submitSummarizeTask(task)
}

// SubmitScoreTask 提交 Session 评分任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.ScoreTask
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) SubmitScoreTask(task *dto.ScoreTask) error {
	return pm.submitScoreTask(task)
}

// deduplicateAndStoreMessages 批量去重并存储消息
//
//	使用 IN 查询一次性获取已存在的消息，批量创建不存在的消息，保持原始顺序返回 ID 列表
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param messageDAO *dao.MessageDAO
//	@param messages []*dbmodel.Message
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) deduplicateAndStoreMessages(tx *gorm.DB, messageDAO *dao.MessageDAO, messages []*dbmodel.Message) ([]uint, error) {
	if len(messages) == 0 {
		return []uint{}, nil
	}

	checksums := make([]string, len(messages))
	for i, m := range messages {
		checksums[i] = m.CheckSum
	}

	existingMessages, err := messageDAO.BatchGetByField(tx, "check_sum", checksums, []string{"id", "check_sum"})
	if err != nil {
		return nil, err
	}

	existingMap := make(map[string]uint, len(existingMessages))
	for _, m := range existingMessages {
		existingMap[m.CheckSum] = m.ID
	}

	newMessages := make([]*dbmodel.Message, 0)
	for _, m := range messages {
		if _, exists := existingMap[m.CheckSum]; !exists {
			newMessages = append(newMessages, m)
			existingMap[m.CheckSum] = m.ID
		}
	}

	if len(newMessages) > 0 {
		if err := messageDAO.BatchCreate(tx, newMessages); err != nil {
			return nil, err
		}
	}

	messageIDs := make([]uint, len(messages))
	for i, m := range messages {
		messageIDs[i] = existingMap[m.CheckSum]
	}

	return messageIDs, nil
}

// deduplicateAndStoreTools 批量去重并存储工具
//
//	使用 IN 查询一次性获取已存在的工具，批量创建不存在的工具，保持原始顺序返回 ID 列表
//	@receiver pm *PoolManager
//	@param tx *gorm.DB
//	@param toolDAO *dao.ToolDAO
//	@param tools []*dbmodel.Tool
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (pm *PoolManager) deduplicateAndStoreTools(tx *gorm.DB, toolDAO *dao.ToolDAO, tools []*dbmodel.Tool) ([]uint, error) {
	if len(tools) == 0 {
		return []uint{}, nil
	}

	checksums := make([]string, len(tools))
	for i, t := range tools {
		checksums[i] = t.CheckSum
	}

	existingTools, err := toolDAO.BatchGetByField(tx, "check_sum", checksums, []string{"id", "check_sum"})
	if err != nil {
		return nil, err
	}

	existingMap := make(map[string]uint, len(existingTools))
	for _, t := range existingTools {
		existingMap[t.CheckSum] = t.ID
	}

	newTools := make([]*dbmodel.Tool, 0)
	for _, t := range tools {
		if _, exists := existingMap[t.CheckSum]; !exists {
			newTools = append(newTools, t)
			existingMap[t.CheckSum] = t.ID
		}
	}

	if len(newTools) > 0 {
		if err := toolDAO.BatchCreate(tx, newTools); err != nil {
			return nil, err
		}
	}

	toolIDs := make([]uint, len(tools))
	for i, t := range tools {
		toolIDs[i] = existingMap[t.CheckSum]
	}

	return toolIDs, nil
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
```

注意：`lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role)` 使用 `enum.Role` 类型比较，与原代码一致。

- [ ] **Step 2: 验证构建**

```bash
go build ./...
```

如有错误，根据错误信息修复。

- [ ] **Step 3: 提交**

```bash
git add internal/infrastructure/pool/pool.go
git commit -m "refactor(pool): simplify PoolManager as pure dispatcher with store/agent pools"
```

---

## Task 6: 更新配置文件

**Files:**
- Modify: `config/config.yaml.template`
- Modify: `config/config.yaml`

- [ ] **Step 1: 更新 config.yaml.template**

找到 pool 相关配置，替换为：

```yaml
pool:
  store:
    workers: 50
    queue_size: 1000
  agent:
    workers: 10
    queue_size: 100
```

- [ ] **Step 2: 更新 config.yaml（如存在）**

同样替换 pool 配置段。

- [ ] **Step 3: 提交**

```bash
git add config/config.yaml.template config/config.yaml
git commit -m "refactor(config): split pool config into store and agent groups"
```

---

## Task 7: 运行全量测试

- [ ] **Step 1: 运行测试**

```bash
make test
```

或

```bash
go test -count=1 ./...
```

- [ ] **Step 2: 如有错误，修复并重新测试**

- [ ] **Step 3: 提交所有更改**

```bash
git add -A
git commit -m "refactor(pool): implement store/agent pool separation"
```

---

## 实施检查清单

- [ ] Task 1: Config 结构体重构完成
- [ ] Task 2: Agent 单例化完成
- [ ] Task 3: Store 池创建完成
- [ ] Task 4: Agent 池创建完成
- [ ] Task 5: PoolManager 重构完成，构建通过
- [ ] Task 6: 配置文件更新完成
- [ ] Task 7: 全量测试通过

---

## 风险与注意事项

1. **配置兼容性**：如果生产环境 config.yaml 中有旧的 `pool.workers` 和 `pool.queue.size` 配置，新的分组配置不会自动读取。需要确认配置迁移策略。

2. **日志前缀**：Store 池用 `[StorePool]`，Agent 池用 `[AgentPool]`，与原 `[PoolManager]` 区分。
