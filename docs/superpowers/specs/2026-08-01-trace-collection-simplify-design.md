# Trace 采集极简重构 设计文档（2026-08-01）

## 背景：数据驱动的问题清单

对生产库 `trace_events`（2026-07-28 ~ 07-31 采集，共 1008 条）与 `traces` 表分析后，确认四类冗余：

| # | 问题 | 数据证据 | 根因 |
|---|------|---------|------|
| 1 | **event_msg 97.6% 冗余** | 252 条中 token_count 110 / agent_message 71 / agent_reasoning 33 / user_message 12 等 | codex `event_msg`（增量事件流）与 `response_item`（完整消息）双源记录同一内容；token_count 等纯统计噪音无消费者 |
| 2 | **turn_context 无消费** | 12 条，每条 `{cwd, model, effort, summary, turn_id}` 快照 | model 与 `traces.model` 逐条一致、cwd 在 traces 表、turn_id 在 response_item 均有覆盖；前端 classifyEvent 无分支、服务端无消费 |
| 3 | **hook 事件双源重复/无价值** | 313 条中 PreToolUse 142 / PostToolUse 139 与 rollout function_call/function_call_output 双源重复；UserPromptSubmit 10 / PermissionRequest 2 为已下线 hook 存量 | 07-31 hook 收敛（剩 SessionStart/Stop/SubagentStop）前的旧采集；当前 hook 记录（SessionStart/Stop）仅承载 traces 元数据与 done 触发，前端渲染为普通卡片 |
| 4 | **trace status（active/done）无业务消费且不可靠** | 8 个 trace = 7 done + 1 僵尸 active（id=369，kill -9 遗留，无超时兜底） | status 唯一消费者是前端列表页"状态"列 + 详情页徽标；服务端零业务消费；判定依赖客户端 hook 配合，僵尸无法自愈 |

## 目标

1. **event_msg 白名单采集**：客户端只采集 `task_complete` + `task_started`，其余 type（含未来可能出现的 `world_state`）不采集、不上报、不入库。
2. **turn_context 不采集**。
3. **hook 纯触发**：SessionStart / Stop / SubagentStop 继续安装、继续触发 transcript 增量读取，但不再生成任何 hook 记录（trace_events 无 `hook_event` 类型行）。
4. **status 彻底移除**：`traces.status` 字段、done 判定链路、前端状态列/徽标全部删除。
5. **存量清洗**：生产库删除已确认冗余的旧数据（event_msg 非白名单 234 条、turn_context 12 条已清洗；hook_event 313 条待清洗）。

## 非目标

- 不改变 trace 页面展示的信息主体（消息、工具调用、系统提示词、时间线）。
- 不重构 claude agent 采集（保持现状，仅保证接口兼容）。
- 不做服务端超时兜底/轮询机制（status 移除后不再需要）。
- 不删除数据库列（gorm AutoMigrate 不删列，代码层移除即可，避免生产 DDL）。

## 设计决策

### D1. 过滤发生在客户端采集时

在 transcript 增量读取阶段丢弃被忽略记录（不进 spool、不上报、不入库），最大化省传输与存储。过滤规则归属各 agent adapter（agent 特定数据格式知识），通过 `AgentAdapter` 接口新增方法统一表达：

```go
// IgnoreTranscriptLine 返回 true 表示该行 transcript 记录不采集。
// codex：event_msg 仅放行 task_complete/task_started；turn_context 全部忽略。
// claude：返回 false（现状不变）。
IgnoreTranscriptLine(meta TranscriptMeta) bool
```

调用点：`RolloutReader.parseRolloutLines` 在 `r.rolloutRecord(...)` 之后判断，跳过则 `continue`（不 Append、不产生 PendingRecord）。

### D2. hook 元数据 per-session 持久化（仅 codex）

现在 batch 的 `Model/CWD/Source` 取自 `batch[0]` 的 hook 记录；hook 不再生成记录后，rollout 记录本身不带这些字段（`rolloutRecord` 未填充 Model/CWD/SessionSource），必须改从持久化状态读取。

- SessionStart / Stop hook 触发时，将 `{SessionID, Model, CWD, Source}` 写入 per-session 状态文件（复用现有 `stateDir` + 文件锁机制，按 sessionID 命名，如 `<sessionID>.meta`，权限 0600）。
- `flush` / `flushSubagent` 组装 batch 时，从状态文件读取 `Model/CWD/Source` 填充 `ingestBatch`；读取失败时降级为空串（服务端容忍空元数据，仅影响展示）。
- 状态文件随 trace 删除或会话结束可清理（无清理任务则残留无害，按 sessionID 覆盖写入）。
- **仅对 codex 生效**：claude 与 codex 共用 `Ingestor.Ingest()`，claude 注册 11 个 hook（SessionStart/UserPromptSubmit/PreToolUse/PostToolUse/Stop/SubagentStart/SubagentStop/PreCompact/PostCompact/SessionEnd），其 hook 记录仍是 claude 采集的数据源之一，**保持现状**（claude 采集重构不在本次范围）。实现上按 `adapter.Name() == TraceAgentCodex` 分流，claude 走原逻辑。

### D3. status 全链路移除（不做 done 判定）

> 附注：因 status 移除，D2 方案无需 batch 级 done 标记（早期讨论的 `done: true` 字段不再需要），hook 纯触发 + 元数据持久化即完成闭环。

`active/done` 生命周期状态无业务消费者且不可靠，整体移除：

- 服务端删除：`traceDoneEvents` 注册表、`doneEvents` 参数、`isComplete` 计算、`MarkDone` 调用与仓库方法、`TraceStatusActive/Done` 常量。
- 模型/仓库/DTO/查询视图同步删除 `Status` 字段（`ReportTraceRecordResult.Status` 是 accepted/duplicate/rejected，**保留**，概念不同）。
- 数据库 `traces.status` 列保留不删（AutoMigrate 不删列），代码不再读写。
- 前端删除：列表页 `statusBadge` + 表格"状态"列 + 卡片角标、详情页 `statusBadge`、`TraceSummary.status` / `TraceDetail.status` 字段、i18n `trace.status_*` 键（zh/en/ja）。

### D4. task_complete / task_started 保留为纯事件

移除 done 判定后，`task_complete` / `task_started` 不再承担状态机职责，仅作为时间线上任务生命周期标记（前端普通卡片）。event_msg 白名单仍保留两者，为未来展示"任务开始/完成时间点"留数据。

## 改造后数据流

```
codex CLI
  │ SessionStart hook ──► ingest(codex 分支) ──► 写 per-session 元数据(model/cwd/source) + 触发 ReadNew（不生成记录）
  │ Stop hook        ──► ingest(codex 分支) ──► 写 per-session 元数据 + 触发 ReadNew（不生成记录）
  │ SubagentStop hook ──► ingest ──► 触发子代理 ReadNewForSubagent（不生成记录，现状不变）
  │ transcript(jsonl) ──► RolloutReader.parseRolloutLines
  │                        ├─ session_meta        → 采集（dedup 稳定键）
  │                        ├─ turn_context        → IgnoreTranscriptLine=true，丢弃
  │                        ├─ response_item       → 采集
  │                        ├─ event_msg           → 白名单校验，仅 task_complete/task_started
  │                        └─ unknown             → 采集（现状不变）
  │
  │ claude CLI：Ingest(claude 分支) 保持现状 —— 生成 hook 记录 + 触发 ReadNew
  ▼
spool → flush → ingestBatch{SessionID, Model, CWD, Source, Records[]}
  ▼
服务端 report_trace_event：Upsert traces（元数据） + insert trace_events（rollout 记录）
  ▼
前端：/api/v1/trace/list + /api/v1/trace/event/list（无 status、无 hook_event）
```

## 删除面清单

### Go 服务端

| 文件 | 删除内容 |
|------|---------|
| `internal/common/constant/sql.go` | `TraceStatusActive`、`TraceStatusDone`（保留 `TraceRecordStatus*` 与 event 常量） |
| `internal/infrastructure/database/model/trace.go` | `Trace.Status` 字段 |
| `internal/infrastructure/repository/trace_repository.go` | `m.Status`/`t.Status` 赋值 |
| `internal/domain/trace/repository.go` | `Trace.Status` 字段、`MarkDone` 方法声明 |
| `internal/application/trace/command/report_trace_event.go` | `traceDoneEvents`、`doneEvents` 参数、`Status: Active`/`existing.Status`、`isComplete` 判定、`MarkDone` 调用 |
| `internal/application/trace/query/get_trace.go` / `list_traces.go` | `Status` 映射 |
| `internal/application/trace/port/handler.go` | 视图类型 `Status` 字段 |
| `internal/handler/trace.go` | `TraceSummary`/`TraceDetail` 构造中的 `Status`（保留 `ReportTraceRecordResult.Status`） |
| `internal/dto/trace.go` | `TraceSummary.Status`、`TraceDetail.Status`（保留 `ReportTraceRecordResult.Status`） |

### 客户端（Go）

| 文件 | 改动 |
|------|------|
| `internal/client/trace/adapter.go` | `AgentAdapter` 接口新增 `IgnoreTranscriptLine(meta TranscriptMeta) bool` |
| `internal/client/trace/codex.go` | 实现 `IgnoreTranscriptLine`：event_msg 白名单（task_complete/task_started）+ turn_context 忽略；导出 event_msg 白名单常量或函数 |
| `internal/client/trace/claude.go` | 实现 `IgnoreTranscriptLine` 返回 false |
| `internal/client/trace/rollout.go` | `parseRolloutLines` 跳过被忽略行；`rolloutRecord` 不需要改（过滤在调用处） |
| `internal/client/trace/ingest.go` | **codex 分支**：hook 不再 `spool.Append` hook 记录，SessionStart/Stop 仍触发 `ReadNew`，写 per-session 元数据；`flush`/`flushSubagent` 的 Model/CWD/Source 改从状态读取。**claude 分支**：原逻辑不变 |

### 前端

| 文件 | 改动 |
|------|------|
| `web/src/lib/types.ts` | `TraceSummary.status`、`TraceDetail.status` 删除 |
| `web/src/app/(dashboard)/trace/page.tsx` | `statusBadge` 函数、卡片角标、表格"状态"列（TableHead + TableCell + i18n key）删除 |
| `web/src/components/trace-detail/trace-detail-client.tsx` | `statusBadge` 函数与使用处删除 |
| `web/src/locales/zh.json` / `en.json` / `ja.json` | `trace.status_active` / `trace.status_done` 键删除 |

### 测试

`test/unit/trace/` 与 `test/unit/client/trace/` 中所有涉及 hook 记录、done/status 的上报断言需同步（hook 记录不再上报、status 字段移除、batch 元数据来源变化）。

## 存量数据清洗（生产，需逐次授权）

1. ✅ 已完成（2026-08-01，事务内 DELETE）：`event_msg` 非白名单 234 条 + `turn_context` 12 条，已提交。
2. ⏳ 待做：`record_type='hook_event'` 全部 313 条（SessionStart 8 + Stop 12 + 旧 hook 293），与 rollout 双源重复、无消费。部署新客户端后不再产生新 hook 记录，可随时清理。

## 验证策略

- 单元测试：`go test -count=1 ./test/unit/trace/ ./test/unit/client/trace/` + 全量 `go test -count=1 ./...`。
- 前端：`npx tsc --noEmit` + eslint + `npm run build`。
- 部署后手工验证：
  1. 新采集 trace 的 `trace_events` 中无 `hook_event`、无 `turn_context`、`event_msg` 仅 task_complete/task_started；
  2. traces 的 model/cwd/source 元数据仍正确（来自 per-session 状态文件）；
  3. trace 详情/列表页无状态列/徽标，时间线无重复消息卡片。

## 已知权衡

- 移除 status 后无法在 UI 区分"进行中/已结束"会话；僵尸 active 概念消失（不再维护状态）。
- 客户端 per-session 元数据文件在会话异常终止时可能残留（无害，按 sessionID 覆盖）。
- codex 若未来 event_msg 出现 `response_item` 未覆盖的新内容类型，需显式加入白名单才会被采集。
