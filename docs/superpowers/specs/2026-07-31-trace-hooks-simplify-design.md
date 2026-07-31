# Trace Hook 简化与主/子 Agent 关联设计

日期：2026-07-31
状态：已确认（用户批准：保留 SessionStart，hook 收敛为 {SessionStart, Stop, SubagentStop}）

## 背景与问题

当前 codex 注册 10 个 hook（SessionStart、UserPromptSubmit、PreToolUse、PermissionRequest、PostToolUse、Stop、SubagentStart、SubagentStop、PreCompact、PostCompact），每个 hook 触发 `aris trace ingest`，每次 ingest 同时写入：① hook 事件本身（hook 记录）；② transcript 增量（rollout 记录）。

生产库实测（883 条 trace_events）发现冗余：

1. **双源重复**：同一逻辑事件被 hook 与 rollout 各存一份。104 个工具调用在 hook（PreToolUse/PostToolUse）与 rollout（function_call/function_call_output）双源都有，占 415 行（单源仅需 ~208 行）；hook Stop 的 `last_assistant_message` 与 rollout `agent_message` 内容重复；hook UserPromptSubmit 与 rollout user_message 基本重叠。去重只发生在读取投影时（`dedupeMessage`/`dedupeToolCall` 的 rollout 升级逻辑），写入不去重。
2. **session_meta 重复存储**：压缩/轮次重写 transcript 文件（截断重写，新文件从 session_meta 开始），行号重置 + 内容变化 → dedup key（`rollout:{session}:{line}:{hash}`）失效 → 同一会话存多份 session_meta（实测 trace 174 存 4 份、trace 143 存 3 份，各 ~22 kB）。
3. **永不消费的事件**：`message`（85 条/163 kB）、`token_count`（96 条/81 kB）、`reasoning` 等不参与对话投影，仅原始时间线 API 可见。

## 目标

1. hook 注入收敛为 3 个：**SessionStart、Stop、SubagentStop**，消除 hook 与 rollout 双源重复（UserPromptSubmit/PreToolUse/PostToolUse/PermissionRequest/SubagentStart/PreCompact/PostCompact 不再注入）。
2. 主会话数据只来自主 transcript（Stop 时增量上报）；子代理数据来自子代理独立 transcript（SubagentStop 时增量上报）。
3. 存储层面建立主/子 Agent trace 关联（`traces.parent_trace_id`），子代理内容完整可分析。
4. 顺带修复 session_meta 重复存储（dedup key 改用稳定语义）。

## 方案

### 1. Hook 注册收敛（codex）

`TraceClientCodexHookEvents` 从 10 个收敛为 3 个：

```go
var TraceClientCodexHookEvents = []string{
    TraceEventSessionStart,   // 会话开始：元数据 + 主 transcript 首次读取/resume 兜底
    TraceEventStop,           // 每轮结束：主 transcript 增量上报
    TraceEventSubagentStop,   // 子代理结束：子代理 transcript 增量上报（唯一带 agent_transcript_path）
}
```

已安装机器重跑 `aris trace install` 时，`installAgentHooks` 现有清理逻辑（按 `" trace ingest"` 子串移除旧 aris hook 组）自动收敛，无需手动清理。`InspectCodexHooks` 同步只检查这 3 个。

**依据**（docs/openai/codex_hooks.md + codex_session_format.md + 生产数据验证）：

- `SessionEnd` timeout 仅 1-3s 且明确不运行于 subagent，不能作为上报点；`Stop` 是 per-turn、timeout 600s、含 `transcript_path`，是最合适的主会话上报点。
- 子代理是独立 session（独立 transcript 文件，`session_meta.source.subagent.thread_spawn.parent_thread_id` 反向指向父会话）；主 transcript 的 `spawn_agent` function_call 参数不含子代理 transcript 路径；`agent_transcript_path` 只存在于 `SubagentStop` hook 输入——这是采集子代理内容的唯一途径。
- Codex 无崩溃专用 hook。兜底 = Stop/SubagentStop 提前上报（崩溃最多丢当前轮）+ spool 本地落盘重试 + resume 后首个 Stop 重读增量。
- 压缩会截断重写主 transcript 文件（新文件从 session_meta 开始，数据库实测 session_meta 与 turn_context 行号一一对应），压缩后的内容全部在主 transcript 中，去掉 PreCompact/PostCompact 只丢"何时压缩"信号，内容完整。

### 2. 客户端行为（internal/client/trace/）

#### SessionStart

行为不变：上报 hook 记录（含 session_id/model/cwd/source）→ 服务端创建 trace（active，source=startup/resume/clear/compact）。顺带触发主 transcript 增量读取（resume 时补报上次未上报增量）。

#### Stop（主会话）

- 读取主 transcript 增量，逐行上报（rollout 记录）。
- **Stop 记录本身裁剪 payload**：原始输入含 `last_assistant_message` 全文（与 rollout agent_message 重复）。客户端生成合成轻量记录，payload 只保留 `{session_id, hook_event_name, turn_id, stop_hook_active}` 等小字段。
- 服务端命中 Stop → 主 trace `MarkDone`（现有 doneEvents 机制不变）。

#### SubagentStop（子代理）

- `codexAdapter.ParseHook` 扩展 `codexHookEnvelope` 支持 `agent_transcript_path`、`agent_id`、`agent_type` 字段 → `HookInfo` 增加 `AgentTranscriptPath`、`AgentID`、`AgentType`。
- **只读子代理 transcript 增量**（`agent_transcript_path`），不读主文件（避免与 Stop 重复）。复用 `RolloutReader`（state 按 transcript 路径 hash 分派，天然支持多文件）。
- 子代理记录的 `SessionID` 用**子代理自己的 id**（优先从 transcript 文件名 `rollout-<ts>-<id>.jsonl` 解析，fallback 读首行 `session_meta.payload.id`），用于 dedup key（`rollout:{子session}:{line}:{hash}`）与入库归属。
- `ingestBatch` 增加 `parent_session_id`（= hook 输入的父 session_id）与子代理元数据（agent_id/agent_type），随 batch 上报。
- 服务端命中 SubagentStop → 子 trace `MarkDone`。

#### PendingRecord / ingest 传输

- `PendingRecord` 增加 `ParentSessionID`；`ingestBatch` 增加 `ParentSessionID`、`AgentID`、`AgentType` 字段（batch 级，仅 SubagentStop 上报时填充）。
- `ingestRecord` 不变（事件级字段与现有一致）。

### 3. 存储结构（主/子关联）

**traces 表新增字段**（GORM AutoMigrate，model 加 tag 即迁移）：

```go
ParentTraceID uint `gorm:"column:parent_trace_id;not null;default:0;index:idx_trace_parent;comment:父 trace id，0 表示主 trace"`
```

- 主 trace：`parent_trace_id = 0`
- 子 trace：`parent_trace_id = 父 trace.id`，由服务端 `ensureTrace` 按 `parent_session_id` 查父 trace 建立；父 trace 不存在时容错（子 trace 照常创建，parent_trace_id=0，避免子代理数据丢失）。
- 子代理元数据（agent_id/agent_type/agent_nickname）存 `traces.metadata`（已有 serializer:json 字段）。
- 子 trace 的 `source` 设为 `"subagent"`（新增枚举，标识子代理派生）。
- **trace_events 不变**：子代理事件挂子 trace，按 trace_id 天然隔离。

**DTO/API**：

- `TraceSummary`、`TraceDetail` 增加 `ParentTraceID uint` 字段（`json:"parentTraceId"`）。
- 列表接口展示子 trace（带 parentTraceId），主/子关系由前端按需组织；本次不实现 children 聚合查询。

### 4. 服务端改动（internal/application/trace、internal/domain/trace、internal/infrastructure/repository）

- `port.ReportTraceEventCommand` 增加 `ParentSessionID`、`AgentID`、`AgentType`（batch 级）。
- `domain/trace.Trace` 与 `dbmodel.Trace` 增加 `ParentTraceID`；`toTraceRecord`/`toTraceDomain` 同步。
- `ensureTrace`：`ParentSessionID` 非空时查父 trace（`FindBySessionID`），命中则设置子 trace 的 `ParentTraceID`；未命中容错。子 trace 的 `model/cwd` 来自 SubagentStop hook 输入（Common input 自带），`source` 置 `"subagent"`。
- `traceDoneEvents`：codex 注册事件集增加 `TraceEventSubagentStop`（现有 `{Stop, TaskComplete}` → `{Stop, TaskComplete, SubagentStop}`）。
- `UpsertBySessionID` 的 OnConflict 更新列增加 `parent_trace_id`。
- trace 列表/详情查询无需过滤（子 trace 正常展示，带 parentTraceId 字段）。

### 5. session_meta 去重修复（顺带）

**根因**：压缩/轮次重写 transcript 后行号重置、内容变化（git commit、dynamic_tools 变化），`rollout:{session}:{line}:{hash}` dedup key 失效。

**修复**：客户端生成 rollout 记录的 dedup key 时，对 `session_meta` 记录改用稳定语义键：`rollout:{session}:session_meta:{payload.id}`（会话 ID 稳定，压缩重写后重读自动去重）。其他记录保持现有 `line:hash` 键。

实现点：`rolloutRecord` 生成 dedup key 时按 `meta.RecordType == session_meta` 分支。注意 `session_meta` 的 `payload.id` 需从 raw 行提取（`ClassifyTranscriptLine` 当前未解析 id，需扩展 `TranscriptMeta` 或单独提取）。

### 6. 信息缺口（接受清单）

| 去掉的 hook | 影响 | 结论 |
|---|---|---|
| UserPromptSubmit | prompt 内容在 rollout user_message 已有 | 覆盖 ✓ |
| PreToolUse / PostToolUse | tool_input/tool_response 在 rollout function_call/function_call_output 已有 | 覆盖 ✓ |
| PermissionRequest | 权限请求事件 rollout 无对应 | 丢失（生产库仅 2 条，可接受） |
| SubagentStart | agent_id/agent_type 元数据由 SubagentStop 提供；子代理内容由子 transcript 采集 | 覆盖 ✓ |
| PreCompact/PostCompact | 压缩触发信号丢失，内容在 transcript 完整 | 可接受 |

### 7. 崩溃与异常兜底

| 场景 | 兜底 |
|---|---|
| 会话中途崩溃 | 前几轮 Stop 已上报，最多丢当前轮 |
| 子代理崩溃 | 子代理数据在其自己的 SubagentStop 上报；子代理异常导致 SubagentStop 不触发时，该子代理内容丢失（与主会话同理，可接受） |
| 上报失败（网络/服务端不可用） | spool 本地落盘 + 重试（现有机制） |
| resume | SessionStart(source=resume) 触发主 transcript 增量读取，补报上次未上报增量 |

### 8. 测试与验证

- 服务端单元测试（test/unit/trace/）：
  - 新增：report 携带 ParentSessionID → 子 trace 建立 parent_trace_id 关联；父不存在时容错。
  - 新增：SubagentStop 命中 → 子 trace done；主 trace 仍由 Stop 标记 done。
  - 现有测试（usecase_test 等）行为不变，应全绿。
- 客户端行为测试（test/unit/trace/ 下新增或独立 topic）：Stop 记录 payload 裁剪；SubagentStop 读取 agent_transcript_path 增量且 SessionID 用子代理 id；session_meta dedup key 稳定化。
- 手工验证：本地 codex 会话（含真实子代理场景）→ 检查 traces 主/子关联、事件无 hook 重复、session_meta 无重复。

## 不做的事（YAGNI）

- 不做子 trace children 聚合 API（前端按 parentTraceId 自行组织）。
- 不改 claude agent 的 hook 集合（其 SessionEnd timeout 1-3s 需单独设计，本次范围仅 codex）。
- 不在本次实现子代理 transcript 的实时增量采集（SubagentStop 一次性上报已覆盖完整生命周期）。
- 不删除历史冗余数据（迁移成本高，新逻辑上线后自然不再产生）。
