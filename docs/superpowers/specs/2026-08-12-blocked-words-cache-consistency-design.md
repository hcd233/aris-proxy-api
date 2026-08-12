# 敏感词多副本缓存一致性重设计文档

> 日期：2026-08-12
> 状态：待评审
> 分支：待建（feature/blocked-words-cache-consistency-2026-08-12）

## 背景与问题（来自生产 RCA）

生产 `aris-proxy-api` 为 **2 副本** Deployment。敏感词匹配走进程内 AC 自动机
（`internal/application/blocked/service.go` 的 `BlockedService`），增删改后仅重建
**收到请求的那个 pod** 的内存（`rebuildNotify = svc.Rebuild`，本进程内全量重载），
无任何跨 pod 通知机制。

实测证据（GORM SQL 日志，2026-08-12）：
- 08:43:53 pod 156 `UPDATE action='deny' WHERE id=7` + Rebuild（内存：你好=deny）
- 08:44:01 pod 157 `UPDATE deleted_at=... WHERE id IN (7)` + Rebuild（内存：已清除）
- 08:44:33 请求（traceId `38a970dd-67b4-47e6-ae41-1908d5035d87`）被路由到 **pod 156**，
  其内存仍含已删除的 id=7 → 命中敏感词返回 403（删除后 32 秒仍拦截）

根因：**多副本下进程内内存 matcher 各自加载、各自重建，变更不广播 → 数据分叉**。

## 目标

1. 敏感词增删改后，所有副本在**秒级内**收敛到 DB 最新状态（一致延迟 ≤ 轮询间隔，目标 2s）。
2. 不改变敏感词管理能力：PG `blocked_words` 表保持真源（分页列表、hit_count 统计、软删审计不变）。
3. 并发增删敏感词不产生永久分叉（任意顺序/丢失通知最终收敛）。
4. 复刻项目既有基础设施（Redis pub/sub、`BlockedHitCache` 等 cache 封装），不引入新依赖。

## 非目标

- 不迁移敏感词数据本身到 Redis（AC 自动机必须驻内存，见"关键事实"）。
- 不改管理后台 API 语义与前端交互。
- 不改变命中检查的性能特征（AC 自动机 O(n) 不变）。

## 关键事实：为什么数据不能整体迁到 Redis

`ACmatcher`（`internal/application/blocked/matcher.go`）是 Aho-Corasick 自动机，
为单次请求 O(n) 扫描构建的内存 trie。若每次请求实时查 Redis 逐词匹配，性能退化为
O(词数 × 文本长度)，LLM 代理高并发场景不可接受。因此：

- **数据真源**：PG `blocked_words`（唯一，含审计/分页/hit_count）。
- **匹配载体**：各 pod 进程内 AC matcher（从 PG 全量重建，幂等）。
- **Redis 角色**：仅做"变更信号"（pub/sub 频道 + 版本计数器），**不存敏感词数据**。

这样规避"PG/Redis 双写数据分叉"，数据真源永远只有一份。

## 方案架构

### 三通道收敛（分工）

```
写入路径（handler，PG 事务成功之后）：
  ① Publish("blocked:changed")                          → 即时（正常路径，毫秒级）
  ② INCR blocked:version                                → 可靠兜底（原子计数，不丢变更）
  ③ 保留本进程同步 rebuildNotify（立即生效，现状）        → 本 pod 零延迟（签名兼容，见下）

各 pod 读取侧（每副本各自运行）：
  A. pub/sub 订阅：收到 blocked:changed → 立即 Rebuild（毫秒级）
  B. 版本轮询 goroutine：每 2s 读 blocked:version，> 本地 lastVersion → Rebuild（兜底）
  C. 低频无条件 Rebuild（并入 B 的 goroutine，距上次 Rebuild ≥5 分钟则无条件重建）→ 极端兜底

其中 A/B/C 全部复用同一个幂等 Rebuild()（SELECT PG 全量 → 整体替换 matcher）。

> **B/C 为什么不用 cron 框架**：现有 cron 全部 `cron.New()` 无 `cron.WithSeconds()`，
> 标准 5 字段只能到分钟级；且 cron 每次执行走审计链路（`CronCallAuditStore`），2 秒级会刷爆审计表。
> 故 B/C 用独立 goroutine（`time.Ticker`），由 `BlockedService.StartSync(ctx)` 统一启动，
> 在 lifecycle OnStart 调用（每 pod 一次）。
```

### 通道职责与故障矩阵

| 故障场景 | A pub/sub | B 版本轮询 | C 低频重建 | 收敛时间 |
|---------|-----------|-----------|-----------|---------|
| 正常 | ✅ 毫秒级 | 不触发 | 不触发 | ~0ms |
| pub/sub 断连/丢消息 | ❌ | ✅ 2s 内补上 | - | ≤2s |
| pub/sub + INCR 都失败 | ❌ | ❌ | ✅ ≤5min | ≤5min |
| pod 重启 | 启动即订阅 | lastVersion=0 首轮必触发 | - | ≤2s |
| 并发增删（A 删 X / B 加 Y） | 全量重建读到最新全集 | 同左 | 同左 | ≤2s |

### 为什么并发/乱序/丢失都安全

核心性质：**每次重建都是"读 PG 全量、整体替换"的幂等操作**。

- 并发增删：PG 层各自事务提交、最终一致；无论 A/B/C 哪条通道先触发 Rebuild，
  读到的都是 PG 当时的最新全集（含 X 删、Y 加），一步收敛。
- 通知乱序：通道只携带"有变更"信号，不携带增量操作 → 顺序无影响。
- Torn rebuild：`Rebuild()` 持写锁（`mu.Lock`），`Check()` 持读锁（`RLock`），
  重建期间新写入导致 version 再变 → 下一轮再重建，最终必收敛（现状锁语义已安全）。
- Redis INCR 失败：best-effort，不阻塞写；由通道 C 兜底。

## 组件设计

### 1. 版本计数（Redis）

- key：`blocked:version`（新增常量 `constant.BlockedVersionKey`，对齐 `BlockedHitKeyPrefix` 命名）。
- 语义：**单调递增的变更代数**，每发生一次成功写（create/update/delete）`INCR` 一次。
- 各 pod 在内存保存 `lastSeenVersion`（随 `Rebuild()` 成功更新为读取值）。

### 2. pub/sub 频道

- channel：`blocked:changed`（新增常量 `constant.BlockedChangeChannel`）。
- 消息体：空 payload 或 `{"pod": hostname}`（仅信号，无增量，保持幂等性）。
- 订阅者：每 pod 启动时 `Subscribe`（对齐 `CronManager.StartListener` 模式）。

### 3. 同步启动（`BlockedService.StartSync`，对齐 CronManager 先例）

由 `BlockedService.StartSync(ctx)` 统一启动订阅 + 轮询，lifecycle OnStart 调用（每 pod 一次）：

```go
func (s *BlockedService) StartSync(ctx context.Context) {
    s.pubSub = s.cache.Subscribe(ctx, constant.BlockedChangeChannel)
    go s.syncLoop(ctx)          // 每 2s tick：版本轮询 + 低频兜底
    go func() {                 // pub/sub 即时通道
        for range s.pubSub.Channel() { s.Rebuild(ctx) }
    }()
    // 订阅建立后立即对比一次 version，消除"订阅前已发生的变更"竞态
    s.checkVersion(ctx)
}

func (s *BlockedService) syncLoop(ctx context.Context) {
    t := time.NewTicker(2 * time.Second)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-t.C:
            s.checkVersion(ctx)
            // 低频兜底：距上次成功 Rebuild ≥ 5min 则无条件重建（失败不更新游标 → 每 2s 重试）
            if time.Since(s.lastRebuildAt) >= 5*time.Minute {
                _ = s.Rebuild(ctx) //nolint:errcheck // 失败不更新 lastRebuildAt，由下轮 tick 重试
            }
        }
    }
}

func (s *BlockedService) checkVersion(ctx context.Context) {
    v, err := s.cache.Get(ctx, constant.BlockedVersionKey).Int64()
    if err != nil {
        return // 读不到版本号时由低频兜底收敛；Redis 故障时本 pod 保持现状
    }
    if v != s.lastSeenVersion {
        if err := s.Rebuild(ctx); err == nil {
            s.lastSeenVersion = v
        }
    }
}
```

要点：
- 每 pod 各自维护 `lastSeenVersion` / `lastRebuildAt`（内存字段），互不共享；
- 分布式锁**不需要**（每 pod 重建各自内存，无共享写；区别于 hit_sync 的 DB 写场景）；
- `Rebuild()` 需区分成败（见下方"Rebuild 失败重试"）；首次启动 `lastRebuildAt` 为零值 → 首轮即触发重建（覆盖启动竞态）；
- `redis.PubSub.Channel()` 断线时由 go-redis 自动重连，断窗变更由版本轮询（B）兜底；
- OnStop 时 `s.pubSub.Close()`。

**Rebuild 失败重试语义**：现状 `Rebuild()` 返回 void，DB 查询失败时 `rebuild(空集)` 清空 matcher 且调用方无法感知。
本设计将 `Rebuild()` 改为返回 `error`，且**失败时保持原 matcher 不变**（不清空）：
- `checkVersion` 仅在 `Rebuild() == nil` 时更新 `lastSeenVersion`；
- 低频兜底仅在成功时更新 `lastRebuildAt`；
- 失败时两个游标都不动 → 下一次 tick（2s 后）B/C 都会**再次重试**，直到成功；
- 失败不清空的原因：清空会让重试窗口内所有敏感词失效（请求全部漏检），比"短暂保持旧规则"更危险。

**pub/sub 突发消息**：`pubSub.Channel()` 单 goroutine 串行投递，收到即 Rebuild（内部持写锁），
管理操作低频，突发时逐条串行重建可接受；如未来需要可加"dirty 标记合并"，当前 YAGNI 不做。

### 4. handler 变更（create/update/delete）

```go
// 现状（仅本进程重建）
h.repo.Create(...) / Update / DeleteBatch
h.rebuildNotify(ctx)   // = svc.Rebuild（本进程）

// 变更后：PG 写成功后，三通道
h.repo.Create(...) / Update / DeleteBatch
h.rebuildNotify(ctx)                            // 保留：本 pod 立即生效（现状）
svc.NotifyChanged(ctx)                          // 新增：Publish + INCR version（best-effort）
```

`BlockedService.NotifyChanged(ctx)`：
```go
func (s *BlockedService) NotifyChanged(ctx context.Context) {
    // best-effort，错误仅记日志，不阻塞写响应
    _ = s.cache.Publish(ctx, constant.BlockedChangeChannel, payload).Err()
    _ = s.cache.Incr(ctx, constant.BlockedVersionKey).Err()
}
```

**rebuildNotify 签名兼容**：现有 `rebuildNotify func(ctx)`（handler 构造参数）直接绑定 `svc.Rebuild`。
`Rebuild` 改为返回 `error` 后，3 处装配点（`bootstrap/modules/application.go` 的
NewCreate/Update/DeleteBlockedHandler）改为闭包适配：
`blockedcommand.NewDeleteBlockedHandler(repo, func(ctx context.Context) { _ = svc.Rebuild(ctx) })`
（本进程立即生效路径忽略错误，失败由 A/B/C 通道重试收敛，语义不变）。
lifecycle OnStart 的 `params.BlockedService.Rebuild(ctx)` 同样改为忽略返回值的闭包或 `_ =`。

### 5. DI 装配与启动

- `BlockedService` 增加 `cache *redis.Client` 依赖（`NewBlockedService` 签名扩展，
  现有装配点 `bootstrap/modules/application.go:NewBlockedService` 同步修改）。
- `BlockedService.StartSync(ctx)` 在 lifecycle OnStart 调用（`internal/bootstrap/lifecycle.go`
  已有 `params.BlockedService.Rebuild(ctx)` 的启动 Hook，同处追加 StartSync），每 pod 启动一次；
  OnStop 关闭 pubSub（`s.pubSub.Close()`）。
- 不新增 cron 条目（B/C 走 goroutine）；`blocked_hit_sync`（每 5 分钟写 DB 命中计数）保持现状不变。

## 错误处理

| 场景 | 行为 |
|------|------|
| Publish 失败 | 记 WARN，写响应不阻塞；B/C 通道兜底 |
| INCR 失败 | 同上 |
| Rebuild 中 DB 查询失败 | `Rebuild()` 返回 error 且保持原 matcher；游标不更新，下一次 tick（2s）自动重试；日志 ERROR |
| pub/sub 断线 | go-redis 自动重连；断窗变更由 B 兜底 |
| 订阅启动失败 | 记 ERROR，仅 A 通道不可用，B/C 继续收敛 |

## 测试

1. **单元测试**（`internal/application/blocked/`）：
   - `NotifyChanged`：Publish/INCR 调用断言（mock redis / miniredis）；
   - `Rebuild` 幂等性：连续两次 Rebuild 结果一致；
   - `Rebuild` 失败：repo 返回 error → `Rebuild()` 返回 error 且 matcher 保持原状（不清空）→ 断言游标不更新；
   - 并发 Check + Rebuild 无数据竞争（`go test -race`）。
2. **集成测试（StartSync/syncLoop）**：
   - version 变化 → 触发 Rebuild；不变 → 不触发（用可控 ticker 间隔或单步调用 checkVersion）；
   - 低频兜底：`lastRebuildAt` 置旧 → 下一次 tick 无条件 Rebuild；
   - pub/sub 消息到达 → 立即 Rebuild。
3. **回归**：`go test ./internal/application/blocked/... ./internal/cron/... ./internal/application/llmproxy/...`
4. **手工 E2E（生产或准生产）**：
   - 在 pod A 删除一个 deny 敏感词 → 反复请求直到被路由到 pod B → 应 ≤2s 内不再命中；
   - 并发在 Web 后台新增/删除/改 action → 两个 pod 的行为一致；
   - 用 RCA 的同类场景（删除后立即 403）作为回归用例。

## 部署与回滚

- 滚动发布（现有 `docker-publish.yml` 流程），2 副本逐个替换；
- 旧 pod（无订阅）与新 pod（有订阅）混跑期间：写操作 Publish 到频道无订阅者 → 消息丢弃，
  但新 pod 有 B 通道轮询兜底 → 收敛不受影响；旧 pod 行为保持现状（不更差）；
- 回滚：镜像回退即恢复现状（无数据迁移、无 schema 变更、无新依赖），零风险。

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | 数据真源 | **PG 保留**，Redis 不存词数据 | 管理后台依赖 PG（分页/hit_count/软删审计）；规避双写分叉 |
| D2 | 收敛机制 | **pub/sub 即时 + 版本轮询兜底 + 低频重建兜底**（三通道） | pub/sub 快但不持久，轮询可靠但慢，低频重建兜底极端情况；三者共用幂等全量 Rebuild，组合后"快且必达" |
| D3 | 轮询载体 | **独立 goroutine（time.Ticker，2s）**，不用 cron 框架 | 现有 cron 无 WithSeconds（仅分钟级）+ 每次执行走审计链路，2s 级会刷爆审计表 |
| D4 | 分布式锁 | **不需要** | 每 pod 重建各自内存，无共享写；区别于 hit_sync 的 DB 写场景 |
| D5 | 订阅生命周期 | **BlockedService.StartSync 统一启动订阅+轮询**（lifecycle OnStart，每 pod 一次） | 单点启动；pub/sub 断线由 go-redis 自动重连，断窗由轮询兜底 |
| D6 | handler 现状 | **保留本进程同步 rebuildNotify** + 新增 NotifyChanged | 本 pod 零延迟不回归；不破坏现有单测 |
| D7 | 低频兜底频率 | 5 分钟（并入轮询 goroutine） | 远大于任何故障窗口（2s 轮询 + 重连），成本可忽略 |

## 影响范围

- `internal/application/blocked/service.go`（+cache 依赖、+NotifyChanged、+StartSync、+lastSeenVersion/lastRebuildAt、+pubSub 字段与 Close；`Rebuild` 改为返回 error）
- `internal/application/blocked/command/{create,update,delete}_blocked.go`（+NotifyChanged 调用；构造签名不变，装配侧适配闭包）
- `internal/bootstrap/modules/application.go`（DI 装配：注入 cache + rebuildNotify 闭包适配）
- `internal/bootstrap/lifecycle.go`（OnStart 追加 StartSync；OnStop 关闭 pubSub；Rebuild 忽略返回值）
- `internal/common/constant/string.go` 或 cache 常量（BlockedVersionKey / BlockedChangeChannel）
- 不新增 cron 条目；`internal/cron/blocked_hit_sync.go` 不动
