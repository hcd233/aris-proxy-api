# Pool Manager 重构设计

## 1. 背景与目标

**现状问题：**
- `Manager` 结构体直接持有多个 Pond Pool，全部使用相同配置
- `Manager` 绑定了 `messageDAO`、`sessionDAO`、`toolDAO`、`summarizer`、`scorer`，职责过重
- 新增任务类型需修改 `Manager` 结构体，扩展不便

**目标：**
- 按处理对象分组，2 组独立池：Store 池（消息/工具存储）和 Agent 池（AI 能力：总结、评分）
- 配置按组分：每组独立配置 `workers` 和 `queue_size`
- `Manager` 瘦身为纯分发器，不直接持有业务依赖
- 支持未来新任务按对象归入现有组

## 2. 架构设计

### 2.1 分组策略

| 组 | 任务类型 | 职责 | 配置建议 |
|----|---------|------|---------|
| store | MessageStore | 消息/工具/会话的 DB 存储，高频 | workers=50, queue=1000 |
| agent | Summarize, Score | AI 能力（LLM 调用 + DB 更新），低频 | workers=10, queue=100 |

### 2.2 结构设计

```
internal/infrastructure/pool/
├── pool_manager.go   # 分发器，按任务类型路由到对应池
├── store_pool.go     # Store 池初始化和任务处理
└── agent_pool.go     # Agent 池初始化和任务处理
```

**PoolManager**（精简为分发器）：
```go
type PoolManager struct {
    storePool pond.Pool
    agentPool pond.Pool
}
```

**职责：**
- 持有多池引用
- `SubmitMessageStoreTask` → storePool
- `SubmitSummarizeTask` → agentPool
- `SubmitScoreTask` → agentPool
- `Stop()` 停止所有池

### 2.3 Agent 单例化

`Summarizer` 和 `Scorer` 内部持有 LLM 客户端（可能有连接池），不适合在每次任务执行时创建新实例。需要在 agent 包补充单例访问方式：

```go
// internal/agent/summarizer.go
var summarizer *Summarizer

func GetSummarizer() *Summarizer {
    if summarizer == nil {
        summarizer = lo.Must1(NewSummarizer())
    }
    return summarizer
}
```

任务闭包通过 `agent.GetSummarizer()` 获取，而非自己创建。

### 2.4 配置设计

```yaml
# config.yaml
pool:
  store:
    workers: 50
    queue_size: 1000
  agent:
    workers: 10
    queue_size: 100
```

Config 结构体新增：
```go
type PoolConfig struct {
    Store PoolGroupConfig
    Agent PoolGroupConfig
}

type PoolGroupConfig struct {
    Workers   int
    QueueSize int
}
```

## 3. 任务执行闭包设计

任务处理逻辑从 `Manager` 移到各池的提交方法中，闭包捕获必要依赖。

**示例 - MessageStore：**
```go
func (pm *PoolManager) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
    logger := logger.WithCtx(task.Ctx)
    db := database.GetDBInstance(task.Ctx)
    messageDAO := dao.GetMessageDAO()
    sessionDAO := dao.GetSessionDAO()
    toolDAO := dao.GetToolDAO()

    return pm.storePool.Go(func() {
        // ... 原有逻辑，引用局部变量
    })
}
```

**示例 - Summarize：**
```go
func (pm *PoolManager) SubmitSummarizeTask(task *dto.SummarizeTask) error {
    log := logger.WithCtx(task.Ctx)
    db := database.GetDBInstance(task.Ctx)
    sessionDAO := dao.GetSessionDAO()

    return pm.agentPool.Go(func() {
        summary, err := agent.GetSummarizer().SummarizeWithRetry(...)
        // ... 原有逻辑
    })
}
```

## 4. 行为兼容性

### 4.1 公开接口不变

现有 `SubmitXxxTask` 方法签名保持不变：
- `SubmitMessageStoreTask(*dto.MessageStoreTask) error`
- `SubmitSummarizeTask(*dto.SummarizeTask) error`
- `SubmitScoreTask(*dto.ScoreTask) error`

### 4.2 初始化流程

```
InitPoolManager()
  → 读取配置 PoolConfig{Store, Agent}
  → 创建 storePool = pond.NewPool(cfg.Store.Workers, pond.WithQueueSize(cfg.Store.QueueSize))
  → 创建 agentPool = pond.NewPool(cfg.Agent.Workers, pond.WithQueueSize(cfg.Agent.QueueSize))
  → 赋值给 Manager
```

### 4.3 关闭流程

```go
func (pm *PoolManager) Stop() {
    if pm.storePool != nil {
        pm.storePool.Stop()
    }
    if pm.agentPool != nil {
        pm.agentPool.Stop()
    }
}
```

## 5. 文件变更清单

| 操作 | 文件 | 说明 |
|-----|------|-----|
| 重写 | `internal/infrastructure/pool/pool.go` | 精简为 PoolManager，移除业务依赖 |
| 新增 | `internal/infrastructure/pool/store_pool.go` | Store 池任务处理（MessageStore） |
| 新增 | `internal/infrastructure/pool/agent_pool.go` | Agent 池任务处理（Summarize、Score） |
| 修改 | `internal/agent/summarizer.go` | 新增 `GetSummarizer()` 单例访问 |
| 修改 | `internal/agent/scorer.go` | 新增 `GetScorer()` 单例访问 |
| 修改 | `internal/config/config.go` | 新增 PoolConfig 结构体和配置加载 |
| 修改 | `config/config.yaml.template` | 新增 store/agent 分组配置 |
| 修改 | `config/config.yaml` | 同上 |

## 6. 测试策略

测试文件放在 `test/pool_manager/` 目录。

### 6.1 单元测试

- **PoolManager 分发测试**：验证任务路由到正确的池
- **配置加载测试**：验证配置正确解析为 PoolConfig

### 6.2 集成测试

- **任务执行测试**：提交各类型任务，验证正确执行
- **池隔离测试**：验证存储任务不受 AI 任务影响

## 7. 风险与缓解

| 风险 | 缓解 |
|-----|-----|
| 闭包捕获变量导致内存泄漏 | 闭包只捕获必要依赖（DAO、DB），不捕获大对象 |
| AI 任务阻塞存储任务 | 独立池物理隔离 |
| 配置迁移成本 | 向后兼容，默认值覆盖旧配置 |
