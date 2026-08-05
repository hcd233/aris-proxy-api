# Review: PR #126 — Cron 手动触发 (feature/cron-manual-trigger-2026-08-05)

> 审查方式：只读。在 worktree 中 `git diff master...HEAD`（8 commits），逐文件核对实现与需求，
> 并对高风险的 `TriggerWithLock` 用 `go test -overlay` 注入临时测试做**实证验证**（未修改仓库，`git status` 干净）。
> 构建/静态检查：`go build ./...` 通过、`go vet` 无问题、`go test ./test/unit/cron/` 27 个用例通过。

## Review

### Correct（已正确实现，附证据）

1. **审计链路 trigger_source 贯穿完整**：常量（`internal/common/constant/cron.go` 新增 `CronTriggerSourceScheduled/Manual`）
   → model（`internal/infrastructure/database/model/cron.go:44` gorm 列 `trigger_source;not null;default:scheduled`，`base.go:32` 已注册 AutoMigrate，存量行自动补 `scheduled`）
   → 仓储 Save/List（`internal/infrastructure/repository/cron_audit_repository.go`）+ SQL 字段列表 `constant.CronCallAuditRepoFields` 已加 `FieldTriggerSource`（`internal/common/constant/sql.go:279`）
   → port view（`internal/application/cronaudit/port/handler.go`）→ DTO（`internal/dto/cron.go` `TriggerSource`）→ handler 映射 → 前端 `web/src/lib/types.ts` `triggerSource` → audit 页 Badge → i18n（zh/en/ja 三语 key 齐全）。
2. **拿锁失败零审计**：`TriggerWithLock` 在 `locker.Lock` 失败/未拿到时直接返回 false，不创建 goroutine、不调 `saveCronCallAudit`（`internal/cron/lock_runner.go:204-210`），符合"锁占用 → 423 + 无记录"的需求。
3. **wrapCronFunc source 参数改造对 scheduled 路径无行为回归**：启用检查加了 `source == constant.CronTriggerSourceScheduled` 守卫（`lock_runner.go:146`），scheduled 行为与原来完全一致；panic 处理与审计均带 source。4 个任务实现（session_dedup / soft_delete_purge / think_extract / blocked_hit_sync）全部更新为传入 `CronTriggerSourceScheduled`，`Trigger()` 方法统一走 `TriggerWithLock` 并复用同一个 `lockKey`/`locker`。
4. **HTTP 层**：路由 `POST /api/v1/cron/trigger`（`internal/router/cron.go:40-51`，挂在 `/api/v1/cron` group 下，与前端 `api-client.ts:715` 一致），admin 权限中间件与 list/update 一致；错误映射正确 —— `ErrDataNotExists` → biz 10003 → HTTP 404、`ErrResourceLocked` → biz 10009 → HTTP 423（`internal/common/model/error.go:65-84`），handler 经 `NewHumaBizError` 正确解包 `InternalError`（`ierr.ToBizError`，`internal/api/util/error.go:57`）。DI 接线完整（bootstrap/modules/application.go + handler.go）。
5. **disabled 任务可手动触发**：`TriggerWithLock` 不做 DB enabled 检查（设计如此），且 `disableLocked` 只停调度器不删 entry，`CronManager.Trigger` 能找到实例。✅
6. **Trigger 与 scheduled 运行互斥**：二者使用同一 Redis 锁 key（`c.lockKey`），锁层面互斥，不依赖 pub/sub 广播。设计正确（除下述 Critical 的续期失效外）。

### 按严重度分组的问题

#### 🔴 Critical

**C1. `TriggerWithLock` 的 `defer cancel()` 使后台执行拿到已取消的 context，手动触发实际必然失败，且锁永不续期**
- 位置：`internal/cron/lock_runner.go:198-199, 212-234`
- 问题：`childCtx, cancel := context.WithCancel(ctx)` + 外层函数 `defer cancel()`。`TriggerWithLock` 在 `go func(){...}()` 之后立即 `return true`，`cancel()` 随即执行 → `childCtx` 在 goroutine 真正开始跑 `fn` 之前就被取消。
  - 实证：用 `go test -overlay` 注入临时测试（fn 内检查 `ctx.Err()`），输出 `fn received CANCELED context (ctx.Err()=context canceled)`，稳定复现。
  - 影响 1（功能全坏）：4 个任务的 fn 全部把 ctx 传给 DB/Redis —— session_dedup `db.WithContext(ctx)`、soft_delete_purge `db.WithContext(ctx)`（`soft_delete_purge.go:117`）、think_extract `repo.FindThinkExtractCandidates(ctx,...)`/`UpdateMessageContent(ctx,...)`（`think_extract.go:118,140`）、blocked_hit_sync `hitCache.PopAll(ctx)`/`BatchIncrementHitCount(ctx,...)`（`blocked_hit_sync.go:75,83`）。已取消的 ctx 使查询立即返回 `context canceled`，手动触发每次都秒败并写一条 `failed` 审计，任务实际什么都没做。
  - 影响 2（并发保证失效）：`go renewLoop(childCtx, ...)`（`lock_runner.go:230`）首轮 select 即命中 `ctx.Done()` 退出，手动执行期间**锁从不续期**。任务运行超过默认 TTL（5 分钟，`constant.CronLockDefaultTTL`）后锁自然过期，定时调度或第二次手动触发可再次拿到锁 → 同一任务并发执行，破坏"单实例执行"保证与"正在执行 → 423"的保护窗口。
- 修复建议：把 `cancel()` 移到 goroutine 内（`go func(){ defer cancel(); go renewLoop(childCtx,...); metadata, fnErr = fn(childCtx) }()`），使 childCtx 生命周期 = fn 生命周期（fn 返回后再 cancel，同时停止 renewLoop）；外层函数不要 `defer cancel()`。审计写入用外层 `ctx`（当前实现已是，不受影响）。

#### 🟠 Major

**M1. 前端 `ResourceLocked` → "任务正在执行中" 是死代码，423 场景永远走不到**
- 位置：`web/src/app/(dashboard)/cron/page.tsx:133-143`
- 问题：后端对锁占用返回 HTTP 423 + body `{"error":{"code":10009,...}}`；`api-client.ts` 的 `request` 对非 2xx 抛 `ApiError`。`parseError(ApiError)`（`web/src/lib/api-errors.ts` 的 ApiError 分支）解析 body 后只检查**顶层** `bodyObj.code`（为 undefined，因为 code 嵌套在 `error` 下），`HTTP_FALLBACK` 又无 423 条目 → 返回 `code: 0`。于是 `parsed.code === BusinessErrorCode.ResourceLocked`（0 === 10009）永不成立，用户看到的是通用"触发失败"toast 而非 `cron.running` 文案。
- 修复建议：优先用 `ApiError` 已解析好的 `structured`（`api-client.ts` 构造函数已把嵌套 `{error:{code,message}}` 解析进 `this.structured`），即 `const parsed = (err as ApiError).structured ?? parseError(err)`；或修复 `parseError` 的 ApiError 分支改为检查 `bodyObj?.error?.code ?? bodyObj?.code`。

#### 🟡 Minor

**m1. `CronManager.Trigger` 在释放锁后调用 stale 实例的 `Trigger()`，与 Restart/Disable/Enable 存在竞态**
- 位置：`internal/cron/manager.go:293-301`
- 问题：`m.mu.RLock()` 读 `entry` 后立即 `RUnlock`，随后才 `entry.cron.Trigger()`。若其间 `restartLocked`/`enableLocked`/`disableLocked`（持 `m.mu.Lock()`）替换或停止了实例，手动触发会落在**已停止的旧实例**上执行 fn。当前危害有限（旧实例的 locker/lockKey 与 Redis 键不变，与新旧实例互斥，DAO 为共享单例），但模式上是竞态。
- 修复建议：让 `Trigger` 在持 `m.mu.RLock()` 期间调用 `entry.cron.Trigger()`（或读锁内取出后立即调用），避免触发到 stale 实例。

**m2. 单元测试未覆盖关键行为，且现有断言存在盲区**
- 位置：`test/unit/cron/lock_runner_test.go`（`TestTriggerWithLock_Acquired_RunsAsync`）、`test/unit/cron/cron_test.go`（`mockCron.Trigger` 无人使用）、`test/e2e/cron_trigger/cron_trigger_test.go`
- 问题：
  - 没有测试断言手动触发时 fn 收到的 ctx **未被取消**，也没有测试断言手动执行期间 `renewLoop` 真的续期了锁 —— C1 因此未被任何测试捕获。
  - 计划文档测试清单里写的 `CronManager.Trigger`（not found → ErrDataNotExists；locked → ErrResourceLocked）与 `wrapCronFunc` scheduled 回归测试**未落地**；`mockCron.Trigger`/`triggerResult` 加了字段但无用例引用。
  - e2e `TestE2E_CronManualTrigger_ProducesManualAudit` 只轮询"出现 triggerSource=manual 的记录"，不看 status —— 即使任务因 context canceled 秒败（记录 status=failed），e2e 依然通过，无法发现 C1。
- 建议：补 (a) TriggerWithLock 的 ctx 活性 + 续期断言；(b) CronManager.Trigger 两分支用例；(c) e2e 断言 manual 记录 status=success 且 metadata 非空。

**m3. 触发确认复用 `DeleteConfirmDialog`，且确认按钮点击后对话框立即关闭，loading 状态无效**
- 位置：`web/src/app/(dashboard)/cron/page.tsx:263-276` + `web/src/components/delete-confirm-dialog.tsx`
- 问题：Radix `AlertDialogAction` 点击即触发关闭（`onOpenChange(false)` → `setTriggeringJob(null)`），请求还在途时对话框已消失，`loading`/`loadingLabel`（"执行中"）几乎不可见；且弹窗标题带 AlertTriangle/destructive 样式，语义上不像"立即执行"。
- 建议：换中性确认框；或在 `triggering` 为 true 时忽略 `onOpenChange(false)`（`onOpenChange={(open) => { if (!open && !triggering) setTriggeringJob(null); }}`）。

**m4. `TestCronHandler_TriggerCronJob_Error` 只断言 `err != nil`，未验证 423/10009 映射**
- 位置：`test/unit/cron/cron_handler_test.go`（新增用例）
- 建议：用 `huma.StatusError` 断言 `GetStatus() == http.StatusLocked` 与错误体 code=10009。

#### ⚪ Nit

**n1. `handleTrigger` 中 `if (rsp.error)` 分支不可达**（`page.tsx:131-134`）：该端点业务错误均为非 2xx，`request` 直接抛错，`rsp.error` 恒为 undefined。无害但易误导，可删。
**n2. e2e `TestE2E_CronTrigger_NotFound` 只断言非 200**，可进一步断言 404/10003，非必须。
**n3. 文档与实现的"错误"一致**：spec（`docs/superpowers/specs/2026-08-05-cron-manual-trigger-design.md`）与 plan（`docs/superpowers/plans/2026-08-05-cron-manual-trigger.md:168-169`）都写明了同样的 `defer cancel()` 代码，且 spec 明确承诺"goroutine 内 renewLoop 续期" —— 这是文档与实现共同的设计缺陷，修复代码后需同步修订文档，并补一条工程经验（spec 中"拿锁失败零记录"等其余承诺与实现一致）。

## 总体结论

**Verdict: request changes**

核心功能（手动触发一次、审计区分来源、锁占用零记录、disabled 可触发、HTTP/DI/前端接线、审计链路）的架构与实现基本贴合 spec，审计链路与错误码映射扎实。但 `TriggerWithLock` 的 context 生命周期缺陷（C1）是**致命级**：手动触发拿到的执行 context 在函数返回瞬间即被取消，任务实际必然失败、锁永不续期，且现有测试（含 e2e）无法发现它 —— 已用 overlay 测试实证。前端 423 提示死代码（M1）也需一并修复。其余为低危竞态与测试覆盖问题。
