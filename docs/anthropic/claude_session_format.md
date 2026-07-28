## Claude Code Session 文件格式

Claude Code（CLI）会把每一次会话完整持久化为一个 **JSONL** 文件，用于会话回放、恢复（`--resume` / `--continue` / `/resume`）与审计。本文档基于本地 `~/.claude/projects/` 下的真实 rollout 文件调研整理，字段以实际样本为准。

### 存储位置

```
~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl
```

- 目录按**项目工作目录**切分，cwd 的路径编码规则：把绝对路径中的 `/` 全部替换为 `-`。
  - 例：`/Users/centonhuang/Desktop/code/aris-proxy-api` → `-Users-centonhuang-Desktop-code-aris-proxy-api`
- 文件名即 `session_id`（UUIDv4），如 `5add3648-b40d-49f5-9a3b-fbddb140331a.jsonl`。
- 同级可能存在同名目录 `<session-uuid>/`，用于存放该会话派生的子代理数据：
  ```
  ~/.claude/projects/<encoded-cwd>/<session-uuid>/
  └─ subagents/
     ├─ agent-<agentId>.jsonl          # 子代理会话记录
     └─ agent-<agentId>.meta.json      # 子代理元数据
  ```

> 注：Claude Code 还在 `~/.claude/` 下维护若干旁路数据：`transcripts/ses_*.jsonl`（IDE/精简版转录）、`todos/<session-uuid>-agent-<agentId>.json`（TodoWrite 任务列表）、`history.jsonl`（命令行输入历史）、`session-env/<session-uuid>/`（会话环境快照）。本文重点描述主 session 文件，旁路格式见文末。

### 文件结构

- 每行是一个独立、完整的 JSON 对象，称为一条 **记录（record）**。
- 记录按时间顺序追加写入，行与行之间无逗号、无外层数组。
- 首条记录通常是 `permission-mode`（权限模式声明）或 `attachment`（SessionStart hook 注入的上下文）。
- 记录之间通过 `parentUuid` → `uuid` 构成**消息树**（而非纯线性列表），便于 fork / 分支对话。

---

## 记录通用信封（Envelope）

绝大多数记录都携带以下顶层字段：

- `type: string`

  记录类型，决定其余字段的结构。取值之一：`"permission-mode"` | `"attachment"` | `"user"` | `"assistant"` | `"file-history-snapshot"` | `"last-prompt"` | `"system"` | `"progress"`。

- `uuid: string`

  本条记录的唯一 ID（UUIDv4）。

- `parentUuid: string 或 null`

  父记录的 `uuid`，用于构成消息树。首条记录为 `null`。

- `sessionId: string`

  会话 ID（UUIDv4），与文件名一致。

- `timestamp: string`

  记录写入时间，ISO 8601 UTC，如 `"2026-07-08T15:57:16.269Z"`。

- `cwd: string`

  该记录产生时的工作目录。

- `gitBranch: string`

  当前 Git 分支，如 `"master"`。不在仓库中时可能缺省。

- `version: string`

  Claude Code 版本号，如 `"2.1.92"`。

- `userType: string`

  用户类型，通常为 `"external"`（真实用户）。

- `entrypoint: string`

  会话入口，如 `"cli"`。

- `isSidechain: boolean`

  是否为子链（子代理）记录。主会话为 `false`，子代理记录为 `true`。

- `permissionMode: string`（部分记录）

  当前权限模式，如 `"default"` | `"acceptEdits"` | `"plan"` | `"bypassPermissions"`。仅出现在 `user`（真实用户输入）和 `permission-mode` 记录上。

示例（一条 `user` 记录的顶层信封）：

```json
{
  "type": "user",
  "uuid": "7ac3faca-7133-442c-8d8f-5f44474884e7",
  "parentUuid": "5add3648-b40d-49f5-9a3b-fbddb140331a",
  "sessionId": "5add3648-b40d-49f5-9a3b-fbddb140331a",
  "timestamp": "2026-07-08T15:57:16.269Z",
  "cwd": "/Users/centonhuang/Desktop/code/aris-proxy-api",
  "gitBranch": "bugfix/dataset-preview-route-2026-07-04",
  "version": "2.1.92",
  "userType": "external",
  "entrypoint": "cli",
  "isSidechain": false,
  "permissionMode": "default",
  "message": { "role": "user", "content": "..." }
}
```

---

## 1. `permission-mode` — 权限模式声明

记录当前会话的权限模式。通常出现在会话首行，以及模式切换时。

### 字段

- `type: "permission-mode"`
- `sessionId: string`
- `permissionMode: string` — `"default"` | `"acceptEdits"` | `"plan"` | `"bypassPermissions"`

```json
{
  "type": "permission-mode",
  "permissionMode": "default",
  "sessionId": "5add3648-b40d-49f5-9a3b-fbddb140331a"
}
```

---

## 2. `attachment` — 附加上下文

注入到会话上下文、但非用户直接输入的附加内容：hook 输出、MCP 指令增量、skill 列表等。通过 `attachment.type` 细分。

### 通用字段

- `type: "attachment"`
- `attachment: object` — 附加内容对象，内部 `type` 决定细类
- 其余信封字段（`cwd` / `gitBranch` / `version` / `userType` / `entrypoint` / `isSidechain` / `parentUuid` / `sessionId` / `timestamp` / `uuid`）

### 2.1 `attachment.type = "hook_success"` — Hook 成功执行

SessionStart / PreToolUse / PostToolUse 等 hook 执行成功的回执。

- `attachment.type: "hook_success"`
- `attachment.hookName: string` — 如 `"SessionStart:startup"`
- `attachment.hookEvent: string` — 如 `"SessionStart"` | `"PreToolUse"` | `"PostToolUse"`
- `attachment.toolUseID: string` — 触发该 hook 的工具调用 ID（SessionStart 时为 hook 自身标识）
- `attachment.command: string` — 实际执行的命令
- `attachment.stdout: string` — 标准输出（常为 JSON，含 `hookSpecificOutput.additionalContext`）
- `attachment.stderr: string`
- `attachment.exitCode: integer`
- `attachment.durationMs: integer`
- `attachment.content: string` — 简化内容（常为空）

### 2.2 `attachment.type = "hook_additional_context"` — Hook 额外上下文

hook 产出的、需注入到模型上下文的附加文本。

- `attachment.type: "hook_additional_context"`
- `attachment.content: array of string` — 注入文本数组
- `attachment.hookName: string`
- `attachment.hookEvent: string`
- `attachment.toolUseID: string`

### 2.3 `attachment.type = "mcp_instructions_delta"` — MCP 指令增量

MCP server 连接 / 断开时，对系统提示词中 MCP 指令段的增量更新。

- `attachment.type: "mcp_instructions_delta"`
- `attachment.addedNames: array of string` — 新增的 MCP server 名，如 `["deepwiki"]`
- `attachment.removedNames: array of string` — 移除的 MCP server 名
- `attachment.addedBlocks: array of string` — 新增的指令文本块

### 2.4 `attachment.type = "skill_listing"` — Skill 列表

注入当前可用的 skill 列表摘要。

- `attachment.type: "skill_listing"`
- `attachment.content: string` — skill 列表正文（每行一个 skill 摘要）
- `attachment.skillCount: integer`
- `attachment.isInitial: boolean` — 是否为初始注入

---

## 3. `user` — 用户消息

既表示**真实用户输入**，也表示**工具调用结果**（工具结果以 `role=user` 的消息回传给模型）。

### 字段

- `type: "user"`
- `message: object` — 消息对象：
  - `role: "user"`
  - `content: string 或 array`
    - **字符串**：真实用户输入的文本。
    - **数组**：工具结果块，每块为 `{ "type": "tool_result", "tool_use_id": string, "content": string | array, "is_error": boolean }`。
- `promptId: string` — 提示 ID（真实用户输入时存在）
- `toolUseResult: string 或 object` — 工具结果的元信息（JSON 字符串或对象，含 `stdout` / `stderr` / `exitCode` 等）
- `sourceToolAssistantUUID: string` — 对应的 assistant 工具调用记录 UUID（工具结果时存在）
- `permissionMode: string` — 真实用户输入时携带

### 示例：真实用户输入

```json
{
  "type": "user",
  "message": { "role": "user", "content": "checkout到master分支，把本地非master分支都删了" },
  "promptId": "ac7b2272-ef2c-4663-a1cb-75d17c5fdf65",
  "permissionMode": "default"
}
```

### 示例：工具结果

```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "type": "tool_result",
        "tool_use_id": "call_00_205iqFVlW2vXjU8TTkWS3146",
        "content": "+ bugfix/api-key-name-session-2026-06-21\n  bugfix/...\n* master\n",
        "is_error": false
      }
    ]
  },
  "toolUseResult": "{\"stdout\":\"+ bugfix/...\\n* master\\n\",\"exitCode\":0}",
  "sourceToolAssistantUUID": "7ac3faca-7133-442c-8d8f-5f44474884e7"
}
```

---

## 4. `assistant` — 助手消息

模型返回的消息，内含若干内容块（thinking / text / tool_use）。

### 字段

- `type: "assistant"`
- `message: object`：
  - `role: "assistant"`
  - `id: string` — 消息 ID
  - `model: string` — 模型名，如 `"deepseek-v4-flash"`；合成消息为 `"<synthetic>"`
  - `content: array of object` — 内容块（见下）
  - `stop_reason: string` — 停止原因，如 `"tool_use"` | `"end_turn"` | `"stop_sequence"`
  - `stop_sequence: string 或 null`
  - `type: string` — 消息类型，通常为 `"message"`
  - `usage: object` — Token 用量（见下）
  - `container: object`（可选）— 容器信息（部分版本）
  - `context_management: object`（可选）— 上下文管理信息（部分版本）
- `error: object`（记录级，可选）— 错误信息
- `isApiErrorMessage: boolean`（记录级，可选）— 是否为 API 错误消息

### 4.1 内容块（content block）

每个块由 `type` 区分：

#### `type = "thinking"` — 思维链

- `type: "thinking"`
- `thinking: string` — 思考正文
- `signature: string`（可选）— 思考签名（部分模型/版本携带，用于回放校验）

```json
{ "type": "thinking", "thinking": "The user wants me to checkout to master...", "signature": "..." }
```

#### `type = "text"` — 文本输出

- `type: "text"`
- `text: string` — 文本正文

```json
{ "type": "text", "text": "我已经把本地非 master 分支都删了。" }
```

#### `type = "tool_use"` — 工具调用

- `type: "tool_use"`
- `id: string` — 调用 ID，如 `"call_00_205iqFVlW2vXjU8TTkWS3146"`，与 `user` 工具结果的 `tool_use_id` 关联
- `name: string` — 工具名，如 `"Bash"` | `"Read"` | `"Edit"` | `"Write"` | `"Glob"` | `"Grep"` | `"Task"` | `"WebSearch"`
- `input: object` — 结构化参数（对象，非 JSON 字符串）

```json
{ "type": "tool_use", "id": "call_00_205iqFVlW2vXjU8TTkWS3146", "name": "Bash", "input": { "command": "git branch", "description": "List all local branches" } }
```

### 4.2 `usage` — Token 用量

字段随模型/版本略有差异，完整形态如下：

- `input_tokens: integer`
- `output_tokens: integer`
- `cache_creation_input_tokens: integer`
- `cache_read_input_tokens: integer`
- `server_tool_use: object` — 服务端工具用量
  - `web_search_requests: integer`
  - `web_fetch_requests: integer`
- `service_tier: string 或 null` — 如 `"standard"`
- `cache_creation: object`
  - `ephemeral_1h_input_tokens: integer`
  - `ephemeral_5m_input_tokens: integer`
- `inference_geo: string 或 null`
- `iterations: array 或 null`
- `speed: string 或 null` — 如 `"standard"`

合成消息（`model: "<synthetic>"`）的 usage 各字段均为 `0` / `null`。

```json
{
  "input_tokens": 40391,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 0,
  "output_tokens": 113,
  "server_tool_use": { "web_search_requests": 0, "web_fetch_requests": 0 },
  "service_tier": "standard",
  "cache_creation": { "ephemeral_1h_input_tokens": 0, "ephemeral_5m_input_tokens": 0 },
  "inference_geo": "",
  "iterations": [],
  "speed": "standard"
}
```

---

## 5. `file-history-snapshot` — 文件历史快照

记录某次消息产生前后的文件状态快照，用于 undo / 文件回滚。

### 字段

- `type: "file-history-snapshot"`
- `messageId: string` — 关联的消息 UUID
- `isSnapshotUpdate: boolean` — 是否为增量更新
- `snapshot: object`：
  - `messageId: string`
  - `timestamp: string`
  - `trackedFileBackups: object` — 被追踪文件的备份内容，key 为文件路径，value 为备份信息

```json
{
  "type": "file-history-snapshot",
  "messageId": "7ac3faca-7133-442c-8d8f-5f44474884e7",
  "isSnapshotUpdate": false,
  "snapshot": {
    "messageId": "7ac3faca-7133-442c-8d8f-5f44474884e7",
    "trackedFileBackups": {},
    "timestamp": "2026-07-08T15:57:16.269Z"
  }
}
```

---

## 6. `last-prompt` — 最后一个提示

记录会话最后一条用户提示，便于快速定位。通常出现在文件末尾。

- `type: "last-prompt"`
- `sessionId: string`
- `lastPrompt: string` — 最后一条用户输入文本

```json
{ "type": "last-prompt", "sessionId": "5add3648-...", "lastPrompt": "checkout到master分支..." }
```

---

## 7. `system` — 系统消息

非模型产生的系统级消息，通过 `subtype` 细分。多见于子代理（`isSidechain: true`）会话。

### 7.1 `subtype = "stop_hook_summary"` — Stop hook 汇总

- `subtype: "stop_hook_summary"`
- `hookCount: integer`
- `hookInfos: array of object` — 每项 `{ command, durationMs }`
- `hookErrors: array`
- `preventedContinuation: boolean`
- `stopReason: string`
- `hasOutput: boolean`
- `level: string` — 如 `"suggestion"`
- `toolUseID: string`

### 7.2 `subtype = "turn_duration"` — 轮次耗时

- `subtype: "turn_duration"`
- `durationMs: integer` — 该轮总耗时
- `messageCount: integer` — 该轮消息数
- `isMeta: boolean`

```json
{
  "type": "system",
  "subtype": "turn_duration",
  "durationMs": 141989,
  "messageCount": 102,
  "isMeta": false
}
```

---

## 8. `progress` — 进度事件

流式 / hook 执行过程中的进度通知，用于 UI 展示，不影响模型上下文。

### 字段

- `type: "progress"`
- `data: object` — 进度数据，内部 `type` 细分：
  - `data.type: "hook_progress"` — hook 执行进度
    - `hookEvent: string` — 如 `"PostToolUse"`
    - `hookName: string` — 如 `"PostToolUse:Read"`
    - `command: string`
- `parentToolUseID: string`
- `toolUseID: string`

```json
{
  "type": "progress",
  "data": { "type": "hook_progress", "hookEvent": "PostToolUse", "hookName": "PostToolUse:Read", "command": "callback" },
  "parentToolUseID": "Read_4",
  "toolUseID": "Read_4"
}
```

---

## 子代理（Subagent）会话

主会话通过 `Task` 工具派生子代理时，子代理拥有独立的会话记录文件，位于：

```
~/.claude/projects/<encoded-cwd>/<parent-session-uuid>/subagents/agent-<agentId>.jsonl
~/.claude/projects/<encoded-cwd>/<parent-session-uuid>/subagents/agent-<agentId>.meta.json
```

### meta.json 结构

- `agentType: string` — 代理类型，如 `"general-purpose"` | `"compact"`（自动压缩代理）
- `description: string` — 派生时的任务描述

```json
{ "agentType": "general-purpose", "description": "Implement Task 3: Store pool" }
```

### 子代理 jsonl 结构

与主 session 文件**完全同构**（同样的信封与记录类型），但携带以下额外字段：

- `isSidechain: true`
- `agentId: string` — 子代理 ID（hex），如 `"ada7f770698c0fc6e"`
- `slug: string` — 会话别名（三词短语），如 `"ethereal-doodling-reddy"`
- `sessionId` 仍指向**父会话** UUID
- `parentUuid` 指向父会话中触发派生的记录

子代理文件名中的 `<agentId>` 与 `agent-` 前缀后的部分对应；`compact` 类代理文件名带 `acompact-` 前缀，如 `agent-acompact-85d77e175af651cf.jsonl`。

---

## 旁路：Transcripts（精简转录）

`~/.claude/transcripts/ses_<opaque_id>.jsonl` 是面向 IDE / 回放的精简版转录，结构与主 session 不同：

- 文件名：`ses_` + 不透明 ID（非 UUID）。
- 每行一个 JSON，顶层仅三个核心字段：
  - `type: string` — `"user"` | `"tool_use"` | `"tool_result"`
  - `timestamp: string`
  - `content: string | object | null` — 内容（`tool_use` / `tool_result` 的 `content` 在精简版中常为 `null`，详细数据以主 session 为准）
- 不含 `uuid` / `parentUuid` / `sessionId` 等关联字段，不可用于精确回放，仅作展示。

> 需要完整、可回放的会话数据时，**一律以 `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl` 为准**。

---

## 旁路：Todos（TodoWrite 任务列表）

`~/.claude/todos/<session-uuid>-agent-<agentId>.json` 存放 `TodoWrite` 工具的快照，是一个 JSON 数组：

```json
[
  { "content": "Install kuroshiro and analyzer dependencies", "status": "completed", "priority": "high", "id": "1" },
  { "content": "Test the improved Japanese conversion", "status": "completed", "priority": "medium", "id": "3" }
]
```

- `status`: `"pending"` | `"in_progress"` | `"completed"`
- `priority`: `"high"` | `"medium"` | `"low"`
- 文件名含 `agent-<agentId>`，主会话与子代理各有独立 todo 文件。

---

## 附录：一条会话的典型记录序列

```
permission-mode                    # 权限模式声明（default）
├─ attachment/hook_success          # SessionStart hook 执行回执
├─ attachment/hook_additional_context # hook 注入的 additionalContext（如 superpowers）
├─ attachment/skill_listing         # 可用 skill 列表注入
├─ attachment/mcp_instructions_delta # MCP server 指令注入
├─ user (真实输入)                  # 用户提问
├─ assistant (thinking + tool_use)  # 思考 + 工具调用
├─ user (tool_result)              # 工具结果回传
├─ assistant (text)                 # 助手文本回复
├─ file-history-snapshot            # 文件状态快照
├─ system/turn_duration             # 轮次耗时统计
├─ last-prompt                      # 记录最后一条用户提示
└─ permission-mode                  # 会话结束时的权限模式
                                   # ...（后续轮次重复 user → assistant → tool_result → assistant）
```

子代理派生时，主会话中会出现一条 `assistant` 的 `tool_use`（`name: "Task"`），其结果由子代理 `subagents/agent-<id>.jsonl` 独立记录，父会话仅保留 `tool_result` 摘要。

### 解析要点

- 用顶层 `type` 判断记录大类；`attachment` / `system` 需再用内层 `attachment.type` / `subtype` 判断细类。
- `assistant` 的 `tool_use` ↔ `user` 的 `tool_result` 通过 `tool_use.id` ↔ `tool_result.tool_use_id` 关联；记录级还可用 `sourceToolAssistantUUID` 反查父 assistant 记录。
- 消息树通过 `parentUuid` → `uuid` 重建，不要假设记录严格线性（fork / resume 会产生分支）。
- `user` 记录的 `message.content` 是**字符串**表示真实用户输入，是**数组**表示工具结果——二者共享同一 `type`，需靠 `content` 类型与 `toolUseResult` 字段区分。
- `model: "<synthetic>"` 的 assistant 记录是 Claude Code 本地合成的（如错误回执），非模型真实输出，usage 各字段为 `0`。
- 子代理记录的 `sessionId` 指向**父会话**，自身身份用 `agentId` + `slug` 标识；`isSidechain: true` 是判定子代理的可靠标志。
- Token 统计分散在每条 `assistant.message.usage` 中，无独立的 `token_count` 汇总记录（与 Codex 不同）；如需会话累计，需自行累加。
- `thinking` 块的 `signature` 字段是否存在取决于模型/版本；回放给模型时需保留 signature 以满足思维链校验。
- `file-history-snapshot` 的 `trackedFileBackups` 通常为空对象，仅在被工具改动过的文件上才有内容；用于 `/rewind` 等撤销操作。
- 部分版本的 Claude Code 会在会话首行写一条 `summary` 记录（含 `leafUuid` 与摘要文本）用于 `/resume` 列表展示；本机样本未观测到，遇此场景以实际为准。
