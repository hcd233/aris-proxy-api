# Claude Trace 设计：多 Agent 统一抽象与 Claude Code 会话摄取

## 1. 背景

aris-proxy-api 已实现 Codex CLI 的 trace 上报（见 `2026-07-17-trace-codex-rollout-design.md`）：`aris trace ingest` 客户端读取 hook stdin JSON 与 rollout JSONL，经本地 spool 批量上报到 `POST /api/v1/trace/event`，服务端幂等入库并投影为对话视图。

当前实现把 Codex 格式硬编码在两层中：

- **客户端**：hook envelope 按 Codex 字段解析（`turn_id` / `tool_use_id`）；rollout 按 Codex envelope（`session_meta` / `turn_context` / `response_item` / `event_msg`）分类；stdout 契约硬编码 `Stop` → `{}`；安装脚本只写 `~/.codex/hooks.json`。
- **服务端**：`ensureTrace` 硬编码 `Agent: codex`；完成事件硬编码 `Stop` / `task_complete`；`BuildConversation` 投影硬编码 Codex 事件名。

Claude Code 的数据链路同构但格式不同（格式事实见 `docs/anthropic/claude_session_format.md` 与 Claude Hooks 参考文档）：

- Hook 事件名与 Codex 高度重合，但**无 `turn_id`**（轮次用 `prompt_id` 标识），另有 `PostToolUseFailure`、`PostToolBatch`、`SessionEnd`、`Notification` 等事件。
- Transcript 为 `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl`，记录类型为 `user` / `assistant` / `attachment` / `system` / `progress` / `permission-mode` / `file-history-snapshot` / `last-prompt`，通过 `parentUuid→uuid` 构成消息树。
- Hook 配置在 `~/.claude/settings.json` 的 `hooks` 键，需保护同文件其他设置项。
- stdout 契约不同：`SessionStart` / `UserPromptSubmit` 的 stdout 会被注入模型上下文，trace hook 必须保持静默。
- `Stop` 事件每轮（turn）结束都触发，不能作为会话完成信号；会话终止用 `SessionEnd`。
- 子代理在 `<session-uuid>/subagents/agent-<id>.jsonl` 独立记录，`isSidechain: true`；settings 级 hook 在子代理内同样触发。

本期目标：抽取统一的多 Agent 抽象，接入 Claude Code trace 上报，并为后续其他 agent 保留扩展点。

## 2. 目标与非目标

### 2.1 目标

1. `aris trace ingest --agent <name>` 显式区分 agent；codex 与 claude 的 hook 命令都显式携带 `--agent`。
2. 客户端按 adapter 抽象 hook 解析、transcript 行分类、stdout 契约；spool、offset 增量读取、批量上报、fail-open 契约全 agent 共享。
3. 服务端 batch envelope 携带 `agent`；完成事件与对话投影按 agent 注册表分发。
4. 摄取 Claude Code 主会话与子代理的 hook 事件及 transcript 记录，原样保存，未知字段不丢失。
5. Claude 对话投影：按 turn 分组展示 user / assistant / tool_call / tool_result，rollout（transcript）优先、hook fallback 去重补齐。
6. 安装脚本支持选择 Codex / Claude Code / 两者，幂等注册 hook，保护已有配置。
7. 全程 fail-open：任何采集错误不改变 Claude Code / Codex 的行为。

### 2.2 非目标

1. 不做完整插件体系（动态注册、每 agent 独立配置节、web UI 插件）；两个 agent 用轻量注册表即可。
2. 不把两种格式归一化为中间 schema 存储；原始 payload 原样保留仍是硬约束。
3. 投影 v1 不展示 thinking/reasoning 块（与 codex 现状一致），不做子代理独立对话视图（`isSidechain` 记录进 raw timeline，不进主投影）。
4. 不支持 Windows；不支持除 codex / claude 外的 agent。
5. 不改 Trace 查询 API、票据/下载链路（install 脚本为服务端渲染模板，web 前端无改动）。

## 3. 关键决策

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | agent 识别 | hook 命令显式携带 `--agent codex` / `--agent claude`；允许 breaking change |
| 2 | 抽象形态 | 轻量 adapter 注册表（客户端）+ 两个 registry（服务端完成事件、投影构造器） |
| 3 | 缺失 `--agent` | 写本地错误日志并 exit 0，不上报（fail-open） |
| 4 | batch envelope | 新增顶层 `agent` 字段；服务端为空时默认 `codex`（兼容在途旧 spool 记录） |
| 5 | legacy 单事件路径 | 删除（`records` 为空且无 `agent` 的旧兼容分支） |
| 6 | Claude 完成事件 | `SessionEnd` 置 trace done；`Stop` 只表示 turn 完成 |
| 7 | Claude stdout 契约 | 恒静默（任何事件都不输出）；codex 保持 `Stop` → `{}` |
| 8 | 子代理记录 | 摄取进库（hook 在子代理内触发时经 `transcript_path` 自动增量读取）；投影 v1 跳过 `isSidechain` 记录 |
| 9 | Claude turn 归组 | 客户端不写 TurnID；投影两遍扫描：transcript 真实输入记录的 `promptId→uuid` 建 alias，hook `prompt_id` 经 alias 归并 |
| 10 | thinking 块 | 投影跳过；原始记录仍在 |
| 11 | 安装脚本 | 单脚本内多选 Codex / Claude Code（默认全选），按选择写对应配置文件 |

## 4. 统一契约规范

后续新 agent 接入需满足下述契约；这是"统一抽象与规范"的核心。

### 4.1 AgentAdapter（客户端）

```go
// internal/client/trace/adapter 或同包内 agent.go
type HookInfo struct {
    SessionID      string
    EventName      string
    Model          string   // 仅 SessionStart 类事件携带
    CWD            string
    SessionSource  string   // startup/resume/...
    TurnID         string   // codex: turn_id；claude: prompt_id
    CallID         string   // 工具调用关联 ID（tool_use_id）
    TranscriptPath string
}

type TranscriptMeta struct {
    RecordType string // 归一化记录类型（对应 DB record_type）
    Event      string // 细分事件名（对应 DB event）
    TurnID     string
    CallID     string
}

type AgentAdapter interface {
    Name() string
    ParseHook(raw []byte) (HookInfo, error)
    ClassifyTranscriptLine(raw []byte) TranscriptMeta // 单行 transcript 分类，不失败（未知→unknown）
    StdoutAck(info HookInfo) string                   // hook 完成后需回显的 stdout；空串=静默
}

// 触发 trace done 的事件集由服务端 registry 持有（见 4.3），与客户端解耦。
```

registry：`map[string]AgentAdapter`，`LookupAdapter(name)` 返回 `(adapter, ok)`。codex、claude 各实现一个文件。

### 4.2 上报信封（不变 + agent）

- 本地 spool `PendingRecord` 增加 `agent` 字段；batch envelope 增加顶层 `agent`。
- 幂等键沿用 `hook:<spoolID>:<seq>` 与 `rollout:<sessionID>:<line>:<sha256(raw)>`，不加 agent 前缀（spoolID / session_id 天然区分）。
- 批次仍按单 session 聚合；同一 session 必属同一 agent。

### 4.3 服务端 registry

```go
// 完成事件注册表
var traceDoneEvents = map[string][]string{
    "codex":  {"Stop", "task_complete"},
    "claude": {"SessionEnd"},
}

// 投影构造器注册表（internal/domain/trace）
var conversationBuilders = map[string]func([]*TraceEvent) *Conversation{
    "codex":  buildCodexConversation,   // 现有 BuildConversation 逻辑改名
    "claude": buildClaudeConversation,  // 新增
}
// 对外：BuildConversationFor(agent string, events []*TraceEvent) (*Conversation, error)
```

`ensureTrace` 用 `cmd.Agent`；同 `session_id` 已存在且 agent 不一致时 reject 该批次（防御数据串号）。

### 4.4 Claude 数据事实（实现依据）

- Hook stdin 公共字段：`session_id`、`transcript_path`、`cwd`、`hook_event_name`、`permission_mode`、`prompt_id`（v2.1.196+）；工具事件另有 `tool_name` / `tool_input` / `tool_use_id` / `tool_response`(PostToolUse) / `error`(PostToolUseFailure)；Subagent 事件另有 `agent_id` / `agent_type` / `agent_transcript_path`(SubagentStop)；SessionStart 另有 `source` / `model`。
- Claude hook event 无 matcher 字段要求；注册 `matcher: ""` 匹配全部。
- Transcript 行分类规则：
  - `user`：content 为字符串 → event `user_prompt`；content 为数组（tool_result）→ event `tool_result`，CallID 取首个 `tool_use_id`。真实输入记录含 `promptId` 与 `uuid`（turn alias 依据）。
  - `assistant`：event `assistant_message`；CallID 取首个 `tool_use` 块 id。
  - `attachment` → `attachment`；`system` → event 取 `subtype`（如 `turn_duration`）；`permission-mode` / `file-history-snapshot` / `last-prompt` / `summary` → type 原样；不认识 → `unknown`。
- 子代理 transcript 位于 `<session>/subagents/*.jsonl`，`isSidechain: true`，记录结构与主会话同构；hook 在子代理内触发时其 `transcript_path` 指向子代理文件，offset 机制按路径哈希分派状态，天然支持多文件增量读取。
- 旧版本 Claude 的 hook input 可能无 `prompt_id`：hook 记录 TurnID 留空，投影归入当前进行中的 turn。

## 5. 客户端改造

### 5.1 命令与配置

- `cmd/client`：`trace ingest` 加 `--agent` string flag。缺失/未知 agent → `writeLocalError` 后 exit 0（fail-open）。
- `~/.aris/trace/config.json`：废弃 `agent` 字段（读取时忽略，不回写）。
- spool `PendingRecord.Agent`：旧记录无此字段时按 `codex` 兜底。
- batch 取首条记录 session 聚合一组，`agent` 取组内首条记录的 agent。

### 5.2 adapter 接入点

- `ingest.go`：`adapter.ParseHook` 替代硬编码 `hookEnvelope`；stdout ack 由 `adapter.StdoutAck` 决定（在 ingest 前先按解析结果回显，任何后续失败不影响已回显内容）。
- `rollout.go`：保留 offset / 文件锁 / 原子状态 / 半行处理；`rolloutRecord()` 的分类段替换为 `adapter.ClassifyTranscriptLine(raw)`。文件名与类型名是否泛化（如 `TranscriptReader`）以最小 churn 为准：保留 `RolloutReader` 名称，注释说明其为通用 transcript 增量读取器。

### 5.3 codex adapter

把现有 `hookEnvelope`、`rolloutRecordType`、Stop→`{}` 语义原样搬入 `codex.go`，行为零变化。

## 6. 服务端改造

- DTO `ReportTraceEventReqBody`：加 `agent` 字段（omitempty→服务端默认 codex）；`records` 改为必填；**删除 legacy 单事件字段**（`hook_event_name` / `turn_id` / `prompt` / `tool_name` / `tool_use_id` / `tool_input` / `tool_response` / `last_assistant_message` / `agent_id` / `agent_type` / `trigger` / `transcript_path` / `permission_mode` 及 `Raw` 透传），envelope 仅保留 `session_id` / `model` / `cwd` / `source` / `agent` / `records`。command/port 同步加 `Agent`、删 `RawPayload` 与单事件字段。
- `report_trace_event.go`：
  - `Handle`：解析/校验 agent（空→codex；非法→批报 validation error）。
  - 删除 legacy 单事件拼装分支（`normalizeRecords` 中 `len(records)==0` 的构造路径）；`records` 为空直接 validation error。
  - `ensureTrace`：Trace 创建/更新用 `cmd.Agent`；已有 trace 的 Agent 与 cmd.Agent 不同 → 返回 validation error。
  - 完成判定：`lo.Contains(traceDoneEvents[agent], record.Event)` 替代硬编码。
- 查询层（list/get/events）无改动。

## 7. Claude 对话投影（`internal/domain/trace/claude.go`）

`buildClaudeConversation(events []*TraceEvent) *Conversation`：

1. **Pass 1**：遍历 source=rollout 且 event=`user_prompt` 的记录，解析 payload 取 `uuid` / `promptId`，建 `alias[promptId]=uuid`。
2. **Pass 2**：按事件顺序遍历：
   - transcript `user_prompt`（非 sidechain）：开启新 turn（key=`uuid`），产生 user 消息项（content 字符串）。
   - transcript `assistant_message`：text 块 → assistant 消息（多块拼接）；`tool_use` 块 → tool_call 项（CallID=`id`，Arguments=`input` 原始 JSON）；thinking 块跳过。
   - transcript `tool_result`：逐块按 `tool_use_id` 回填既有 tool_call 的 Output；content 为数组时序列化紧凑 JSON。
   - hook `UserPromptSubmit` → user 消息（content=`prompt`）；hook `Stop` → assistant 消息（`last_assistant_message`）；hook `PreToolUse` → tool_call（CallID=`tool_use_id`，ToolName，Arguments=`tool_input` 原始）；hook `PostToolUse` → 回填 Output（`tool_response` 原始）。turn 用 `prompt_id` 经 alias 归并；无 alias 时归当前 turn，无当前 turn 入 `session` 兜底。
   - `isSidechain: true` 记录：跳过投影（仍在 raw 记录表）。
3. **去重**：复用现有 role+content 消息去重与 callID tool 去重 helper（升级到 rollout 来源覆盖 hook）；这部分从 `rollout.go` 抽出为共享私有函数，codex 投影行为不变。
4. 只有调用无结果 → tool_call 保留 pending（无 Output）；有结果无调用 → 忽略（与 codex 一致）。

命名与文件：`internal/domain/trace/` 下现有 `rollout.go`（codex 投影 + RolloutRecord 解析）保留；共享类型（`Conversation` / `ConversationTurn` / `ConversationItem`）与去重 helper 移至 `conversation.go`；codex 构造器改名 `buildCodexConversation` 留在 `rollout.go`；新增 `claude.go` 与 `projector.go`（registry + `BuildConversationFor`）。

## 8. 安装脚本

`install_trace_client.sh.tmpl` 调整：

- Step `[2/4]`：多选 `Select agent`（`huh.NewMultiSelect`，Space 勾选/取消，Enter 确认），默认 Codex、Claude Code 均选中，禁止空选；确认后回显 `✓ Agent · Codex, Claude Code`。
- Step `[4/4]`：按选择执行一到两段 hook 注册：
  - Codex：`~/.codex/hooks.json`，命令 `~/.aris/bin/aris trace ingest --agent codex`，事件集不变（10 个）。
  - Claude：`~/.claude/settings.json`，命令 `~/.aris/bin/aris trace ingest --agent claude`，事件集：SessionStart UserPromptSubmit PreToolUse PostToolUse PostToolUseFailure Stop SubagentStart SubagentStop PreCompact PostCompact SessionEnd（11 个）。jq 表达式与 codex 段同构（`( .hooks[$event] // [] ) | map(select(...command 去重...)) + [$group]`），文件不存在时从 `{}` 起步，写前 `.bak` 备份 + 0600，临时文件原子替换。
- 完成提示追加：Claude Code 重启会话后 hook 生效（settings 文件变更会被自动监听，但鼓励用户用 `/hooks` 审核）。

## 9. 错误处理与安全

1. 客户端任何解析/IO/网络错误只写本地错误日志（不含 payload / API Key），exit 0。
2. claude adapter 的 `ParseHook` 缺 `session_id` / `hook_event_name` → 视为非法输入，不上报但 exit 0。
3. `~/.claude/settings.json` 写坏风险：jq 解析失败时中止该段并保持原文件（先解析校验再原子替换）。
4. 服务端拒绝未知 agent，防止拼写错误静默落库为第三种 agent。
5. payload 含系统提示、隐私内容：沿用 owner 隔离与"日志不打印 payload"约束。

## 10. 测试策略

### 10.1 单元测试

- claude adapter：`ParseHook` 各事件字段抽取（含 `prompt_id` 缺失）；`ClassifyTranscriptLine` 覆盖 `user_prompt` / `tool_result` / `assistant_message` / `attachment` / `system.subtype` / 未知类型；`StdoutAck` 恒空。
- codex adapter：行为与现状快照一致（回归）。
- ingest：`--agent` 缺失/非法 exit 0；batch 携带 agent。
- claude 投影：turn alias（hook prompt_id ↔ transcript promptId）、tool_use/tool_result 配对、多块 user tool_result、hook↔transcript 去重升级、thinking 跳过、sidechain 跳过、无 prompt_id 旧版 fallback。
- 服务端：agent 默认值、非法 agent reject、agent 冲突 reject、`SessionEnd` 置 done、`Stop` 不置 done（claude）。

### 10.2 E2E

扩展 `test/e2e/trace/`：模拟 claude hook 序列（SessionStart→UserPromptSubmit→PreToolUse→PostToolUse→Stop）+ fixture transcript JSONL（含 tool_use/tool_result/attachment/system），验证入库记录分类、trace agent=claude、投影 turn 结构与去重；codex 既有用例保持绿。

## 11. 验收标准

1. `aris trace ingest --agent claude` 由 `~/.claude/settings.json` hook 触发，hook 事件与 transcript 记录原样入库，`traces.agent='claude'`。
2. codex 链路行为不变（hook 命令改为显式 `--agent codex` 后同效果）。
3. Claude 对话投影按 turn 展示 user/assistant/tool，hook 与 transcript 不双显，子代理内容不污染主投影。
4. 安装脚本可选 Codex / Claude / Both，重复运行幂等，`~/.claude/settings.json` 其他键完好。
5. 全过程 fail-open；日志无 API Key / payload。
6. lint / 单测 / e2e 全绿。

## 12. 已知限制

1. 旧版本 Claude（<2.1.196）hook 无 `prompt_id`，对应 turn 归组降级到当前进行中 turn / `session` 兜底。
2. `prompt_id` 与 transcript `promptId` 的 alias 依赖 transcript 行已入库；hook 先到时该轮 hook 记录暂归当前 turn——投影是查询时实时计算，transcript 补齐后下一次查询自动归位。
3. 子代理对话内容只进 raw timeline，无结构化视图。
4. thinking 块不投影，需要推理视图时另行扩展 `ConversationItem.Kind`。
