# Agent 框架调研报告：adk-go / eino / trpc-agent-go

## 1. 背景

`aris-proxy-api` 是 LLM 代理网关 + 配套管理后台，已经具备完整的 LLM 代理链路（OpenAI / Anthropic 跨协议转换）、session/audit/api-key/endpoint/model 管理、定时任务、cron 分布式锁等基础设施。当前的 LLM 调用以"一次性请求 → 响应"为主，缺少更复杂的智能行为能力，例如：

- 多步推理 / 工具调用（让模型自己决定调哪些内部能力，而不是把所有逻辑前置到 Web 前端）。
- 运维 / 业务场景下的自动化 agent（think-extract 之外的 cron 自动化、告警归因、日志分析）。
- 未来扩展智能查询助手（让用户用自然语言查 session / audit / model 数据）。

本调研旨在为后续引入一个 Go Agent 框架提供事实依据。**本次交付仅为调研报告 spec，不涉及具体代码改动、不生成 plan、不启动 PoC。**

## 2. 目标与边界

### 2.1 调研目标

1. 对 `google/adk-go`、`cloudwego/eino`、`trpc-group/trpc-agent-go` 三个框架做能力、架构、生态、可维护性四个维度的横向比较。
2. 在通用基座 + 多场景预留扩展空间的评估锚点下，给出**有充分证据支撑的选型建议**。
3. 沉淀一份可复用的 spec 文档，作为后续 PoC 计划、选型决策、技术债跟踪的源头。

### 2.2 评估锚点

- **通用基座**：不是为单一用例设计，必须支持多场景（ReAct 工具调用、多 agent 协作、长期记忆、工具生态）。
- **多场景预留**：保留扩展空间，但**不做任何未要求的能力堆砌**。
- **与 aris-proxy-api 现状契合**：
  - Go 1.25.1 后端、dig 依赖注入、fiber + huma HTTP 层、sqlx/PostgreSQL、Redis、Pond 协程池、cron。
  - 已有多 provider 模型路由与 SSE 合并 / 归一化能力。
  - 日志、context 传递、错误处理（`ierr`）、test 约定已成体系。
- **最小可控制成本**：学习曲线、API 复杂度、对现有架构的侵入性必须可控。

### 2.3 边界

- 不实现任何 PoC 代码，不写 `docs/superpowers/plans/`。
- 不绑定具体使用场景（"做 LLM 推理"还是"做运维 cron"留作下一轮 PoC 决定）。
- 不比较其它未在题目范围内的框架（如 LangChainGo、autogen-go、crewAI-go 等）。

## 3. 三个框架的来源与一手材料

| 框架 | 仓库 | 本地 clone | 最近提交 | `go.mod` 要求 |
|------|------|-----------|---------|---------------|
| adk-go | https://github.com/google/adk-go | `/tmp/opencode/agent-research/adk-go` | 2026-06-16 | `go 1.25.0` |
| eino | https://github.com/cloudwego/eino | `/tmp/opencode/agent-research/eino` | 2026-06-15 | `go 1.18`（核心包） |
| trpc-agent-go | https://github.com/trpc-group/trpc-agent-go | `/tmp/opencode/agent-research/trpc-agent-go` | 2026-06-16 | `go 1.21` |

> 注意：eino 由三个仓库组成：核心 `cloudwego/eino`、扩展实现 `cloudwego/eino-ext`、示例 `cloudwego/eino-examples`。本调研只看核心；扩展实现以官方仓库为准。

上下文与背景信息通过 [Context7](https://context7.com) 拉取（`/google/adk-go`、`/cloudwego/eino`、`/trpc-group/trpc-agent-go`），并与本地 clone 的源代码交叉验证。

## 4. 架构与抽象层对比

### 4.1 核心接口形态

| 维度 | adk-go | eino | trpc-agent-go |
|------|--------|------|---------------|
| Agent 接口入口 | `agent.Agent`（`Name/Description/Run/SubAgents/FindAgent/FindSubAgent`） | `adk.TypedAgent[M MessageType]`（**泛型 M**，编译期约束 `*schema.Message` ∪ `*schema.AgenticMessage`） | `agent.Agent`（`Run/Tools/Info/SubAgents/FindSubAgent`） |
| LLM/Model 接口 | `model.LLM.GenerateContent` → `iter.Seq2[*LLMResponse, error]`（Go 1.23 范围迭代） | `model.BaseModel[M]`（泛型，与 MessageType 对齐） | `model.Model.GenerateContent` → `<-chan *Response`（channel 流式） |
| Tool 接口 | `tool.Tool` + `toolset.Toolset`（动态过滤） + `runnableTool`（带 `Declaration/Run`） | `tool.BaseTool` + `tool.InvokableTool/StreamableTool`，通过 `compose.ToolsNode` 编排 | `tool.Tool` + `tool.CallableTool/StreamableTool` + `toolset.ToolSet` |
| Run/Runner | `runner.Runner`（**强绑定** sessionService + memoryService + plugin + artifactService） | `adk.TypedRunner[M]`（**轻量**，核心只持 Agent 与 CheckPointStore；session/memory 是 agent 内部状态） | `runner.Runner`（与 adk-go 类似，组合 sessionService/memoryService/artifactService/plugins） |
| Event 通道 | `iter.Seq2[*session.Event, error]` | `*AsyncIterator[*TypedAgentEvent[M]]` | `<-chan *event.Event` |
| 上下文 | 自有 `InvocationContext` 接口，封装 agent / session / artifacts / memory / runConfig | 用 `context.Context` + `compose.ProcessState` + `schema.RegisterName` 做 checkpoint 序列化 | 自有 `Invocation` 结构（`agent.Invocation`） + `InvocationContext` 接口 |

### 4.2 多 Agent 编排模式

| 模式 | adk-go | eino | trpc-agent-go |
|------|--------|------|---------------|
| Sequential | `workflowagents/sequentialagent` | `compose.Chain`（不是 agent，是 graph） | `chainagent.ChainAgent` |
| Parallel | `workflowagents/parallelagent` | `compose.Parallel`（graph 节点并行） | `parallelagent.ParallelAgent` |
| Loop | `workflowagents/loopagent` | `compose.Graph` + 自定义分支 | `cycleagent.CycleAgent` |
| 路由分发（host/supervisor） | 通过 sub-agent transfer（runner.findAgentToRun） | `flow/agent/multiagent/host.MultiAgent`（host 决定） + `adk/prebuilt/supervisor` | `team.Team`（`ModeCoordinator` / `ModeSwarm`） |
| Plan-and-Execute | 需自己实现 | `adk/prebuilt/planexecute`（开箱即用） | 需自己实现 / 用 graph |
| Deep Agent（Claude Code 风格） | 需自己实现 | `adk/prebuilt/deep.DeepAgent`（开箱即用，含 task_tool、filesystem、todos） | 需自己实现 |
| Agent-as-Tool | `tool/agenttool`（将子 agent 包成 tool） | `adk/agent_tool.go.NewAgentTool` | `tool/agent`（配合 `team.ModeCoordinator`） |
| Graph 工作流（LangGraph 等价物） | ❌ 无内置 | ⚠️ 通过 `compose.Graph` 实现 | ✅ `graph` 包（**声明式图、条件边、checkpoint、cache、人机协同**） |

> **关键观察**：eino 官方注释明确说 `TransferToAgent`（full-context transfer）"has not proven to be more effective empirically"，官方推荐 `ChatModelAgent + AgentTool` 或 `DeepAgent`；同时把 `supervisor` 标记为 `NOT RECOMMENDED`。这是 eino 当前的主推方向：**Claude Code / Manus 风格的 DeepAgent**，而不是"supervisor 派活"。

### 4.3 Tool 生态

| 工具/能力 | adk-go | eino（核心 + ext） | trpc-agent-go |
|----------|--------|---------------------|----------------|
| MCP（Model Context Protocol） | `tool/mcptoolset`（基于 `modelcontextprotocol/go-sdk`） | ext 子仓库提供 | `tool/mcp`（基于 `trpc-mcp-go`）+ `mcpbroker` |
| Google Search / 内置工具 | `tool/geminitool/google_search.go` | 无内置（依赖 ext） | `tool/duckduckgo`、`tool/arxivsearch`、`tool/google`（google 由 ext 提供） |
| Function Tool | `tool/functiontool` | `components/tool/utils.NewTool` | `tool/function` |
| Tool 过滤/动态启用 | `tool.AllowedToolsPredicate` + `tool.FilterToolset` | 通过 `compose.ToolsNode` + state pre/post handler | `tool/filter.go` + `tool_activation` |
| Tool-as-Confirmation（HITL） | `tool.WithConfirmation` + `ConfirmationProvider` | ext `humaninloop` | `tool/permission.go` |
| 工具结果合并 | ❌ | `compose.ValuesMerge` + `tool.Merge` | `tool/merge.go` |
| Code Execution | ❌ | `adk/prebuilt/deep` 中内置 Shell / StreamingShell | `codeexecutor`（local、containerized、workspace） |
| Long-running tool | `tool.IsLongRunning() bool` + 单独支持 | 通过 streamable tool + middleware | 通过 state pre/post handler + `awaitreply` |

### 4.4 Session / Memory / Knowledge / Artifact

| 能力 | adk-go | eino | trpc-agent-go |
|------|--------|------|---------------|
| Session 抽象 | `session.Service`（`InMemoryService`、`session/database` 提供 SQL 实现） | 通过 `compose.GenLocalState` + `CheckPointStore` | `session.Service` + 多个实现（`inmemory`、`redis`、`postgres`、`mysql`、`sqlite`、`clickhouse`、`pgvector`、`noop`） |
| Memory（跨 session 长期记忆） | `memory.Service`（`AddSessionToMemory/SearchMemory`） | 通过 `compose.CheckPointStore` 持久化 agent state | `memory.Service`（inmemory / redis / postgres / sqlite） + `memoryinmemory.Tools()` 让 agent 自己操作 |
| Knowledge（RAG） | ❌ 无独立包（与 Google 生态 Vertex AI Search 集成） | 通过 `Retriever/Indexer/Embedder` 组件（ext 实现 vector store） | `knowledge` 包（独立抽象 `Knowledge.Search`），含 `embedder`、`vectorstore`、`chunking`、`extractor`、`graph` 增强 |
| Artifact（用户上传文件、产物） | `artifact.Service` + GCS 后端 | `adk/filesystem` + `adk/middlewares/filesystem` | `artifact.Service` + `internal/artifact` + S3-like |

> **对 aris-proxy-api 的含义**：trpc-agent-go 的 session / knowledge / memory 实现最丰富，与项目现有 PostgreSQL + Redis 栈契合度最高。adk-go 的 session database 走 GORM，与现有 `dbmodel` 体系有重叠。eino 没有官方 session service，agent 状态通过 checkpoint 持久化（更像 LangGraph 思路），需要自己粘合到现有 session。

### 4.5 协议与外部集成

| 协议/能力 | adk-go | eino | trpc-agent-go |
|----------|--------|------|---------------|
| A2A（Agent-to-Agent） | `a2a-go`（`github.com/a2aproject/a2a-go` v0.3.15） | ❌ | `a2aagent`、`a2aadk`、`a2acodeexecution`、`a2amultipath`、`a2asubagent`、`a2ui` |
| AG-UI（前端协议） | `adk-web`（外部仓库） | ❌ | `agui/` |
| OpenAI 兼容 HTTP Server | `server/` 包（adk-go 自带 server） | ❌ | `openaiserver`（**直接以 OpenAI API 协议暴露 agent 能力**） |
| Live / Bidi 实时语音 | ✅ `agent/live.go` + `RunLive`（Gemini Live API） | ❌ | ❌ |
| Vertex AI / Gemini 集成 | ✅ 一等公民（`model/gemini`、`model/vertexai`） | ext 提供 | `model/gemini` + `model/anthropic` + `model/openai` + `model/bedrock` + `model/huggingface` + `model/hunyuan` + `model/ollama` |
| 模型 Failover / Hedge | ❌ | ✅ `adk/failover_chatmodel.go` + `adk/retry_chatmodel.go` + `adk.TypedModelRetryConfig` + `adk.ModelFailoverConfig` | ✅ `model/failover` + `model/hedge` |
| Provider Registry | ❌ | ❌ | ✅ `model/registry.go` |
| Prompt Caching | 通过 Gemini 客户端透传 | 通过 model option 透传 | ✅ 内置 prompt caching（README 声称可节省 90%） |

### 4.6 可观测性 / 工程能力

| 能力 | adk-go | eino | trpc-agent-go |
|------|--------|------|---------------|
| OpenTelemetry trace | ✅ `internal/telemetry` + `telemetry/` 包 | 通过 callback handler 注入（`compose.WithCallbacks`） | ✅ `telemetry/` + `telemetry/semconv/trace` |
| OpenTelemetry metric | ⚠️ 部分（OTLP exporter 在 deps） | ❌ | ✅（otel metric SDK） |
| Langfuse / 自有 trace 后端 | 文档示范 Langfuse | 通过 callback 自由扩展 | `telemetry/` + Langfuse 示例 |
| Checkpoint / 恢复 | ❌（无内置） | ✅ `compose.CheckPointStore` + `Runner.Resume` + `Runner.ResumeWithParams` + 跨 checkpoint 升级兼容（react.go 中 v0.7/v0.8 兼容层） | ✅ `graph/checkpoint` + 多 backend（postgres、redis、sqlite、mysql） |
| Interrupt / Resume | ❌ | ✅ `compose.Interrupt` + `compose.StatefulInterrupt` + `CompositeInterrupt` | ✅ 通过 `await_user_reply` + state |
| Evolution / Self-improvement | ❌ | ❌ | ✅ `evolution` 包（async skill-extraction pipeline，2026-06-16 新增） |
| Evaluation | ❌ | ❌ | ✅ `evaluation` + `benchmark` |
| 单元 / E2E 测试样例 | `examples/` + `cmd/launcher` 完整 CLI | `examples/`（独立仓库）+ 大量 `*_test.go` | `examples/` 60+ 个，覆盖几乎所有能力 |

### 4.7 代码体量与活跃度

仅算核心代码（不含 examples，单仓 `find . -name '*.go' -not -path './examples/*'`）：

| 框架 | 文件数 | 代码行数 | 最近一次提交 |
|------|--------|----------|--------------|
| adk-go | 329 | ~75k | 2026-06-16 |
| eino | 338 | ~150k | 2026-06-15 |
| trpc-agent-go | 2205 | ~1,000k | 2026-06-16 |

> 解读：trpc-agent-go 的体量约是 adk-go 的 13 倍、eino 的 6.7 倍，功能最全但学习曲线最陡。adk-go 最小巧精炼。eino 居中，能力最聚焦在 agent + graph。

## 5. 与 aris-proxy-api 现状契合度

### 5.1 共建契合点

| 项目现有能力 | adk-go 契合 | eino 契合 | trpc-agent-go 契合 |
|-------------|------------|----------|-------------------|
| 多 provider LLM 路由（OpenAI / Anthropic / 自建） | ⚠️ adk-go 模型层是 Gemini 一等公民 + `google.golang.org/genai`，接 OpenAI/Anthropic 需要实现 `model.LLM`（已有 `apigee` 示例） | ✅ 通过 `BaseModel[M]` 泛型接口接任意实现（ext 仓库已提供 OpenAI、Ollama、ARK 等），`failover_chatmodel` 可直接复用多 provider 思路 | ✅ 内置 `openai` / `anthropic` / `gemini` / `bedrock` / `ollama` / `huggingface` / `hunyuan` provider，且 `registry.go` 支持按名查表 |
| SSE 流式响应 | ✅ Runner 透传 `iter.Seq2`，Live 支持 bidi | ✅ `compose.Stream` + `StreamReader` 原语 | ✅ `<-chan *event.Event` 直接映射到现有 SSE 链路 |
| dig 依赖注入 | ⚠️ 框架自身是构造器模式（`New`），需要写 factory 包一层 | ⚠️ 同上 | ⚠️ 同上 |
| 现有 `dbmodel` / PostgreSQL | ⚠️ adk-go `session/database` 用 GORM，与 `internal/infrastructure/database` 体系重叠 | ❌ 没有 session 概念 | ✅ `session/postgres`、`session/pgvector` 可直接用，但要注意不能强行替换现有 `SessionReadRepository` 接口 |
| Redis 已有 | ⚠️ session 没官方 Redis 实现 | ❌ | ✅ `session/redis`、`memory/redis` |
| 现有 cron 体系 | ⚠️ agent 可作为 cron task 跑，但需要自己写 wrapper | ⚠️ 同上 | ⚠️ 同上，但 `team/cycleagent` 天然契合 |
| OpenTelemetry 已有 | ✅ 框架自带 OTel | ⚠️ 通过 callback 注入 | ✅ 框架自带，且暴露 `telemetry/semconv/trace` |
| Context 传递约束（`util.CtxValueString` 等） | ⚠️ 用 `InvocationContext`，需要写适配层 | ⚠️ 用 `context.Context` 透传 | ⚠️ 用 `Invocation`，需要写适配层 |
| 现有 `ierr` 错误处理 | ⚠️ adk-go 错误散落 `fmt.Errorf`，需要逐个 wrap | ⚠️ eino 错误散落 `fmt.Errorf` | ⚠️ trpc-agent-go 同样有 `fmt.Errorf` |
| `sonic` JSON | ✅ 已有依赖；adk-go 不直接用 JSON | ✅ 依赖 sonic | ✅ 依赖 jsoniter/sonic 系列 |

### 5.2 风险点

| 风险 | 说明 |
|------|------|
| **API 稳定性** | adk-go 注释明确 `ConfirmationProvider`、`TransferToAgent` 等仍标记 `EXPERIMENTAL` / `NOT RECOMMENDED`，未来 API 可能变化。eino 在 `react.go` 维护 v0.7/v0.8 的 checkpoint 兼容层，说明 API 早期变更频繁。trpc-agent-go 体量最大，breaking change 频率未必更低，但 channel/event 模型稳定。 |
| **依赖膨胀** | trpc-agent-go 引入 `trpc-mcp-go`、`tencentyun/cos-go-sdk-v5`、`ants/v2`、`go-ego/gse`、`gonfva/docxlib` 等与 aris-proxy-api 当前依赖图谱差距较大。eino 引入 `nikolalohinski/gonja`（模板）、`wk8/go-ordered-map`、`slongfield/pyfmt`。adk-go 引入 `cloud.google.com/go/*`、`a2aproject/a2a-go`、`modelcontextprotocol/go-sdk`（同 trpc-agent-go 同源）。 |
| **学习曲线** | adk-go 接口最少、构造器最干净。eino 泛型约束 (`MessageType`) 优雅但门槛高。trpc-agent-go 概念最多（agent / graph / knowledge / memory / artifact / skill / event / planner / team / evalution / evolution / telemetry）。 |
| **与现有 SSE 协议合并逻辑的冲突** | aris-proxy-api 现有 SSE 合并 / 归一化在 `infrastructure/transport` 与 `application/llmproxy/converter`；如果引入 agent，需决定 agent 的 SSE 是否走原通道还是单独通道。 |
| **"是否引入图编排"** | trpc-agent-go 的 `graph` 是最大亮点（LangGraph 等价物），但 aris-proxy-api 现有架构不依赖图；引入图等于引入新的执行模型，后续排查 / 测试成本上升。 |

## 6. 选型建议

### 6.1 推荐优先级

按"通用基座 + 多场景预留 + 与现有栈契合"打分（1-5，5 最好）：

| 维度 | adk-go | eino | trpc-agent-go |
|------|--------|------|---------------|
| 与现有 LLM 路由契合 | 2 | 4 | 5 |
| 与现有 session/audit 体系契合 | 2 | 2 | 4 |
| 工具生态丰富度 | 4 | 4 | 5 |
| 多 agent 编排能力 | 3 | 4 | 5 |
| 图工作流 | 1 | 3 | 5 |
| 协议集成（A2A/AG-UI/MCP） | 4 | 2 | 5 |
| 工程能力（checkpoint/eval/telemetry） | 3 | 3 | 5 |
| API 稳定与成熟度 | 3 | 3 | 3 |
| 学习曲线友好 | 5 | 3 | 2 |
| 依赖控制 | 3 | 3 | 2 |
| **加权合计（粗略）** | **30** | **31** | **41** |

> **推荐顺序：trpc-agent-go > eino > adk-go**（作为主选型基座）；eino 与 adk-go 留作特定场景的"参考实现 / 局部借鉴"。

### 6.2 推荐理由（trpc-agent-go）

1. **生态完整度最高**：唯一同时具备 `graph`、`knowledge`、`memory`、`artifact`、`team`、`skill`、`evaluation`、`evolution`、多 backend session/memory、`openaiserver`、A2A、AG-UI、MCP 的 Go Agent 框架，能覆盖"通用基座 + 多场景"诉求。
2. **Provider 一等公民**：内置 OpenAI / Anthropic / Gemini / Bedrock / Ollama / HuggingFace / Hunyuan provider，与 aris-proxy-api 的多 provider LLM 路由心智模型一致。
3. **GraphAgent 是杀手锏**：声明式图 + checkpoint + 条件路由 + interrupt/resume，对应"复杂多步推理 / 工具调用 / 运维自动化"三大潜在场景，门槛比从零写状态机低很多。
4. **OpenAI 协议 server 直出**：`openaiserver` 让 agent 能力直接以 OpenAI 协议暴露给前端，能与 aris-proxy-api 现有 Web 端 chat 链路无缝集成。
5. **可观测性完整**：自带 OTel trace/metric、Langfuse 示例、`telemetry/semconv`；现有 aris-proxy-api 的 OTel 链能复用。

### 6.3 备选与不推荐

- **eino**：作为"agent 设计模式参考"价值最高（DeepAgent / Plan-Execute / 中间件机制 / ReAct 作为 graph 节点 / AgenticMessage 双轨消息），但 session 抽象缺失，与现有栈契合度低。如果未来要做"Claude Code 风格"的 agent 任务自动化，eino 的 `adk/prebuilt/deep` 设计可以直接借鉴。
- **adk-go**：API 最干净、构造器最优雅、依赖最少。如果未来某个 PoC 只想跑通"Gemini + MCP + Live"单点能力，可以作为快速参考实现。但其 session/database GORM 体系与现有 `dbmodel` 冲突，且没有 Knowledge / Evolution / Team / OpenAI-Server 等关键能力。

## 7. 后续路线（仅作记录，不在本期实现）

> 以下条目**不在本期范围内**，列在这里仅为避免未来重新调研。

- **下一步 PoC（未启动）**：
  1. 跑通 trpc-agent-go `llmagent` + `function tool` + `session/inmemory` 的最小链路，对比与现有 LLM 代理的延迟与 token 差异。
  2. 将 1-2 个 session 列表 / audit 列表查询包装成 `tool.FunctionTool`，验证"自然语言查内部数据"场景的可行性。
  3. 评估 `openaiserver` 是否能直接挂到现有 `/api/openai/v1/chat/completions`，作为"内部 agent 能力对外暴露"通道。
- **远期**：若 PoC 成功，再讨论 graph 工作流 / team / knowledge 是否引入；不必一次性铺开。
- **备选思路**：若不希望引入完整 agent 框架、只想解决"工具调用 + ReAct 循环"，可考虑从 eino 的 `adk/prebuilt/deep` 抄设计（最聚焦），或自己基于 huma handler 实现 100-200 行的极简 loop（最少依赖，但牺牲可维护性）。

## 8. 依赖与风险

- **结论的有效期**：本调研基于 2026-06-16 当天的源码；adk-go / eino / trpc-agent-go 都在持续演进（最近 30 天均有 commit）。建议在真正启动 PoC 前再跑一次 release notes diff。
- **避免一次性大改造**：三个框架都未在 aris-proxy-api 真实生产流量上验证过，第一步必须是 PoC + 灰度，不能直接替换现有 LLM 代理主路径。
- **可逆性**：所有候选框架都通过 `go get` 引入，封装在 `internal/agent/` 独立 package 之下，不污染主路径，必要时可整包删除。
- **与 SPEC 之外的依赖**：
  - `trpc-mcp-go`、`a2aproject/a2a-go` 等子模块遵循上游 semver，需要在 `go.mod` 明确 indirect 升级策略。
  - 若选 trpc-agent-go，需要 review 它对 `cloud.google.com/go`、`github.com/tencentyun/cos-go-sdk-v5` 等"我方不直接使用"的传递依赖。

## 9. 附录：关键文件索引

> 以下文件来自本地 clone，可作为后续 PoC 的源码入口。

### 9.1 adk-go

- 核心抽象：`agent/agent.go`（Agent interface）、`agent/llmagent/llmagent.go`（LLMAgent 构造器）、`model/llm.go`（LLM interface）、`tool/tool.go`（Tool interface）、`session/service.go`、`memory/service.go`。
- Runner：`runner/runner.go`（Run + RunLive）。
- Workflow agents：`agent/workflowagents/{sequential,parallel,loop}agent/`。
- 内置工具：`tool/{functiontool,geminitool,agenttool,mcptoolset,skilltoolset,toolconfirmation,loadmemorytool,preloadmemorytool,loadartifactstool,exampletool,exitlooptool}/`。
- 示例：`examples/{quickstart,workflowagents,tools,mcp,skills,toolconfirmation,web,bidi,telemetry,rest,a2a,vertexai,agentengine}/`。

### 9.2 eino

- 核心抽象：`adk/interface.go`（TypedAgent）、`adk/chatmodel.go`（ChatModelAgent）、`adk/runner.go`（Runner + Resume/ResumeWithParams）、`adk/react.go`（ReAct as a Graph，含 v0.7/v0.8 checkpoint 兼容层）、`adk/agent_tool.go`、`adk/callback.go`、`adk/interrupt.go`、`adk/middlewares/`、`adk/prebuilt/{deep,planexecute,supervisor}/`。
- Compose（graph 引擎）：`compose/{chain,chain_parallel,chain_branch,branch,graph,tool_node,agentic_tools_node,checkpoint,interrupt,state,stream_reader,values_merge,workflow}.go`。
- Components：`components/{model,tool,retriever,indexer,embedding,prompt,document}/`。
- Schema：`schema/{message,document,select,agentic_message}.go`。
- Flow（中间件 / 多 agent 模式）：`flow/agent/{multiagent/host,react,deep}/`、`flow/agent/multiagent/host/{types,compose,options,callback}.go`。
- 示例与扩展：核心仓不含 examples 与 ext；examples 在 `cloudwego/eino-examples`，ext 在 `cloudwego/eino-ext`。

### 9.3 trpc-agent-go

- 核心抽象：`agent/agent.go`、`agent/llmagent/llm_agent.go`、`agent/{chainagent,parallelagent,cycleagent,a2aagent,graphagent}/`、`agent/await_user_reply.go`、`agent/callbacks.go`、`agent/invocation*.go`。
- Runner：`runner/runner.go`（含 `WithSessionService/WithMemoryService/WithArtifactService/WithPlugins/WithAgent/WithAgentFactory/WithAwaitUserReplyRouting/WithCandidateSelector` 等 option）。
- Tool 生态：`tool/{function,callbacks,context,filter,final_result,merge,metadata,permission,retry,stream,stream_preferences,toolset,mcp,mcpbroker,duckduckgo,arxivsearch,email,google,hostexec,awaitreply,claudecode,codex,codeexec,file,openapi,openviking,skill,todo,taskrun,agent,transfer}/`。
- Model：`model/{model,callbacks,request,response,registry,provider,file_downloader,http_client,message_compare,message_validator,response_error_convert,pointers,token_tailor}.go` + `model/{openai,anthropic,gemini,ollama,bedrock,huggingface,hunyuan,failover,hedge,tiktoken}/`。
- Session：`session/{session,state,track,hook,ingestor}.go` + `session/{inmemory,redis,postgres,mysql,sqlite,clickhouse,pgvector,noop,summary}/`。
- Memory：`memory/memory.go` + `memory/{inmemory,redis,postgres,sqlite}/`。
- Knowledge：`knowledge/{knowledge,default,default_options,graph_knowledge,graph_knowledge_default,load_reporter}.go` + `knowledge/{chunking,document,embedder,extractor,indexer,retriever,vectorstore,graphstore}/`。
- Graph：`graph/{graph,executor,executor_dag,execution_engine,events,emitter,checkpoint,callbacks,call_options,cache,cache_key,errors,completion_control,agent_tool_interrupt,resume}.go` + `graph/checkpoint/`。
- Team：`team/{team,swarm,swarm_members,runtime,options,structure_export}.go`。
- Skill：`skill/{repository,context_repository,scope,state_keys,state_order,url_root}.go`。
- 其他： `event/`、`telemetry/`、`telemetry/semconv/trace/`、`evaluation/`、`evolution/`、`artifact/`、`prompt/`、`planner/`、`plugin/`、`codeexecutor/`。
- 示例：`examples/` 60+ 个子目录，覆盖 llmagent / graph / team / mcp / knowledge / memory / humaninloop / openaiserver / evaluation / evolution 等。

---

**调研人**：基于本地源码 + Context7 文档交叉验证。  
**复现方法**：本调研所有结论可由 `cd /tmp/opencode/agent-research/<repo> && grep/Read` 复现，无需联网。
