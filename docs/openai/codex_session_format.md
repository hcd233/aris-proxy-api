## Codex Session (Rollout) 文件格式

Codex（含 Codex CLI / Codex Desktop）会把每一次会话完整持久化为一个 **JSONL** 文件（rollout 文件），用于会话回放、恢复（resume/fork）与审计。

### 存储位置

```
~/.codex/sessions/<YYYY>/<MM>/<DD>/rollout-<ISO8601>-<session_uuid>.jsonl
```

- 目录按 `年/月/日` 三级切分。
- 文件名格式：`rollout-2026-07-09T15-52-50-019f45dd-798a-79d0-9ea2-c7446f13e324.jsonl`
  - 时间部分为会话创建的本地时间（`:` 用 `-` 代替）。
  - 末尾为 `session_id`（UUIDv7）。

### 文件结构

- 每行是一个独立、完整的 JSON 对象，称为一条 **记录（record）**。
- 记录按时间顺序追加写入，行与行之间无逗号、无外层数组。
- 首条记录通常为 `session_meta`（会话头），随后是若干轮（turn）交织的 `turn_context` / `response_item` / `event_msg`。

---

## 记录通用信封（Envelope）

每条记录都遵循相同的顶层结构：

- `timestamp: string`

  记录写入时间，ISO 8601 UTC 格式，如 `"2026-07-09T07:53:04.719Z"`。

- `type: string`

  记录类型，决定 `payload` 的结构。取值之一：`"session_meta"` | `"turn_context"` | `"response_item"` | `"event_msg"`。

- `payload: object`

  记录负载，结构由 `type` 决定。对 `response_item` 与 `event_msg`，`payload` 内部还有一个 `type` 字段（下文记为 `payload.type`）进一步细分。

示例：

```json
{
  "timestamp": "2026-07-09T07:53:04.719Z",
  "type": "event_msg",
  "payload": { "type": "task_started", "turn_id": "019f45dd-b197-...", "started_at": 1783583584, "model_context_window": 996147, "collaboration_mode_kind": "default" }
}
```

---

## 1. `session_meta` — 会话元数据

会话头，一般位于文件首行；fork/resume 场景可能出现多条。

### payload 字段

- `id: string`

  会话唯一 ID（UUIDv7），如 `"019f45dd-798a-79d0-9ea2-c7446f13e324"`。

- `session_id: string`

  同 `id`，冗余字段。

- `timestamp: string`

  会话创建时间，ISO 8601，如 `"2026-07-09T07:52:50.350Z"`。

- `cwd: string`

  会话启动时的工作目录，如 `"/Users/centonhuang/Desktop/code/aris-proxy-api"`。

- `originator: string`

  发起端标识，如 `"Codex Desktop"`、`"codex_cli_rs"`。

- `cli_version: string`

  Codex 版本号，如 `"0.142.5"`。

- `source: string 或 object`

  会话来源。普通会话为字符串（如 `"vscode"`）；子代理（subagent）派生的会话为对象：

  - `subagent: object`

    - `thread_spawn: object` — 由父线程派生

      - `parent_thread_id: string` — 父线程 ID
      - `depth: integer` — 派生深度，如 `1`
      - `agent_path: string 或 null` — agent 定义路径
      - `agent_nickname: string` — 代理昵称，如 `"Harvey"`
      - `agent_role: string` — 代理角色，如 `"worker"`

    - `other: string` — 其他来源标识，如 `"guardian"`

- `thread_source: string`

  线程来源，如 `"user"`。

- `model_provider: string`

  模型供应商标识，如 `"lvlvko"`。

- `base_instructions: object`

  基础系统提示词。

  - `text: string` — 完整系统提示词正文（可能很长）。

- `dynamic_tools: array of object`

  本会话动态注入的工具/命名空间定义。元素通常为：

  - `type: string` — 如 `"namespace"` | `"function"`
  - `name: string` — 命名空间或工具名，如 `"codex_app"`、`"automation_update"`
  - `description: string` — 描述
  - `tools: array of object` — 当 `type="namespace"` 时的子工具列表

- `git: object`

  会话启动时的 Git 状态（不在仓库中时可能缺省）。

  - `commit_hash: string` — 当前 commit，如 `"9ceefffd189987670fa7e3bf6004e556404113f2"`
  - `branch: string` — 当前分支，如 `"master"`
  - `repository_url: string` — 远程地址，如 `"git@github.com:hcd233/aris-proxy-api.git"`

- `memory_mode: optional string`

  记忆模式（启用记忆功能时出现）。

- `forked_from_id: optional string`

  若为 fork 会话，指向源会话 ID。

- `parent_thread_id: optional string`

  父线程 ID（子代理会话）。

- `multi_agent_version: optional string`

  多代理协议版本。

- `agent_nickname: optional string`

  子代理昵称。

- `agent_role: optional string`

  子代理角色。

### 示例

```json
{
  "timestamp": "2026-07-09T07:53:04.719Z",
  "type": "session_meta",
  "payload": {
    "session_id": "019f45dd-798a-79d0-9ea2-c7446f13e324",
    "id": "019f45dd-798a-79d0-9ea2-c7446f13e324",
    "timestamp": "2026-07-09T07:52:50.350Z",
    "cwd": "/Users/centonhuang/Desktop/code/aris-proxy-api",
    "originator": "Codex Desktop",
    "cli_version": "0.142.5",
    "source": "vscode",
    "thread_source": "user",
    "model_provider": "lvlvko",
    "base_instructions": { "text": "You are Codex, a coding agent ..." },
    "dynamic_tools": [ { "type": "namespace", "name": "codex_app", "description": "Tools provided by the Codex app.", "tools": [ /* ... */ ] } ],
    "git": {
      "commit_hash": "9ceefffd189987670fa7e3bf6004e556404113f2",
      "branch": "master",
      "repository_url": "git@github.com:hcd233/aris-proxy-api.git"
    }
  }
}
```

---

## 2. `turn_context` — 单轮上下文快照

每一轮（turn）开始时记录一次，描述该轮的运行参数。

### payload 字段

- `turn_id: string`

  轮次 ID（UUIDv7），如 `"019f45dd-b197-7e03-b21f-4b02e145a2b8"`。

- `cwd: string`

  该轮工作目录。

- `workspace_roots: optional array of string`

  工作区根目录列表，如 `["/Users/centonhuang/Desktop/code/aris-proxy-api"]`。

- `current_date: string`

  当前日期，如 `"2026-07-09"`。

- `timezone: string`

  时区，如 `"Asia/Shanghai"`。

- `approval_policy: string`

  审批策略，如 `"never"` | `"on-request"` | `"on-failure"` | `"untrusted"`。

- `sandbox_policy: object`

  文件系统/网络沙箱策略，按 `type` 区分：

  - `type: string` — `"read-only"` | `"workspace-write"` | `"danger-full-access"`
  - `network_access: optional boolean` — 仅 `workspace-write`，是否允许联网
  - `exclude_tmpdir_env_var: optional boolean` — 仅 `workspace-write`
  - `exclude_slash_tmp: optional boolean` — 仅 `workspace-write`

- `model: string`

  该轮使用的模型，如 `"deepseek-v4-pro"`。

- `personality: optional string`

  模型人格设定，如 `"friendly"`。

- `effort: optional string`

  推理强度，如 `"xhigh"` | `"high"` | `"medium"` | `"low"`。

- `summary: optional string`

  推理摘要模式，如 `"auto"`。

- `collaboration_mode: optional object`

  协作模式配置。

  - `mode: string` — 如 `"default"` | `"plan"`
  - `settings: object`
    - `model: string`
    - `reasoning_effort: string`
    - `developer_instructions: string` — 该模式对应的开发者指令

- `realtime_active: optional boolean`

  是否处于实时（语音）模式。

- `user_instructions: optional string`

  用户级指令（如 AGENTS.md 汇总）。

- `developer_instructions: optional string`

  开发者级指令。

- `truncation_policy: optional object`

  上下文截断策略。

  - `mode: string` — 如 `"tokens"`
  - `limit: integer` — 阈值，如 `10000`

- `permission_profile: optional object`

  权限配置档。

  - `type: string` — 如 `"disabled"` | `"managed"`
  - `file_system: optional object` — 当 `managed` 时，含 `type`（如 `"restricted"`）与 `entries` 列表（每项 `path` + `access`）
  - `network: optional string` — 如 `"restricted"`

- `file_system_sandbox_policy: optional object`

  更细粒度的文件系统沙箱规则。

  - `kind: string` — 如 `"restricted"`
  - `entries: array of object` — 每项：
    - `path: object` — `{ "type": "special", "value": { "kind": "project_roots" } }` 或 `{ "type": "path", "path": "/abs/path" }`
    - `access: string` — `"read"` | `"write"`

- `multi_agent_version: optional string`

  多代理协议版本。

### 示例

```json
{
  "timestamp": "2026-07-09T07:53:04.724Z",
  "type": "turn_context",
  "payload": {
    "turn_id": "019f45dd-b197-7e03-b21f-4b02e145a2b8",
    "cwd": "/Users/centonhuang/Desktop/code/aris-proxy-api",
    "workspace_roots": ["/Users/centonhuang/Desktop/code/aris-proxy-api"],
    "current_date": "2026-07-09",
    "timezone": "Asia/Shanghai",
    "approval_policy": "never",
    "sandbox_policy": { "type": "danger-full-access" },
    "permission_profile": { "type": "disabled" },
    "model": "deepseek-v4-pro",
    "personality": "friendly",
    "collaboration_mode": {
      "mode": "default",
      "settings": { "model": "deepseek-v4-pro", "reasoning_effort": "xhigh", "developer_instructions": "# Collaboration Mode: Default ..." }
    }
  }
}
```

---

## 3. `response_item` — 模型交互条目

会重放（replay）给模型的结构化条目，对应 Responses API 的 item。通过 `payload.type` 细分。

所有 `response_item` 通常携带：

- `internal_chat_message_metadata_passthrough: object`

  内部透传元数据。

  - `turn_id: string` — 所属轮次 ID。

### 3.1 `payload.type = "message"` — 角色消息

- `type: "message"`
- `role: string` — `"user"` | `"developer"` | `"assistant"`
- `id: optional string` — 消息 ID
- `phase: optional string` — 阶段标识
- `content: array of object` — 内容块，每块：
  - `type: string` — `"input_text"`（输入侧）或 `"output_text"`（模型输出侧）
  - `text: string` — 文本正文

```json
{
  "type": "message",
  "role": "developer",
  "content": [ { "type": "input_text", "text": "<permissions instructions> ..." } ]
}
```

### 3.2 `payload.type = "reasoning"` — 思维链

- `type: "reasoning"`
- `id: string` — 如 `"msg_resp_b679acea-..."`
- `summary: array of object` — 摘要块，每块：`{ "type": "summary_text", "text": "..." }`
- `content: array 或 null` — 原始推理内容（通常为 `null`）
- `encrypted_content: string 或 null` — 加密后的推理内容（无存储时为 `null`）

```json
{
  "type": "reasoning",
  "id": "msg_resp_b679acea-dff7-403e-bbdd-93a34717b581",
  "summary": [ { "type": "summary_text", "text": "The user wants me to download skills ..." } ],
  "content": null,
  "encrypted_content": null
}
```

### 3.3 `payload.type = "function_call"` — 工具调用

- `type: "function_call"`
- `id: string` — 条目 ID，如 `"fc_call_00_UYu6..."`
- `name: string` — 工具名，如 `"exec_command"`、`"apply_patch"`、`"codegraph_search"`、`"update_plan"`
- `arguments: string` — 参数（**JSON 字符串**，需二次解析），如 `"{\"cmd\": \"ls -la ...\"}"`
- `call_id: string` — 调用 ID，用于与 `function_call_output` 关联
- `namespace: optional string` — MCP 命名空间，如 `"mcp__openaiDeveloperDocs"`

```json
{
  "type": "function_call",
  "id": "fc_call_00_UYu6wOyk5cNtddTYx5ie2275",
  "name": "exec_command",
  "arguments": "{\"cmd\": \"ls -la .../external/\", \"justification\": \"查看当前 external 文件夹内容\"}",
  "call_id": "call_00_UYu6wOyk5cNtddTYx5ie2275"
}
```

### 3.4 `payload.type = "function_call_output"` — 工具结果

- `type: "function_call_output"`
- `call_id: string` — 对应 `function_call.call_id`
- `output: string` — 工具输出（纯文本，含 exec 元信息与 stdout）

```json
{
  "type": "function_call_output",
  "call_id": "call_00_UYu6wOyk5cNtddTYx5ie2275",
  "output": "Chunk ID: 04a989\nWall time: 0.0000 seconds\nProcess exited with code 0\n...Output:\ntotal 0\n..."
}
```

### 3.5 `payload.type = "custom_tool_call"` — 自定义工具调用

用于自由格式输入的工具（如 `apply_patch`）。

- `type: "custom_tool_call"`
- `id: string`
- `status: string` — 如 `"completed"`
- `call_id: string`
- `name: string` — 如 `"apply_patch"`
- `input: string` — 原始输入文本（非 JSON，如 patch 补丁文本）

### 3.6 `payload.type = "custom_tool_call_output"` — 自定义工具结果

- `type: "custom_tool_call_output"`
- `call_id: string`
- `output: string` — 结果文本

### 3.7 `payload.type = "web_search_call"` — 联网搜索调用

- `type: "web_search_call"`
- `id: string` — 如 `"ws_02062af2..."`
- `status: string` — 如 `"completed"`
- `action: object` — 搜索动作：
  - `type: string` — `"open_page"` | `"find_in_page"` | `"search"`
  - `url: optional string` — 目标 URL
  - `pattern: optional string` — 仅 `find_in_page`，页内匹配模式

---

## 4. `event_msg` — UI 事件 / 状态消息

面向界面展示的事件流（部分内容与 `response_item` 冗余，如 `agent_message` vs `message`）。通过 `payload.type` 细分。

### 4.1 `task_started` — 轮次开始

- `type: "task_started"`
- `turn_id: string`
- `started_at: integer` — Unix 时间戳（秒）
- `model_context_window: integer` — 模型上下文窗口 token 数，如 `996147`
- `collaboration_mode_kind: string` — 如 `"default"`

### 4.2 `task_complete` — 轮次完成

- `type: "task_complete"`
- `turn_id: string`
- `last_agent_message: string 或 null` — 该轮最后一条助手消息
- `completed_at: integer` — Unix 时间戳（秒）
- `duration_ms: integer` — 该轮总耗时（毫秒）
- `time_to_first_token_ms: integer` — 首 token 耗时（毫秒）

### 4.3 `turn_aborted` — 轮次中止

- `type: "turn_aborted"`
- `turn_id: string`
- `reason: string` — 中止原因，如 `"interrupted"`
- `completed_at: integer`
- `duration_ms: integer`

### 4.4 `user_message` — 用户输入

- `type: "user_message"`
- `message: string` — 用户输入文本
- `client_id: optional string` — 客户端 ID（UUID）
- `images: array` — 远程图片列表（无则为 `[]`）
- `local_images: array` — 本地图片列表
- `text_elements: array` — 富文本元素

### 4.5 `agent_message` — 助手消息

- `type: "agent_message"`
- `message: string` — 助手回复文本
- `phase: string 或 null` — 阶段标识
- `memory_citation: null 或 object` — 记忆引用

### 4.6 `agent_reasoning` — 助手思考

- `type: "agent_reasoning"`
- `text: string` — 思考正文

### 4.7 `token_count` — Token 用量

- `type: "token_count"`
- `info: object 或 null` — 用量信息：
  - `total_token_usage: object` — 会话累计用量
    - `input_tokens: integer`
    - `cached_input_tokens: integer`
    - `output_tokens: integer`
    - `reasoning_output_tokens: integer`
    - `total_tokens: integer`
  - `last_token_usage: object` — 最近一次请求用量（字段同上）
  - `model_context_window: integer`
- `rate_limits: object 或 null` — 限流信息：
  - `limit_id: string` — 如 `"codex"`
  - `limit_name / primary / secondary / credits / individual_limit / plan_type / rate_limit_reached_type` — 多为 `null`，按需填充

```json
{
  "type": "token_count",
  "info": {
    "total_token_usage": { "input_tokens": 3119049, "cached_input_tokens": 2985088, "output_tokens": 17079, "reasoning_output_tokens": 10553, "total_tokens": 3136128 },
    "last_token_usage": { "input_tokens": 90964, "cached_input_tokens": 90624, "output_tokens": 116, "reasoning_output_tokens": 30, "total_tokens": 91080 },
    "model_context_window": 996147
  },
  "rate_limits": { "limit_id": "codex", "limit_name": null, "primary": null }
}
```

### 4.8 `mcp_tool_call_end` — MCP 工具调用结束

- `type: "mcp_tool_call_end"`
- `call_id: string`
- `invocation: object` — 调用信息：
  - `server: string` — MCP server 名，如 `"codegraph"`
  - `tool: string` — 工具名，如 `"codegraph_context"`
  - `arguments: object` — 结构化参数
- `duration: object` — 耗时：
  - `secs: integer`
  - `nanos: integer`
- `result: object` — 结果（`Ok`/`Err` 二选一）：
  - `Ok: object` — 成功：`{ "content": [ { "type": "text", "text": "..." } ], "isError": boolean }`
  - `Err: string` — 失败：错误信息文本

### 4.9 `web_search_end` — 联网搜索结束

- `type: "web_search_end"`
- `call_id: string`
- `query: string` — 查询内容或 URL
- `action: object` — 结构同 `web_search_call.action`

### 4.10 `patch_apply_end` — 补丁应用结束

- `type: "patch_apply_end"`
- `call_id: string`
- `turn_id: string`
- `stdout: string` — 标准输出
- `stderr: string` — 标准错误
- `success: boolean` — 是否成功
- `status: optional string` — 状态标识
- `changes: object` — 逐文件变更，key 为绝对路径，value 为：
  - `type: string` — `"add"` | `"update"` | `"delete"`
  - `content: optional string` — 新增文件（`add`）的完整内容
  - `unified_diff: optional string` — 修改文件（`update`）的统一 diff
  - `move_path: optional string` — 重命名/移动目标路径

### 4.11 `thread_rolled_back` — 线程回滚

- `type: "thread_rolled_back"`
- `num_turns: integer` — 回滚的轮次数

### 4.12 `error` — 错误

- `type: "error"`
- `message: string` — 错误信息，如 `"unexpected status 403 Forbidden ..."`
- `codex_error_info: string` — 错误分类，如 `"other"`

---

## 附录：一条会话的典型记录序列

```
session_meta                       # 会话头（含系统提示、git、工具定义）
├─ event_msg/task_started          # 第 1 轮开始
├─ response_item/message(developer)# 权限、app 上下文等注入
├─ turn_context                    # 本轮运行参数
├─ event_msg/user_message          # 用户输入
├─ event_msg/agent_reasoning       # 思考（UI 侧）
├─ response_item/reasoning         # 思考（模型侧）
├─ response_item/function_call     # 工具调用
├─ event_msg/agent_message         # 助手消息（UI 侧）
├─ response_item/function_call_output  # 工具结果
├─ event_msg/token_count           # 用量统计
└─ event_msg/task_complete         # 本轮完成
                                   # ...（后续轮次重复 task_started ~ task_complete）
```

### 解析要点

- 用 `type` 判断记录大类，再用 `payload.type` 判断细类（`session_meta` / `turn_context` 无 `payload.type`）。
- `function_call.arguments` 是 **JSON 字符串**，需二次 `JSON.parse`；`custom_tool_call.input` 是**原始文本**（非 JSON）。
- `function_call` ↔ `function_call_output` 通过 `call_id` 关联；MCP 调用则用 `mcp_tool_call_end`（含结构化 `result`）。
- 轮次以 `turn_id` 归组，`task_started` / `task_complete` 界定一轮边界。
- `event_msg` 主要用于 UI 展示与统计，`response_item` 是真正重放给模型的历史；两者对同一内容可能各存一份。
