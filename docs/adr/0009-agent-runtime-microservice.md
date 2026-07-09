# 新增 agent-runtime 微服务

在 aris-proxy-api（控制平面/LLM 网关）之外增加独立的 agent-runtime 微服务，负责 AI Agent 的 ReAct 执行循环和沙箱工具调用。控制平面通过 gRPC stream 下发任务并接收事件流，proxy-api 作为唯一的对外窗口（参考 OpenClaw/Hermes 的 Gateway + Channel 模式），Web 对话窗口是第一个 Channel。

## Status
accepted

## Context

aris-proxy-api 目前是 LLM 代理网关 + 管理后台。平台已具备完整的 LLM 转发、鉴权、会话管理、审计和 E2E 测试能力。需要在此基础上增加 Agent Runtime 能力，让 AI Agent 能够在受控的沙箱环境中自主执行命令和文件操作，并通过 Web 端与用户进行对话式交互。

参考了 Claude Managed Agents 的理念，以及 OpenClaw/Hermes 的 Gateway + Channel + Agent 架构模式。

## Considered Options

### A. 同进程嵌入式（未选中）
在 proxy-api 进程内直接调用 eino ChatModelAgent。
- 优点：无网络跳转，LLM 调用零延迟
- 缺点：控制面与沙箱执行强耦合，故障不隔离；沙箱命令 OOM 或卡死会影响网关转发；Agent Loop 与 LLM 网关的扩容策略不同但被绑定

### B. 独立微服务（选中）
agent-runtime 作为独立 Go 服务部署，通过 gRPC stream 与 proxy-api 通信。沙箱用 Docker 容器隔离。proxy-api 作为控制平面（Hermes Gateway），agent-runtime 作为执行引擎。

#### 架构关系

```
用户 ─WebSocket/HTTP─► proxy-api（控制平面）──gRPC stream──► agent-runtime（执行引擎）
                         │                                        │
                         │  LLM Gateway（已有）◄──HTTP API Key────┘
                         │  EndpointResolver / 限流 / 审计
                         │
                         └── Postgres + Redis（共享基础设施）
```

- **proxy-api**：Channel 管理（Web UI 为第一个 channel）、Agent 生命周期管理（CRUD）、会话路由与状态管理、认证鉴权、LLM 网关（已有）
- **agent-runtime**：ChatModelAgent + TurnLoop（eino v0.9）、Docker 沙箱执行、gRPC event stream 回传
- 用户直接与 proxy-api 交互，agent-runtime 不暴露给外部
- agent-runtime 通过 API Key 客户端调用 proxy-api 的 LLM 转发接口（`Authorization: Bearer`），复用 EndpointResolver、跨协议转换、限流、审计，LLM 调用自动沉淀为 model-layer Session

#### Agent 执行模型

- **Agent Loop**：eino v0.9 ChatModelAgent（ReAct 循环：LLM 推理 → 工具调用 → 沙箱执行 → 观察结果 → 下一轮）
- **多轮对话**：TurnLoop（持续运行的 push-based 运行时，支持 Preempt 打断、Checkpoint/Resume 断点恢复、Graceful Stop）
- **消息协议**：eino v0.9 AgenticMessage（content block 模型，支持 text/reasoning/tool_call/tool_result/multimodal）
- **LLM 调用路径**：agent-runtime 内 eino openai 组件 → BaseURL 设为 proxy-api 地址 → `Authorization: Bearer <APIKey>` → proxy-api 已有能力兼容
- **工具集**（第一期）：bash（shell 命令执行）、read（读文件）、write（写文件）、edit（精确替换）、ls（目录列表）、grep（文件搜索）
- **工具执行**：Docker SDK 操作沙箱容器，通过 Sandbox 接口抽象（未来可切换为 CubeSandbox 等）

#### 沙箱

- **运行时**：Docker 容器，Alpine 镜像，内存限制 256m
- **Workspace 持久化**：bind mount `/data/agent-workspace/{user_id}:/workspace`
- **生命周期**：agent-runtime 全权管理（创建/启动/停止/销毁），proxy-api 只知道有"沙箱"不感知 Docker
- **第一期**：所有 user 共用一个容器（ponytail 最简方案）
- **未来升级**：CubeSandbox MicroVM（硬件隔离、Go SDK 原生支持文件/命令/快照/自动暂停/网络策略）。当前不选：生产环境云虚拟机不支持 KVM，PVM 模式需内核替换

#### 通信协议（gRPC）

```protobuf
service AgentRuntime {
  rpc RunAgent(AgentRequest) returns (stream AgentEvent);
}

message AgentRequest {
  string agent_id = 1;
  string session_id = 2;
  oneof action {
    NewTurn new_turn = 3;
    ResumeStream resume = 4;
    Cancel cancel = 5;
  }
  int64 from_sequence = 6;
}

message AgentEvent {
  string agent_id = 1;
  string session_id = 2;
  int64 sequence = 3;
  StatusEvent status = 4;
  AgentMessageChunk message = 5;
  AgentThoughtChunk thought = 6;
  ToolCallEvent tool_call = 7;
  ToolResultEvent tool_result = 8;
  InterruptEvent interrupt = 9;
  TurnComplete turn_complete = 10;
  ErrorEvent error = 11;
}
```

- AgentEvent 的 type 映射参考 eino ADK 0.9 的 `AgentEventToSessionUpdate`：assistant 消息→AgentMessageChunk，reasoning→AgentThoughtChunk，tool_calls→ToolCallEvent，tool_results→ToolResultEvent，interrupt→InterruptEvent
- AgentRequest 的 NewTurn 参考 Hermes `run_conversation(user_message, conversation_history, system_message, task_id)` 和 OpenClaw `/hooks/agent`（message, model, fallbacks, thinking, timeout）
- 命名采用大驼峰（PascalCase），proto 序号连续递增

#### 会话与断线重连

- **AgentSession**：agent-runtime 独立管理（共享 proxy-api 的 Postgres，独立 DAO）。一个 AgentSession 包含多次 LLM 调用（对应多个 model-layer Session）
- **SessionState**：proxy-api 维护状态机 `idle → running → idle/interrupted`，由 gRPC event stream 事件驱动
- **断线重连**：WebSocket 断开后 proxy-api 查 session 状态（`running`→ResumeStream 续订事件流；`idle`→等待新消息；`interrupted`→提示用户重试）
- **崩溃恢复**：TurnLoop checkpoint 保存到 Redis，重启后从 checkpoint 恢复

#### 数据存储

- agent-runtime 和 proxy-api 共享 Postgres 实例（各自独立 schema/表和 DAO 层）
- agent-runtime 数据：AgentSession（会话元数据）、对话消息时间线（event 持久化）
- proxy-api 数据：Agent 定义（system_prompt、workspace、model）、用户认证
- Redis：TurnLoop CheckpointStore（agent-runtime 专属 namespace）

#### 部署

- agent-runtime 作为一行 K8s Deployment 部署到同一集群
- 需要挂载 Docker socket（`/var/run/docker.sock`）和 workspace 持久卷
- 优雅关闭：停止接受新连接 → draining gRPC streams → 等待 TurnLoop Graceful Stop → 关闭 DB → 关闭 Redis
- 第一期单实例（自用场景，一个 agent 多个 session，无需路由）

### C. CubeSandbox 替换 Docker（未选中）
用 TencentCloud CubeSandbox 的 MicroVM 做沙箱。硬件虚拟化隔离、Go SDK 原生支持 Files()/Commands()/快照/克隆/回滚/自动暂停恢复/网络策略、E2B 兼容。
- 优点：隔离级别更高（独立内核+eBPF）、API 设计优雅（直接覆盖全部 6 个工具）、内存开销<5MB、快照/回滚能力强
- 缺点：生产环境云虚拟机不支持 KVM（`/dev/kvm` 不存在，`VMX unsupported`），PVM 模式需替换内核（OpenCloudOS）+ 重启服务器，基础设施变更大
- **决策**：保留为未来升级路径。通过 Sandbox 接口抽象确保切换成本低，工具层代码零改动。

## Consequences

### 正面影响
- Agent 执行与 LLM 网关故障隔离，沙箱 OOM 不影响转发
- 复用 proxy-api 全部 LLM 基础设施（EndpointResolver、跨协议转换、限流、审计），agent-runtime 不需要 LLM 协议逻辑
- eino v0.9 的 TurnLoop/Cancel/Checkpoint 直接解决了多轮对话、打断、断线恢复三大难题
- Sandbox 接口抽象让底层 Docker → CubeSandbox 切换成本低
- 单实例 + 共享容器 + Alpine 镜像保持第一期复杂度最低

### 负面风险
- gRPC stream 增加了网络跳转和断线重连复杂度
- agent-runtime 需要独立部署和运维（Dockerfile、K8s Deployment、Docker socket 挂载）
- Docker 容器安全隔离比 CubeSandbox MicroVM 弱，自用场景可接受但未来多用户需升级
- Session 状态机在 proxy-api 侧维护，状态同步依赖 gRPC stream 事件传递——stream 异常断开需防止状态漂移

### 未决议事项
- Alpine 基础镜像具体预装哪些工具（git、curl、python3、node？）
- Agent 定义在前端的 CRUD UI（Web 管理后台需要新页面）
- agent-runtime 的监控指标和告警规则
- 后续 Channel 扩展（企微/飞书）的优先级
