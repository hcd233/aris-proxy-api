# Codex Trace 设计：独立 CLI、可靠采集与 Rollout 会话

## 1. 背景

aris-proxy-api 已经通过现有 `Session` / `Message` / `Tool` 记录经代理的 LLM API 对话，但 Codex CLI 的 agent 运行还包含一层更高的编排信息：用户输入、多个 turn、模型消息、推理摘要、工具调用、工具结果、子 agent、压缩和中止状态。

当前 Trace 实现由 Web 生成 shell hook，hook 使用后台 `curl` 后立即退出。Codex 回收 hook 进程时可能终止尚未完成的请求，导致一个完整会话只留下一个事件；安装脚本还会把 API Key 写入 hook，采集、配置和可靠投递逻辑都堆在 shell 中。即使上报可靠，Hook payload 也不保证包含完整的中间 assistant 消息和全部历史记录。

Codex 会把会话追加写入 rollout JSONL 文件。该文件以 `session_meta`、`turn_context`、`response_item`、`event_msg` 四类记录保存可回放的完整会话，因此本功能需要同时采集：

1. Hook 事件流：低延迟、实时展示生命周期和工具边界。
2. Rollout 记录流：完整重建对话、工具调用和 turn 上下文。

本次将 shell + `curl` 替换为独立的 `aris` 客户端二进制。服务端和客户端分别从 `cmd/server`、`cmd/client` 构建；客户端只暴露 `aris trace init` 和 `aris trace ingest`，不链接数据库、Server、lint 或 Web 静态资源。

## 2. 目标与非目标

### 2.1 目标

1. 独立构建 `aris` 客户端，支持 `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`。
2. 通过 JWT 鉴权、用户维度 `3 req/min` 限流和短期单次票据安全分发客户端，不把 JWT 写入长期配置。
3. 通过四步轻量向导完成服务器连接、Agent 选择、API Key 配置和 Codex Hook 配置。
4. 由 `aris trace ingest` 上报 Codex 当前支持的全部 command hook 事件，不因 hook 进程提前退出而静默丢失。
5. 可靠读取 `transcript_path` 指向的 rollout JSONL，并将新增记录增量上报。
6. 原样保存 Hook JSON 和 Rollout JSONL 记录，未知字段不丢失。
7. 使用 `session_id`、`turn_id` 和 `call_id` 串联一整个 agent 运行。
8. 提供类似 session 的 Trace 对话视图，至少包含用户消息、assistant 消息、推理摘要、工具调用和工具结果。
9. 支持 at-least-once 上报，通过客户端序号和服务端幂等键避免重试产生重复记录。
10. 采集失败时 fail-open，不阻塞、修改或拒绝 Codex 的正常行为。

### 2.2 非目标

1. 不把 Codex 数据写入现有 proxy `sessions`、`messages`、`tools` 表。
2. 不将 Trace 与 proxy Session 建立外键或强制关联。
3. 不把 rollout 格式当作跨 Codex 版本的稳定公共 API；解析器必须容忍未知记录和字段。
4. 不在本期实现跨 Agent 的全文检索、OTel/Langfuse 导出或训练数据导出。
5. 不在本期依赖服务端读取用户机器上的 `transcript_path`，该路径只能由本地客户端读取。
6. 不在首期支持 Windows，也不提前实现 Codex 之外的 Agent 适配器。
7. 不引入全屏 TUI、自动更新服务或独立客户端发布仓库。

## 3. 关键决策

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 命令边界 | 存量命令迁入 `cmd/server`；`cmd/client` 只注册 `trace init`、`trace ingest` |
| 2 | 客户端名称 | 独立二进制名为 `aris`，调用形式为 `aris trace init` / `aris trace ingest` |
| 3 | 产物平台 | 构建 Darwin/Linux 的 amd64/arm64 四个产物，作为只读文件随服务镜像发布 |
| 4 | 下载鉴权 | Web 以 JWT 换取 10 分钟有效的单次票据；下载时原子消费，不把 JWT 放入脚本 |
| 5 | 下载限流 | 票据签发接口按 JWT 用户 ID 使用 Redis 令牌桶限制为 `3 req/min` |
| 6 | 本地密钥 | API Key 保存于 `~/.aris/trace/config.json`，目录 `0700`、文件 `0600` |
| 7 | 顶层领域实体 | `Trace` 表示一次 Codex `session_id` 的 agent 运行 |
| 8 | 事实来源 | Hook 是实时补充源，rollout JSONL 是完整会话重建的首选源 |
| 9 | 存储方式 | Hook 与 rollout 记录统一存入 Trace 记录表，通过 `source` 和 `record_type` 区分 |
| 10 | 上报可靠性 | `trace ingest` 先落盘 spool，再尝试同步发送；失败记录由后续 Hook 重试 |
| 11 | 服务端幂等 | `session_id + source + client_sequence` 生成唯一幂等键；重复请求返回成功但不重复插入 |
| 12 | 会话重建 | 优先解析 rollout；rollout 缺失或解析不完整时，再使用 Hook 事件补齐 |
| 13 | 工具关联 | rollout 工具使用 `call_id` 关联；Hook 使用 `tool_use_id`，仅作为 fallback |
| 14 | 代理关系 | Trace 与现有 proxy Session 正交，v1 不建立关联 |
| 15 | 生命周期 | `Stop` 仅将 Trace 标记为 done，不影响当前 Stop 记录和迟到 rollout 记录保存 |

## 4. Codex 数据事实

### 4.1 Hook 事件

每个 command hook 从 stdin 接收一个完整 JSON 对象。安装配置必须覆盖：

| 事件 | 范围 | 关键字段 | Trace 用途 |
|------|------|----------|------------|
| `SessionStart` | session | `session_id`, `model`, `cwd`, `source`, `transcript_path` | 初始化 Trace 和 rollout 路径 |
| `UserPromptSubmit` | turn | `turn_id`, `prompt` | 实时用户消息 fallback |
| `PreToolUse` | turn | `turn_id`, `tool_name`, `tool_use_id`, `tool_input` | 实时工具调用开始 fallback |
| `PermissionRequest` | turn | `turn_id`, `tool_name`, `tool_input` | 记录审批边界，不做决策 |
| `PostToolUse` | turn | `turn_id`, `tool_name`, `tool_use_id`, `tool_input`, `tool_response` | 实时工具结果 fallback |
| `Stop` | turn | `turn_id`, `last_assistant_message` | 实时 assistant fallback、turn 完成 |
| `SubagentStart` | subagent | `turn_id`, `agent_id`, `agent_type` | 记录子 agent 边界 |
| `SubagentStop` | subagent | `turn_id`, `agent_id`, `agent_type`, `last_assistant_message` | 记录子 agent 结束 |
| `PreCompact` | turn | `turn_id`, `trigger` | 记录压缩开始 |
| `PostCompact` | turn | `turn_id`, `trigger` | 记录压缩完成 |

Hook 事件字段会随 Codex 版本增加。服务端不得只依赖固定 DTO 重建 JSON；固定字段用于索引，原始请求 body 必须完整保留。

### 4.2 Rollout JSONL

每行是一个独立 JSON record，顺序由文件追加顺序决定。统一 envelope 为：

```json
{
  "timestamp": "2026-07-09T07:53:04.719Z",
  "type": "response_item",
  "payload": {}
}
```

记录分类及用途：

| `type` | 主要 `payload` | 重建用途 |
|--------|----------------|----------|
| `session_meta` | session id、cwd、来源、版本、provider、git、工具定义 | Trace 元数据、会话头 |
| `turn_context` | turn id、model、权限、sandbox、instructions | Turn 配置快照 |
| `response_item` | `message`、`reasoning`、`function_call`、output、custom tool、web search | 模型交互事实、工具调用关联 |
| `event_msg` | task、user/agent message、reasoning、token、tool end、compact、error | UI 消息、统计和生命周期 |

解析必须遵守：

1. 先用 envelope 的 `type` 分支，再用 `response_item` / `event_msg` 的 `payload.type` 分支。
2. `function_call.arguments` 是 JSON 字符串，解析失败时保留原始字符串，不丢弃记录。
3. `custom_tool_call.input` 是原始文本，不强制 JSON 解析。
4. `function_call` 与 `function_call_output` 通过 `call_id` 关联。
5. `event_msg/task_started` 与 `event_msg/task_complete` 界定 turn；记录自身的 `turn_id` 是主要归组键。
6. `event_msg` 与 `response_item` 可能重复表达同一消息；对话视图必须去重，原始记录不能删除。

## 5. 总体架构

```text
Web（已有 JWT）
  ├─ POST /api/v1/trace/client/ticket
  │    └─ 按 user_id 限制 3 req/min，返回 10 分钟单次票据
  └─ 生成短安装脚本
       ├─ 探测 Darwin/Linux + amd64/arm64
       ├─ GET /api/v1/trace/client 下载对应 aris
       ├─ 原子安装到 ~/.aris/bin/aris
       └─ aris trace init --host <Web Origin>

Codex
  ├─ hook(stdin JSON)
  │    └─ ~/.aris/bin/aris trace ingest
  │         ├─ 原样写入本地 hook spool
  │         ├─ 读取 transcript_path 的新增 rollout 行
  │         ├─ 原样写入本地 rollout spool
  │         ├─ 尝试同步发送当前批次
  │         └─ 失败则保留 spool，stdout 保持 Codex 所需格式
  │
  └─ ~/.codex/sessions/.../rollout-*.jsonl
       └─ 每次 Hook 触发时读取上次 offset 之后的完整 JSONL 行

POST /api/v1/trace/event
  └─ TraceIngestHandler
       ├─ 校验 session_id 和 source
       ├─ upsert traces
       ├─ 以幂等键批量插入 hook / rollout records
       ├─ 更新 Trace 的 session_meta、model、cwd、source 等摘要
       └─ Stop / task_complete 后更新生命周期状态

GET /api/v1/trace/...
  ├─ 原始记录时间线：按 server sequence 升序
  └─ 对话视图：rollout 优先解析，Hook fallback 补齐
```

### 5.1 命令与代码边界

Go 可执行入口按服务端和客户端拆分：

```text
cmd/
  server/
    main.go
    root.go
    server.go
    database.go
    object.go
    lint.go
  client/
    main.go
    root.go
    trace.go

internal/tracecli/
  config/
  init/
  ingest/
  codex/
  spool/
```

`cmd/server` 承接全部存量命令，构建为 `aris-proxy-api`。`cmd/client` 只注册 `trace init` 和 `trace ingest`，构建为 `aris`。客户端包不得导入服务端 `cmd`、数据库模型、Bootstrap、Router 或 Web embed 包；Go 只链接实际传递依赖，因此客户端不会携带 DB、Server、lint 等命令。

构建命令：

```text
go build -o aris-proxy-api ./cmd/server
go build -o aris ./cmd/client
```

### 5.2 客户端构建与下载

构建阶段使用 `CGO_ENABLED=0` 交叉编译四个客户端文件：

```text
aris-darwin-amd64
aris-darwin-arm64
aris-linux-amd64
aris-linux-arm64
```

这些文件作为独立只读产物放入运行镜像 `/app/trace-client/`，不嵌入主服务二进制。下载 Handler 只根据 `(os, arch)` 白名单映射到固定文件名，不接受文件路径或文件名参数。

安装脚本通过 `uname -s` / `uname -m` 映射平台，下载到 `~/.aris/bin` 下的临时文件，校验请求成功后设置 `0700` 并原子替换 `~/.aris/bin/aris`。下载失败不得覆盖已有客户端。安装完成后执行：

```text
~/.aris/bin/aris trace init --host "<当前 Web Origin>"
```

`--host` 表示服务 Origin，不包含 `/api/v1`。客户端仅接受 `http` / `https` URL 并移除尾部斜杠；健康检查请求 `<host>/health`，管理 API 从 `<host>/api/v1` 派生。

### 5.3 `aris trace init` 四步向导

`trace init` 必须在 TTY 中运行，采用非全屏轻量向导，显示 `[1/4]` 至 `[4/4]`、当前状态和可操作错误：

1. **连接服务器**：显示并请求 `--host`，使用短超时验证 `/health`；失败允许重试，不自动切换 Host。
2. **选择 Agent**：首期只显示并选择 `Codex`，不提前实现通用插件体系。
3. **配置 API Key**：终端隐藏输入，通过 `GET /api/v1/trace/client/check` 验证 ProxyAPIKey；验证接口成功时返回 `204 No Content`，不产生 Trace 记录。成功后原子写入 `~/.aris/trace/config.json`，目录 `0700`、文件 `0600`。重复初始化时允许保留已有 Key 或输入新 Key。
4. **配置 Hook**：修改 `~/.codex/hooks.json` 前创建备份，保留非 Aris 配置，并为所有支持事件注册绝对命令 `~/.aris/bin/aris trace ingest`。重复初始化更新已有 Aris Hook，不重复追加。

完成后打印配置路径，并明确提示用户进入 Codex `/hooks` 手动审核和信任新增 Hook；未批准前 Codex 不会执行 Hook。API Key 不进入命令参数、Codex 配置、日志或成功提示。

### 5.4 `aris trace ingest` 本地可靠上报

Hook 是同步执行的，Codex 会等待命令退出；`trace ingest` 按以下顺序处理：

1. 读取 stdin 全量内容并识别 Hook 事件；无法解析时将诊断写入受限错误日志，但不阻断 Codex。
2. 为每个 `session_id` 维护本地 spool，记录客户端递增 `sequence`、稳定 spool ID、`source`、原始 payload 和 rollout 行号。
3. 在任何网络请求前持久化当前 Hook 和新增 rollout 记录。
4. 使用临时文件 + 原子 rename 避免半条记录，使用文件锁保护同一 session 的 offset 和 spool。
5. 从全局 spool 按最老优先选择最多 500 条且不超过 4 MiB 的批次，以 5 秒总超时调用 `POST /api/v1/trace/event`，避免 backlog 或超大 payload 长时间阻塞 Codex。
6. 服务端逐条返回 `accepted` / `duplicate` / `rejected`；客户端只删除 `accepted` 和 `duplicate`。`rejected` 记录移入隔离区，避免永久阻塞后续队列。
7. 后续任意 Session 的 Hook 都优先重试全局 pending spool；`Stop` 在相同 5 秒预算内做最后一次发送。网络长期不可用时仍以本地 spool 为准。
8. stdout 严格遵守 Codex 契约：只有 `Stop` 输出 `{}`，其他事件输出空；所有错误路径退出码均为 `0`。

本地布局：

```text
~/.aris/
  bin/aris
  trace/
    config.json
    spool/
    state/
    rejected/
    logs/
```

`trace/` 下目录使用 `0700`，记录、状态和日志文件使用 `0600`。日志不得包含 API Key、完整 payload 或服务端响应正文，按日保留 7 天；rejected 记录保留 7 天。pending spool 不按年龄删除，设置 256 MiB 全局硬上限；达到上限后保留所有既有未确认记录、停止接收新记录并写入不含 payload 的本地错误，仍保持 fail-open。

### 5.5 Rollout 增量读取

Hook payload 中的 `transcript_path` 只在本机有效，因此由 `trace ingest` 读取。每个 transcript 路径维护 inode、字节 offset 和行号：

1. 首次看到路径时从第 1 行读取，之后从已持久化 offset 继续读取。
2. 只接受完整且以换行结束的 JSON 行；文件末尾半行留到下一次读取。
3. 每条 rollout 记录携带 `transcript_line`，原始 JSON 行作为 payload 上报。
4. 文件 inode 变化或 size 小于 offset 时视为替换或截断，从头重扫，并由服务端幂等键消除重复。
5. transcript 不存在、无权限或格式未知时，不阻止 Hook 事件上报；Trace 标记 `rollout_status`，对话视图降级使用 Hook fallback。

## 6. 服务端数据模型

### 6.1 `traces`

沿用现有 Trace 表，增加可用于 rollout 重建的摘要字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGINT PK | 平台 Trace ID |
| `agent` | VARCHAR | 首期固定 `codex` |
| `session_id` | VARCHAR UNIQUE | Codex session id |
| `api_key_name` | VARCHAR | API Key owner |
| `user_id` | BIGINT | 用户 owner |
| `model` | VARCHAR | 最近一次已知模型 |
| `cwd` | VARCHAR | 会话工作目录 |
| `source` | VARCHAR | 保留现有字段，保存 SessionStart 的 source 字符串；复杂来源对象放入 `metadata` |
| `status` | VARCHAR | `active` / `done` / `aborted` |
| `metadata` | JSONB | cli version、originator、provider、git、rollout 状态等 |
| `created_at` | TIMESTAMP | 首次收到记录时间 |
| `updated_at` | TIMESTAMP | 最近收到记录时间 |

`transcript_path` 不作为服务端可访问路径使用；如需审计，保存脱敏后的路径或 basename 到 metadata，完整路径仍只存在受 owner 保护的原始 payload 中。

### 6.2 `trace_events` / 现有 events 表扩展

为保持现有 API 和代码结构，优先扩展现有事件表，不另建第二套消息表：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGINT PK | 服务端插入顺序，作为展示 sequence |
| `trace_id` | BIGINT | 关联 Trace |
| `session_id` | VARCHAR | Codex session id |
| `source` | VARCHAR | `hook` / `rollout` |
| `record_type` | VARCHAR | `hook_event` / `session_meta` / `turn_context` / `response_item` / `event_msg` |
| `event` | VARCHAR | Hook event name 或 rollout `payload.type` |
| `turn_id` | VARCHAR NULL | 归属 turn |
| `call_id` | VARCHAR NULL | 工具调用及结果关联键 |
| `transcript_line` | BIGINT NULL | rollout 原文件行号 |
| `client_sequence` | BIGINT | 本地 spool 生成的递增序号 |
| `dedup_key` | VARCHAR UNIQUE | 幂等键 |
| `payload` | JSONB | 完整原始 JSON |
| `created_at` | TIMESTAMP | 服务端接收时间 |

索引：`(trace_id, id)`、`(trace_id, turn_id, id)`、`(trace_id, call_id)`。唯一约束使用 `dedup_key`，不能只依赖 `session_id + event`，因为同一事件类型会在一个会话中出现多次。

建议的幂等键：

```text
hook:<session_id>:<client_sequence>
rollout:<session_id>:<transcript_line>:<sha256(raw_line)>
```

如果本地 spool 因 fork/resume 重置 sequence，rollout 使用行号 + 内容 hash；Hook 使用持久化的稳定 spool ID 加 sequence，避免不同 spool 相互覆盖。

### 6.3 上报 DTO

上报接口改为批量 envelope，固定字段用于校验和索引，原始 JSON 使用 `sonic.NoCopyRawMessage` 或等价的自定义反序列化保留：

```json
{
  "session_id": "019f...",
  "transcript_path": "/Users/.../rollout-...jsonl",
  "records": [
    {
      "source": "hook",
      "client_sequence": 18,
      "hook_event_name": "PostToolUse",
      "turn_id": "019f...",
      "payload": { "...": "完整原始 hook JSON" }
    },
    {
      "source": "rollout",
      "transcript_line": 42,
      "payload": { "timestamp": "...", "type": "response_item", "payload": {} }
    }
  ]
}
```

服务端不得将完整 payload 反序列化到有限字段结构后再序列化，否则 Codex 新字段会再次丢失。Huma 请求 DTO 仍使用明确的 Body 结构，原始内容作为 JSON raw message 字段承载。

## 7. 对话视图重建

新增 Trace conversation query，返回按 turn 分组的只读投影；不将投影持久化为 Message/Tool。

### 7.1 Turn 归组

1. 以 rollout `turn_id` 为主键。
2. `task_started` 建立 turn 起点，`task_complete` 或 `turn_aborted` 建立终点。
3. 没有 `turn_id` 的 `session_meta` 归入会话头；无 turn 的全局 event 作为 Trace-level event。
4. Hook 记录仅在没有对应 rollout 内容时进入 fallback，避免 `UserPromptSubmit` 与 `event_msg/user_message` 双显。

### 7.2 消息选择和去重

优先级从高到低：

1. `event_msg/user_message.message` 作为 user message。
2. `event_msg/agent_message.message` 作为 assistant message。
3. 若对应 event_msg 不存在，从 `response_item/message` 的 `role` 和 content blocks 提取。
4. `response_item/reasoning.summary` 或 `event_msg/agent_reasoning.text` 作为 reasoning summary；原始推理内容为空或加密时，不伪造明文。
5. Hook `prompt` 和 `last_assistant_message` 仅在 rollout 没有相同 turn/content 时作为 fallback。

重复判断使用 `turn_id + role + normalized content + source priority`，不能删除原始记录，只在 projection 层隐藏重复项。

### 7.3 工具调用和结果

统一输出 `ToolCall`：

| 来源 | 调用 | 参数 | 结果 |
|------|------|------|------|
| `response_item` | `function_call` | 二次解析 `arguments` | `function_call_output.output` |
| `response_item` | `custom_tool_call` | 原始 `input` | `custom_tool_call_output.output` |
| `response_item` | `web_search_call` | `action` | `event_msg/web_search_end` 或原始缺失状态 |
| `event_msg` | `mcp_tool_call_end` | `invocation.arguments` | `invocation.result` |
| Hook fallback | `PreToolUse` | `tool_input` | `PostToolUse.tool_response` |

1. 首选 `call_id`；Hook fallback 使用 `tool_use_id`。
2. 只有调用没有结果时，投影状态为 `pending`，不能丢弃调用。
3. 只有结果没有调用时，投影状态为 `orphaned`，保留结果供审计。
4. `patch_apply_end`、`mcp_tool_call_end` 等 UI 事件作为工具结果补充，不与同一 `call_id` 生成第二个工具调用。

### 7.4 投影返回结构

概念结构如下：

```text
TraceConversation
  traceId
  sessionId
  metadata
  turns[]
    turnId
    model
    startedAt / completedAt
    status: running | completed | aborted
    items[]
      kind: message | reasoning | tool_call | tool_result | lifecycle | error
      role: user | assistant | developer | tool
      content
      callId
      toolName
      arguments
      output
      source: rollout | hook
      recordIds[]
```

`recordIds` 让 UI 可以从结构化项目跳回原始事件，便于排查解析或上报问题。

## 8. API 与 UI

### 8.1 Trace 数据 API

保留既有接口：

- `POST /api/v1/trace/event`：API Key Bearer 鉴权；兼容单条 Hook 上报，并扩展为批量 Hook + rollout records。
- `GET /api/v1/trace/list`：JWT 鉴权的 Trace 列表。
- `GET /api/v1/trace`：JWT 鉴权的 Trace 元数据和记录统计。
- `GET /api/v1/trace/event/list`：JWT 鉴权的原始记录时间线，返回 source、record type、turn/call/line 信息。

新增：

- `GET /api/v1/trace/conversation?id=<trace_id>`：JWT 鉴权，返回结构化 Trace conversation projection。
- `GET /api/v1/trace/client/check`：API Key Bearer 鉴权，只验证 Key 和用户状态，不产生 Trace 数据。

所有查询接口沿用 owner 隔离和 `LimitUserPermissionMiddleware`。批量上报中单条坏记录不得回滚同批其他合法记录，响应为每条记录返回 `accepted` / `duplicate` / `rejected`。

### 8.2 客户端下载 API

新增：

```text
POST /api/v1/trace/client/ticket
GET  /api/v1/trace/client?os=darwin&arch=arm64
```

`POST /trace/client/ticket` 使用现有 JWT 鉴权，要求至少 `user` 权限，并在 JWT Middleware 之后按 `CtxKeyUserID` 使用 Redis 令牌桶限流：容量为 3、补充周期为 1 分钟，允许初始突发 3 次，长期平均速率为 `3 req/min`。服务端用 `crypto/rand` 生成高强度随机票据，Redis 只保存 `SHA-256(ticket)` 对应的 user ID 和有效期，TTL 为 10 分钟。响应返回明文票据和过期时间；明文不写日志。

`GET /trace/client` 使用 `Authorization: Bearer <download-ticket>`。下载 Middleware 计算票据哈希后通过 Redis `GETDEL` 或等价 Lua 原子校验并消费；票据过期、无效或重复使用均拒绝。请求的 `(os, arch)` 只允许：

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

成功响应设置：

```text
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="aris"
Cache-Control: no-store
```

限流只在票据签发时扣减；每张票据只能下载一次，令牌桶的即时突发上限为三个下载机会。下载接口不接收 user ID、文件名或文件路径。

### 8.3 UI

Trace 详情页分成两个视图：

1. **Conversation**：按 turn 展示 user、assistant、reasoning summary、tool call/result 和生命周期状态。
2. **Raw records**：按服务端 sequence 展示 Hook / Rollout 原始记录，可展开 JSON，并显示 transcript line、turn id、call id、dedup source。

解析失败、rollout 缺失、Hook fallback 等状态必须在 UI 中明确显示，不能让用户误以为数据完整。

安装对话框不再接收 Trace URL 或 API Key，也不再生成完整 Hook shell。用户点击“复制”时，前端必须通过统一 `api-client` 请求单次下载票据，再以 `window.location.origin` 生成短安装脚本。脚本负责平台探测、携带票据下载 `aris`、原子安装，并执行 `aris trace init --host <origin>`。票据签发失败时不得复制旧脚本；API Key 只在终端向导中输入。

## 9. 错误处理与安全

1. `trace ingest` 永远 fail-open；配置、磁盘、锁、网络、transcript 或解析错误都不得改变 Codex 行为。
2. 下载票据使用 `crypto/rand` 生成，Redis 只保存 SHA-256 哈希；票据有效期 10 分钟且必须原子单次消费。
3. JWT、下载票据和 API Key 不得出现在日志、URL query、Hook 参数、payload、spool 或错误消息中。
4. 下载 Handler 使用静态 `(os, arch) → 文件` 映射，不拼接用户输入路径，避免路径穿越。
5. API Key 配置和本地 Trace 数据仅当前用户可读；所有写入采用受限权限、临时文件和原子 rename。
6. 服务端单条坏记录不应使同一批次其他记录丢失；客户端隔离永久 rejected 记录，不反复阻塞队列。
7. payload 可能包含系统提示、用户隐私、工具参数、命令和文件内容。查询必须执行现有 owner 隔离，普通日志不得打印 payload。
8. rollout 解析器遇到未知 `type` 或 `payload.type` 时保存原始记录并标记 `unknown`，不能因版本升级导致整个会话不可见。
9. Host 只允许 `http` / `https`，客户端不会根据服务端响应访问另一个任意内部地址；所有 Trace 请求只发送到用户通过 Web 脚本传入的同一 Origin。

## 10. 测试策略

### 10.1 客户端单元测试

1. `cmd/client` 只包含 `trace init` 和 `trace ingest`；`cmd/server` 保留全部存量命令。
2. 四步向导覆盖 Host 校验、连接重试、Codex 选择、隐藏密钥输入、非 TTY 拒绝和完成提示。
3. 配置原子写入且目录/文件权限分别为 `0700` / `0600`；覆盖时不产生可读临时文件。
4. Codex Hook 修改前备份，保留非 Aris Hook，注册全部事件，重复初始化不重复追加。
5. 模拟 API 不可用，断言 `trace ingest` 退出码为 `0`、stdout 满足 Stop 契约、spool 保留记录。
6. API 恢复后再次触发 ingest，断言 pending 记录重发；服务端确认后才删除。
7. 并发触发 `PreToolUse` / `PostToolUse`，断言 spool 不出现半条记录、sequence 冲突或 offset 回退。
8. 使用 fixture rollout 文件，断言首次读取、后续增量、末尾半行、文件截断和 inode 变化。
9. 日志断言不包含 API Key、完整 payload、JWT 或下载票据。

### 10.2 服务端单元测试

1. 票据使用安全随机值、Redis 哈希存储、10 分钟 TTL 和原子单次消费。
2. 同一 JWT 用户可即时签发三次，未补充令牌前第四次返回 `429`；令牌以每分钟三个的速率连续补充，不同用户相互独立。
3. 过期、伪造或重复使用的票据无法下载。
4. 四种白名单平台返回对应文件；未知 OS/架构和路径注入参数被拒绝。
5. 下载响应包含正确 Content-Type、Content-Disposition 和 no-store；票据及文件路径不进入日志。
6. API Key 检查接口只做鉴权，不创建 Trace/Event。
7. 上报 Handler 保存未知 Hook 和 rollout 字段；混合批次 owner 正确，重复 dedup key 不重复插入。
8. `Stop` 保存记录并标记 done；后续迟到的 rollout 仍可保存。
9. rollout parser 覆盖四种 envelope、消息、reasoning、function/custom/web/MCP 工具、token、error 和未知类型。
10. conversation projection 按 turn 分组、按 call_id 关联、去重，并用 Hook fallback 补齐。

### 10.3 E2E 与构建验证

1. JWT 换取票据，下载对应平台产物；同一票据第二次下载失败。
2. 安装脚本正确映射 Darwin/Linux 与 amd64/arm64，失败时不覆盖已有 `aris`。
3. `aris trace init --host <origin>` 生成 `0600` 配置和幂等 Codex Hook，并输出手动批准提示。
4. 模拟完整 Codex Hook + rollout 序列，验证 pending 重试、服务端去重、Trace 状态和 Conversation 投影。
5. 一个 Trace 的对话视图包含 user、assistant 和同一 `call_id` 下的完整工具调用/结果；`recordIds` 可定位原始记录。
6. 分别执行 `go build ./cmd/server` 和 `go build ./cmd/client`，并构建四个平台客户端，确认客户端帮助中没有服务端命令。
7. 执行 Trace 聚焦测试、全量 lint/test、前端 lint/build；浏览器验证复制脚本时才签发票据，失败不复制旧内容。

## 11. 实施分期

### Phase 1：命令拆分与客户端安全分发

- 将存量 Cobra 命令迁入 `cmd/server`，保持现有命令行为。
- 建立只含 `trace init` / `trace ingest` 的 `cmd/client`。
- 增加四平台构建、镜像只读产物目录和 Makefile 目标。
- 实现 JWT 票据签发、用户维度 `3 req/min`、单次消费和 `/trace/client` 下载。
- Web 改为按需签发票据并生成短安装脚本。

### Phase 2：客户端初始化与可靠采集

- 实现四步 `trace init` 向导、API Key 检查、受限配置和 Codex Hook 幂等修改。
- 实现 `trace ingest` 的 fail-open stdout/exit 契约。
- 引入本地 spool、文件锁、原子状态和 rollout 增量读取。
- 扩展批量上报 DTO、TraceEvent 模型和幂等存储。

### Phase 3：Rollout parser 与 conversation projection

- 实现四类 envelope parser、turn 和 call_id 关联。
- 实现 rollout 优先、Hook fallback 的去重投影。
- 增加 conversation API 和单元/E2E 测试。

### Phase 4：UI 展示与完整验收

- Trace 详情增加 Conversation / Raw records 双视图。
- 展示数据完整性、解析失败和 fallback 状态。
- 完成四平台构建、CLI、API、Web 和浏览器端到端验证。

## 12. 验收标准

1. `aris-proxy-api` 从 `cmd/server` 构建且保留所有存量命令；`aris` 从 `cmd/client` 构建且只含 `trace init` / `trace ingest`。
2. 服务镜像内含 Darwin/Linux amd64/arm64 四个独立客户端文件，主服务通过固定映射返回正确文件。
3. 登录用户的票据签发令牌桶容量为 3、补充速率为 `3 req/min`；票据 10 分钟过期、只可使用一次，JWT 不出现在安装脚本中。
4. Web 安装脚本自动识别平台、原子安装客户端，并携带 Web Origin 执行 `aris trace init --host`。
5. 四步向导验证服务器和 API Key，保存 `0600` 配置，幂等注册全部 Codex Hook，并提示在 `/hooks` 手动批准。
6. 完成一次包含工具调用的 Codex 会话后，Trace 原始记录同时包含 Hook 和 rollout 记录。
7. 网络瞬断或 hook 进程退出时，已持久化 spool 不会静默消失；网络恢复后可重试上报。
8. 同一个会话重复上报不产生重复记录；永久拒绝的坏记录不阻塞其他记录。
9. Conversation projection 能按 turn 展示 user、assistant、reasoning 和完整工具调用/结果。
10. rollout 解析失败或字段变化时，原始记录仍可查看，Hook fallback 仍可用。
11. Hook 不改变 Codex 行为：非 Stop stdout 为空，Stop stdout 为 `{}`，所有 ingest 错误路径退出码为 `0`。
12. API Key、JWT、票据和完整 payload 不出现在日志；现有 proxy Session / Message / Tool 不受影响。

## 13. 已知限制

1. 首期只支持 Darwin/Linux 的 amd64/arm64，不支持 Windows。
2. 首期 Agent 选择只有 Codex；保留向导步骤不代表已经建立通用 Agent 插件系统。
3. Codex 的 `transcript_path` 文件格式不是稳定 Hook API；解析器必须版本宽容，无法保证未来字段语义不变。
4. Hook 触发时 rollout 文件可能尚未写完，实时 projection 可能暂时缺少末尾记录；后续 Hook 会补齐。
5. 网络长期不可用时，服务端无法实时看到本地 spool；这是 fail-open 与可靠投递之间的必要取舍。
6. 加密或未落盘的 reasoning 不会被解密或伪造，只展示 Codex 实际提供的摘要或原始状态。
7. 客户端随服务镜像发布，不包含独立自动更新机制；升级通过重新运行 Web 安装脚本完成。
