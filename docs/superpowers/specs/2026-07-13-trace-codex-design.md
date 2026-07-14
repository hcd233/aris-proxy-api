# Trace 功能设计：在 AI 应用中注入 hooks 捕获 session 消息并上报（codex 先行）

## 1. 背景

`aris-proxy-api` 是 LLM 代理网关 + 管理后台，已经完整记录"经代理的 LLM API 对话"（OpenAI / Anthropic 跨协议转换 → `sessions` / `messages`）。但当一个 agent 应用（如 Codex CLI）在本地运行时，它内部的 **agent 级运行信息**（用户提示、工具调用编排、subagent、compact、会话生命周期）并不会经由代理流量体现出来。

本功能的目标：提供一套 **trace（观测）能力**，通过在 AI 应用中注入 hooks 捕获其运行期事件并上报到本平台，使平台能统一查看、检索 agent 运行，与现有 LLM 代理会话能力平级。

首期只支持 **Codex CLI**（v0.124+ hooks 已稳定）。Claude Code / OpenCode 等后续按同一套上报端点 + 各自 hook 扩展。

## 2. 关键决策（已与用户澄清）

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 上报目标 | 本平台新建 trace 存储，与现有 session 能力平级 |
| 2 | 捕获策略 | 事件流实时上报（每个 hook 事件 POST 到 proxy） |
| 3 | Hook 载体 | Shell 脚本（依赖 `jq` + `curl`） |
| 4 | 鉴权 | 复用现有 API Key（`Authorization: Bearer <key>`） |
| 5 | 安装方式 | 前端生成 `codex-trace-setup.sh`，沿用 models 导出到 codex 的体验 |
| 6 | 存储模型 | 新建独立 `traces` & `events` 表 |
| 7 | 与 proxy session 关系 | **完全正交**（v1 不关联，后续可按 owner + 时间软关联） |
| 8 | 接口命名 | 沿用 `/api/v1/<资源>` + 子路径动作风格（见 §6） |

## 3. 领域边界（重要）

两层**正交**概念，必须严格区分：

- **Proxy Session（既有域）**：一次经 aris 代理的 LLM API 对话，落在 `sessions` / `messages`，归属 api_key owner。
- **Trace（新域）**：从 agent 应用（codex）视角看到的"一次 agent 运行"，由 hooks 捕获生命周期事件。一个 agent trace 内部可能发起多次 LLM 调用 → 对应多个 proxy session。两者 v1 不建立外键或归属关系。

为避免命名混淆，新域**不使用 session 字样**：
- 顶层实体 = `traces`（一次 agent 运行，codex `session_id`）
- 子实体 = `events`（运行内的单个 hook 事件）

UI 单列 **Agent Traces**，不进现有 Sessions 页。查询中"重建 session 消息"的说法一律改为**"重建 agent 运行时间线"**——trace 展示的是 agent 级 prompt / tool / assistant 编排，不是 LLM API message。

## 4. Codex Hooks 能力调研（事实依据）

Codex CLI v0.124+ hooks 已 GA 稳定，通过 `~/.codex/hooks.json` 或 `config.toml [hooks]` 声明，支持以下事件（scope 见备注）：

| 事件 | scope | matcher 作用于 | 关键输入字段 |
|------|-------|---------------|--------------|
| `SessionStart` | 会话级 | `source`（startup/resume/clear/compact） | `session_id`, `model`, `cwd`, `source`, `transcript_path` |
| `UserPromptSubmit` | turn 级 | 忽略 | `prompt`（用户文本） |
| `PreToolUse` | turn 级 | `tool_name`（Bash/apply_patch/MCP…） | `tool_name`, `tool_use_id`, `tool_input` |
| `PostToolUse` | turn 级 | `tool_name` | `turn_id`, `tool_name`, `tool_use_id`, `tool_input`, `tool_response` |
| `Stop` | turn 级 | 忽略 | `last_assistant_message` |
| `SubagentStart` / `SubagentStop` | subagent 级 | `agent_type` | `agent_id`, `agent_type`, `last_assistant_message`(Stop) |
| `PreCompact` / `PostCompact` | turn 级 | `trigger`（manual/auto） | `trigger` |

**每个 command hook 通过 stdin 接收一份 JSON**，含公共字段 `session_id` / `transcript_path` / `cwd` / `hook_event_name` / `model` / `permission_mode`，turn 级事件额外含 `turn_id`。

**信任模型**：新 hook 首次需在 codex 内 `/hooks` 审核信任后才能运行（按 hook 定义 hash 记录）。setup 脚本须提示用户首次运行后执行信任。

**关键约束（fail-open 要求）**：本功能仅做观测，绝不拦截或改变 agent 行为。
- `SessionStart` 的 stdout 纯文本会被注入为 developer context —— 我们的 hook **必须输出空**，避免污染对话。
- `Stop` 期望 stdout 为 JSON —— 我们的 hook 输出 `{}`。
- `PreToolUse` / `PermissionRequest` 返回不支持字段会 fail-closed（整段输出被丢弃并报错）—— 我们的 hook 对这些事件**不返回任何 stdout 控制字段**。
- hook 是同步的：codex 会等待进程退出。为保证不阻塞 agent turn，hook 必须把上报放到**后台**并立即 `exit 0`。

## 5. 数据流

```
codex
  └─ hook(stdin JSON)
       └─ codex-hook.sh
            ├─ 解析 hook_event_name（jq）
            ├─ 后台 curl -sS -X POST $TRACE_URL \
            │     -H "Authorization: Bearer $API_KEY" -d @- &
            └─ exit 0   （stdout 保持空 / Stop 输出 {}）
                         │
                         ▼
              POST /api/v1/trace/event
                         │  APIKeyMiddleware 鉴权 → 取 CtxKeyAPIKeyName / CtxKeyUserID
                         ▼
                  TraceIngestHandler
                    ├─ SessionStart → upsert traces（建/续 agent 运行）
                    ├─ 其余事件    → insert events（同 session_id 归到对应 trace_id，按 id 自增排序）
                    └─ Stop        → traces.status = done
```

查询侧（Web/管理后台，JWT 鉴权、owner 隔离）：
```
GET /api/v1/trace/list        → 列出当前 owner 的 traces（分页 + 过滤）
GET /api/v1/trace             → trace 详情（query trace_id）
GET /api/v1/trace/event/list  → 某 trace 的事件时间线（按 seq 排序）
```

## 6. 接口路径（沿用既有命名）

与 `internal/router/session.go`、`audit.go` 完全一致：`/api/v1/<资源>` 分组 + 子路径动作；列表用 `constant.RoutePathList`（`/list`），详情用 `Path: ""` 按 query id 取。
路由注册：`traceGroup := huma.NewGroup(v1Group, "/trace")`，分两组：

- **上报组**（`middleware.APIKeyMiddleware(db)`，hook 用 Bearer）：
  - `POST /api/v1/trace/event` — `OperationID: reportTraceEvent`，`Tags: [TagTrace]`
- **查询组**（`middleware.JwtMiddleware(...)` + `LimitUserPermissionMiddleware`）：
  - `GET /api/v1/trace/list` — `OperationID: listTraces`，`Path: constant.RoutePathList`
  - `GET /api/v1/trace` — `OperationID: getTrace`，`Path: ""`，query `trace_id`
  - `GET /api/v1/trace/event/list` — `OperationID: listTraceEvents`，`Path: "/event/list"`，query `trace_id`

新增常量 `TagTrace = "Trace"`（`internal/common/constant`）。

## 7. 数据模型（新建独立表）

### `traces`（一次 agent 运行）
| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGINT PK auto_increment | 平台内 trace id |
| `agent` | VARCHAR | agent 来源，首期固定 `"codex"` |
| `session_id` | VARCHAR，**索引** | codex 的 session_id（天然幂等键） |
| `api_key_name` | VARCHAR | 归属 API Key 名（来自鉴权） |
| `user_id` | BIGINT | 归属用户（来自鉴权） |
| `model` | VARCHAR | 活跃模型 slug |
| `cwd` | VARCHAR | 会话工作目录 |
| `source` | VARCHAR | startup / resume / clear / compact |
| `status` | VARCHAR | active / done |
| `metadata` | JSON | 透传扩展字段 |
| `started_at` | TIMESTAMP | 首事件时间 |
| `updated_at` | TIMESTAMP | 最近事件时间 |

索引：`(user_id, updated_at)` 供列表分页；`session_id` 唯一键用于 upsert。

### `events`（运行内事件）
| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGINT PK auto_increment | |
| `trace_id` | BIGINT FK → traces.id | |
| `session_id` | VARCHAR，**索引** | 便于跨 trace 回溯 |
| `event` | VARCHAR | hook_event_name |
| `turn_id` | VARCHAR，**可空** | codex turn id |
| `payload` | JSON | 完整 hook 输入（透传存储） |
| `created_at` | TIMESTAMP | |

事件时间线顺序直接由自增主键 `id` 决定（`id` 随插入单调递增），无需额外排序字段。索引：`(trace_id, id)`。

## 8. Hook 脚本（`codex-hook.sh`）

- 头部声明依赖：`jq`、`curl`。
- 读 stdin 全量 JSON 到变量；用 `jq` 取 `hook_event_name`。
- 构造上报：后台执行
  ```sh
  printf '%s' "$PAYLOAD" | curl -sS -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 &
  ```
  随后 `exit 0`。
- stdout 处理：除 `Stop` 输出 `{}` 外，其余事件**不输出任何内容**，确保 fail-open、不注入 context、不拦截。
- 覆盖事件：`SessionStart / UserPromptSubmit / PreToolUse / PostToolUse / Stop / SubagentStart / SubagentStop / PreCompact / PostCompact`。
- 可选兜底（v1 可不做）：上报失败时追加到 `~/.aris/trace/failed.log`，供后续重发。

## 9. 安装体验（前端，沿用 `export-codex-dialog`）

新增/扩展一个 **Trace 安装对话框**（参照 `web/src/components/export-codex-dialog.tsx`），生成 `codex-trace-setup.sh`：
- 写入 `~/.codex/hooks.json`（或 `config.toml [hooks]`），含 §8 列出的全部事件 matcher；
- 将 `codex-hook.sh` 落到 `~/.aris/trace/codex-hook.sh`，并**内嵌** `API_KEY` 与 `TRACE_URL`（默认值来自对话框的 base url + 当前用户 API Key）；
- 注释提示：首次安装后需在 codex 内执行 `/hooks` 信任新 hook。
- 对话框字段：provider/base-url（默认 `window.location.origin/api/openai/v1`）、API Key（默认 `YOUR_API_KEY`）—— 与现有 models 导出体验一致。

## 10. 模块与文件地图（实现期参考）

```
internal/
  common/constant/string.go        # + TagTrace
  dto/trace.go                     # ReportTraceEventReq / ListTraceReq / GetTraceReq / ListTraceEventsReq / *Rsp
  domain/trace/                    # aggregate + repository port（参考 domain/session）
  infrastructure/database/model/trace.go   # Trace, Event GORM models
  infrastructure/repository/trace_repository.go
  application/trace/usecase/       # ingest（upsert trace / insert event / done）/ query
  handler/trace.go
  router/trace.go                  # initTraceRouter + initTraceReportRouter
  bootstrap/modules/...            # 注册 handler / usecase（dig）
web/src/
  components/trace-install-dialog.tsx   # 生成 codex-trace-setup.sh
  app/(dashboard)/trace/...             # Agent Traces 列表 + 详情（v1 最小可用）
```

## 11. 测试

- **单测**（`test/unit/trace_*`）：`trace` repository / usecase —— SessionStart upsert、事件 insert 与 seq 递增、按 owner 列表分页、详情与事件时间线查询。
- **E2E**（`test/e2e/trace/`）：mock 一份 codex hook stdin JSON，调用 `codex-hook.sh`（或直连 `POST /api/v1/trace/event`）→ 断言 `traces` / `events` 落库正确；参照现有 E2E 工程骨架沉淀，不允许仅用 curl 跑完。

## 12. 范围与边界（YAGNI）

- v1 仅支持 codex；claude-code / opencode 后续按同一上报端点 + 各自 hook 脚本扩展。
- 仅观测，不拦截、不策略校验、不修改 agent 行为（fail-open）。
- 不上报到外部 OTel / Langfuse（已确认上报到本平台）。
- 与 proxy session **不关联**（v1）。

## 13. 已知限制与可选增强

- **事件流天然拿不到中间 assistant 推理内容**：仅 `Stop.last_assistant_message` 含助手终稿；`UserPromptSubmit.prompt` 为用户消息；`PostToolUse.tool_input/response` 为工具调用。这是"实时事件流"方案的固有取舍。
  - 可选增强（方案 C 元素，v1 **不做**）：`Stop` 时读取 `transcript_path`（JSONL 全量会话）作为一条 `transcript` 事件补全，覆盖中间消息。
- codex 版本 < 0.124 不支持 hooks，setup 脚本需检测并提示。
- 后台上报为最佳努力（best-effort），进程退出后未完成的上报可能丢失；如需强一致可加本地缓冲（§8 可选兜底）。
