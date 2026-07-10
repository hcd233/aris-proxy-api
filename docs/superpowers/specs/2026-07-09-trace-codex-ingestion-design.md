# Trace 功能设计：从 Codex 注入 Hook 捕获 Session 消息并上报

- 日期：2026-07-09
- 状态：设计已评审，待 writing-plans
- 范围：Codex 单源闭环（最小可用 + 完整字段映射）

## 1. 背景与目标

`CONTEXT.md` 已规划 `Transcript Ingestion（会话摄取）`：对订阅制 Agentic 工具（Claude Code、Codex）流量的离线捕获，读取工具写在本地的会话文件后摄取进平台，沉淀为 `Trace（沉淀会话）`，与 `Proxy Capture` 是两条互斥的数据入口。

现状：`Trace` 目前仅是 CONTEXT.md 中的规划概念，**没有任何代码实现**；Codex 已真实提供 hooks 框架。

本设计目标：从 **Codex** 开始支持 trace——在 AI 应用中注入 hooks，捕获 session 消息并上报到 aris-proxy-api，沉淀为 `Trace` 并展示。

### 调研结论（事实基础）

- **Codex hooks 框架**：提供 `SessionStart` / `UserPromptSubmit` / `PreToolUse` / `PostToolUse` / `Stop` 等事件。每个 hook 以 `command` 形式运行，从 `stdin` 收到一个 JSON，含 `session_id`、`transcript_path`、`cwd`、`model`、`turn_id`、`hook_event_name`、`permission_mode`。
- **关键约束**：官方明确 `transcript_path` 指向的转录文件格式**不是稳定接口，可能变化**。因此解析逻辑必须收敛到服务端一处。
- **本机实测 transcript 格式**（`~/.codex/sessions/**.jsonl`）：
  - 顶层 `type`：`session_meta`、`event_msg`、`response_item`、`turn_context`
  - `response_item.payload.type`（消息类型）：`message`(role: developer/user/assistant)、`reasoning`、`function_call`、`function_call_output`、`custom_tool_call`、`custom_tool_call_output`
  - 配对字段：`function_call`/`custom_tool_call` 的 `call_id` 与 `*_output` 的 `call_id` 配对
- **复用资产**：`UnifiedMessage`（`internal/common/vo/unified_message.go`，含 `ReasoningContent`/`ToolCalls`/`ToolCallID`/`Refusal`）、`Message` 聚合（内容寻址 `checksum` 去重）、`Session` 聚合范式、dig DI、pond 异步池、`ProxyAPIKey` 鉴权、`export-codex-dialog.tsx` 生成脚本范式。

## 2. 方案决策（已澄清确认）

| 维度 | 决策 |
|------|------|
| 捕获机制 | **A：hook 转发 transcript_path，服务端读取解析**（最鲁棒，对格式变化只依赖服务端一处） |
| 触发时机 | **Stop 事件为主**：每次 turn 结束上报全量 transcript，服务端按 `session_id` upsert 保证幂等 |
| 落库模型 | **新建独立 `Trace` 聚合**，复用现有 `Message`/`UnifiedMessage` 值对象，与 `Proxy Capture` 的 `Session` 并存 |
| 鉴权与分发 | **复用 `ProxyAPIKey`**（Bearer 鉴权）+ Web 端生成"Codex 追踪器"脚本写入 `~/.codex/hooks.json` |
| 首期范围 | **Codex 单源闭环 + 完整字段映射**（用户 prompt / assistant 文本 / tool 调用结果 / reasoning 全部映射到 `UnifiedMessage`） |
| 端点路径 | **`POST /api/v1/trace/ingest/codex`**（`/v1` 前缀 + `/codex` 子路径，为后续 `ingest/claude` 等同构入口预留） |
| Web 界面 | 新增独立 **`traces` 界面**（列表展示 ingest 的 trace）+ 内含**导出 Codex tracer 脚本**功能 |

## 3. 整体链路与模块边界

```
Codex CLI (本地)
  └─ Stop hook (command 脚本, 内嵌 ProxyAPIKey)
       └─ POST /api/v1/trace/ingest/codex
            { session_id, transcript_path, cwd, model, turn_id }
                                          │
aris-proxy-api (远端)
  ├─ middleware: Bearer ProxyAPIKey 鉴权 → 注入 APIKeyOwner
  ├─ handler/trace.go  → usecase IngestTrace
  ├─ 读 transcript_path 文件 (服务端本地/挂载路径)
  ├─ converter: codex_transcript → []*UnifiedMessage (+ reasoning/tool)
  ├─ domain/trace 聚合 (Trace 持有 MessageID 列表, 复用 Message 值对象)
  ├─ repository: traces / trace_messages (按 session_id upsert, 幂等)
  └─ pond 异步池提交, 不阻塞响应

Web 端
  ├─ 新增 traces 界面 (路由 /dashboard/traces, 参考 sessions 板块结构)
  ├─ 列表展示已 ingest 的 trace (复用 session 列表/详情展示能力)
  └─ 内含「导出 Codex 追踪器」功能: 生成幂等 bash 脚本 patch ~/.codex/hooks.json
```

**关键边界决策**
- `Trace` 与 `Session` 是两条互斥入口，均归属 `APIKeyOwner`，均复用 `Message` 值对象（内容寻址去重，跨 Trace 共享）。
- 解析只在服务端一处（`internal/application/trace/converter`），本地转录格式变化只改这一处。
- 幂等：以 `session_id` 为键，每次 `Stop` 全量重解析后 upsert（先软删旧引用再重建，复用 `Session` 的引用管理思路）。

## 4. 数据模型（`Trace` 聚合 + 表结构）

### 领域层 `internal/domain/trace/`
- `Trace` 聚合：持有 `owner vo.APIKeyOwner`、`messageIDs []uint`、`metadata map[string]string`（含 `source=codex`、`session_id`、`cwd`、`model`、`cli_version`、`originator`）、`createdAt/updatedAt`。
- `Message` 直接复用 `internal/domain/conversation` 的 `Message` 聚合 + `UnifiedMessage` 值对象（内容寻址 `checksum` 去重，跨 Trace 共享——与 `Session` 同一张 `messages` 表）。
- `Tool` 复用现有 `tools` 表（Codex 的 `function_call`/`custom_tool_call` 映射为 `Tool`）。

### 表结构（新增，不改动 `sessions`/`messages`）
- `traces`：`id`、`owner_api_key_name`、`session_id` UNIQUE、`cwd`、`model`、`source`、`metadata` JSON、`created_at`、`updated_at`
- `trace_messages`：`trace_id`、`message_id`（关联表，参考 `sessions` 的 `messageIDs` 存储方式）
- `tools` 表复用（Codex 工具调用映射）

### 幂等
以 `session_id` 唯一键，每次 `Stop` 全量重解析 → 软删旧 `trace_messages` 引用 → 重建。重复上报不重复落库。

## 5. transcript → Message 映射规则（完整字段映射）

基于本机真实 transcript 实测的 `response_item.payload.type` 分类：

| Codex transcript 类型 | 映射为 | 关键字段 |
|---|---|---|
| `message` role=developer | `Message`(role=system) | `content[].input_text` → `UnifiedMessage.Text` |
| `message` role=user | `Message`(role=user) | `content[].input_text`（用户 prompt） |
| `message` role=assistant | `Message`(role=assistant) | `content[].output_text` → Text |
| `reasoning` | `Message`(role=assistant) + `ReasoningContent` | `summary[].summary_text` → 推理内容 |
| `function_call` / `custom_tool_call` | `Tool`(ToolCallID=call_id, name, arguments/input) | 配对 `call_id` |
| `function_call_output` / `custom_tool_call_output` | `Tool` 执行结果 / `role=tool` 消息 | `output` → 结果；按 `call_id` 配对 |
| `session_meta` | `Trace.metadata` | id(=session_id)、cwd、model、cli_version、originator |
| `event_msg` / `turn_context` | 暂不落库（用于 turn 顺序/上下文，可选） | — |

**配对逻辑**：用 `call_id` 把 `function_call` 与 `function_call_output` 配对成一条 `Tool`（含参数+结果）；transcript 按文件顺序天然有序，直接顺序转 `Message`/`Tool`。
**去重**：`Message`/`Tool` 走现有 `checksum` 内容寻址，同一内容只存一份，`Trace` 持引用。

## 6. 接口与文件清单（首期交付）

### 后端新增
- `internal/router/trace.go`：
  - `POST /api/v1/trace/ingest/codex`（接收 hook 上报）
  - `GET /api/v1/trace`（列表）
  - `GET /api/v1/trace/{id}`（详情，复用 session 详情缓存思路）
- `internal/handler/trace.go` + `internal/application/trace/{port,usecase,query}`
- `internal/domain/trace/aggregate` + `internal/infrastructure/repository/trace_repository.go`
- `internal/application/trace/converter/codex_transcript.go`（解析映射，唯一格式依赖点）
- DI（`container.go`）、pond 任务（`SubmitTraceIngestTask`）、中间件复用 `ProxyAPIKey` 鉴权

### 前端新增
- `web/src/app/(dashboard)/traces/`：列表页 + 详情页（参考 `sessions` 板块）
- `web/src/components/trace/export-codex-tracer.tsx`：导出 Codex 追踪器（生成幂等 bash 脚本 patch `~/.codex/hooks.json`，参考 `export-codex-dialog.tsx` 范式）
- i18n（zh/en/ja）补充 trace 与"Codex 追踪器"文案

### 测试（E2E 沉淀到 `test/e2e/trace_ingest/`）
- hook 脚本调 `ingest/codex` → 校验 `Trace`+`Message`+`Tool` 落库
- 幂等：重复上报不重复落库
- `ProxyAPIKey` 鉴权：无效 key 被拒绝

### CONTEXT.md 回写
把已有的 `Trace`/`Transcript Ingestion` 概念从"规划中"补完为已实现的领域术语（明确 Trace 与 Session 的互斥入口关系、Codex ingest 路径）。

## 7. 风险与权衡
- **transcript 格式不稳定**：所有解析收敛到 `converter/codex_transcript.go` 单点，格式变更只改此处；hook 只传稳定元数据（`session_id`/`transcript_path` 等），不解析内容。
- **服务端需能读 transcript_path**：transcript 文件在用户本地，服务端需能访问该路径（同机部署 / 挂载 / 或 hook 改为上传文件内容）。首期假设为可访问路径；若跨机，后续可扩展为 hook 直接上传 transcript 内容。
- **YAGNI**：首期仅 Codex 单源，预留 `/codex` 子路径与 converter 抽象以支持后续 Claude Code 等同构接入，但不实现。
