---
name: agentmemory
description: Use when starting a new session, needing project context from past sessions, saving insights/decisions/lessons, or when agentmemory server is not running. Covers server health check, startup, memory recall, and experience persistence.
---

# AgentMemory 持久记忆管理

为 AI 编程助手提供跨会话的持久化记忆能力。负责服务器健康检查与启动、历史经验召回、新经验沉淀。

## 触发时机

- 新会话开始时，主动召回项目上下文
- 需要了解过去的技术决策、bug 修复、架构知识时
- 做出重要决策、发现 bug 模式、学到项目约定后，主动保存经验
- 发现 agentmemory 服务器未运行时，启动服务器

---

## Step 1: 检查服务器状态

每次会话开始或需要使用记忆功能时，先检查服务器是否运行：

```bash
curl -s http://localhost:3111/agentmemory/health 2>/dev/null
```

### 判断结果

| 响应 | 含义 | 下一步 |
|------|------|--------|
| JSON 含 `"status":"healthy"` | 服务器正常运行 | 跳到 Step 3 |
| 无响应 / connection refused | 服务器未运行 | 执行 Step 2 |
| JSON 含 `"status":"degraded"` | 服务降级 | 可继续使用，但部分功能受限 |

---

## Step 2: 启动服务器

服务器未运行时，在后台启动：

```bash
nohup npx -y @agentmemory/agentmemory > /tmp/agentmemory.log 2>&1 &
echo "PID: $!"
```

等待 20-30 秒后验证：

```bash
sleep 20 && curl -s http://localhost:3111/agentmemory/health 2>/dev/null
```

### 启动常见问题

| 问题 | 现象 | 解决 |
|------|------|------|
| npx 缓存了旧版本 | 版本号不是最新 | `npx -y @agentmemory/agentmemory@latest` |
| 端口被占用 | 日志显示 EADDRINUSE | `lsof -i :3111` 找到占用进程后 kill |
| iii-engine 启动失败 | 日志显示 engine 错误 | 检查 `~/.local/bin/iii` 是否存在，版本是否为 v0.11.2 |
| 无 LLM provider key | 日志警告 compression disabled | 正常，只是无法自动压缩摘要，核心功能不受影响 |

### 端口说明

| 端口 | 用途 |
|------|------|
| 3111 | REST API（MCP 工具通过此端口通信） |
| 3112 | WebSocket streams |
| 3113 | 实时记忆浏览器（可在浏览器打开查看） |

---

## Step 3: 召回历史经验

服务器就绪后，根据场景召回记忆。

### 3.1 会话开始时（自动上下文注入）

如果 OpenCode 插件 `agentmemory-capture.ts` 已正确安装在 `~/.config/opencode/plugins/`，会话开始时会自动注入项目上下文到 system prompt，无需手动操作。

### 3.2 主动搜索记忆

使用 `memory_smart_search` 进行混合语义+关键词搜索：

```
agentmemory_memory_smart_search:
  query: "<自然语言查询>"
  limit: 10
```

适用场景：
- "之前是怎么处理 JWT 认证的？"
- "这个文件之前出过什么问题？"
- "项目用的什么错误处理模式？"

### 3.3 按关键词召回

使用 `memory_recall` 按关键词搜索，返回完整内容：

```
agentmemory_memory_recall:
  query: "<关键词>"
  limit: 5
  format: full
```

### 3.4 查看最近会话

使用 `memory_sessions` 查看最近的会话列表：

```
agentmemory_memory_sessions: {}
```

### 3.5 搜索经验教训

使用 `memory_lesson_recall` 搜索已沉淀的教训：

```
agentmemory_memory_lesson_recall（通过 memory_smart_search 间接访问）
```

### 搜索策略

| 场景 | 推荐工具 | 说明 |
|------|---------|------|
| 模糊语义查询 | `memory_smart_search` | BM25 + 向量 + 图谱混合检索 |
| 精确关键词 | `memory_recall` | 关键词匹配，返回完整内容 |
| 了解工作历史 | `memory_sessions` | 最近的会话概览 |
| 查找文件相关记忆 | `memory_smart_search` + 文件路径 | 搜索文件历史上下文 |

---

## Step 4: 沉淀新经验

在以下时机主动保存经验到 agentmemory：

### 4.1 何时保存

- 做出重要架构或技术决策后
- 发现 bug 模式或常见错误后
- 学到项目特有的约定或模式后
- 用户明确要求"记住这个"
- 完成一个复杂的调试或修复流程后

### 4.2 使用 memory_save

```
agentmemory_memory_save:
  content: "<完整的经验描述>"
  concepts: "<2-5 个逗号分隔的关键词>"
  type: "<pattern|preference|architecture|bug|workflow|fact>"
  files: "<逗号分隔的相关文件路径>"
```

**type 选择指南：**

| type | 适用场景 | 示例 |
|------|---------|------|
| `pattern` | 代码模式、约定、规范 | 错误处理统一用 ierr，禁止 fmt.Errorf |
| `preference` | 用户偏好、团队习惯 | 用户偏好先写测试再写实现 |
| `architecture` | 架构知识、系统设计 | LLM 代理三层架构 |
| `bug` | Bug 模式、已知陷阱 | 某个版本的已知 bug 及 workaround |
| `workflow` | 工作流程、操作步骤 | 发布流程、调试流程 |
| `fact` | 事实性知识 | 项目使用 Go 1.25.1 和 bytedance/sonic |

**concepts 选择原则：**
- 使用具体的、可搜索的关键词短语
- 优先使用项目特有术语（如 `ierr` 而非 `error-handling`）
- 2-5 个概念，用逗号分隔

### 4.3 使用 memory_lesson_save

保存教训（带置信度评分，会随使用增强、随时间衰减）：

```
agentmemory_memory_lesson_save:
  content: "<教训内容：什么情况下适用、什么做法有效、什么要避免>"
  context: "<适用场景描述>"
  project: "aris-proxy-api"
  tags: "<逗号分隔的标签>"
  confidence: 0.7
```

### 4.4 保存示例

```
# 架构知识
agentmemory_memory_save:
  content: "LLM 代理请求链路：Fiber 中间件 → Huma 路由 → handler → usecase → domain service → repository/transport"
  concepts: "llm-proxy,request-chain,fiber,huma"
  type: "architecture"
  files: "internal/application/llmproxy/usecase,internal/infrastructure/transport"

# Bug 模式
agentmemory_memory_lesson_save:
  content: "修改 OpenAI DTO 字段时必须同步检查 converter 和 SSE 合并工具，否则会导致流式响应解析失败"
  context: "修改 LLM 代理 DTO 时"
  project: "aris-proxy-api"
  tags: "llm-proxy,dto,converter,sse,bug-pattern"
  confidence: 0.8
```

---

## Step 5: 运行记忆整理（可选）

长时间会话或积累了大量观察后，可以运行记忆整理：

```
# 运行四层记忆合并管道
agentmemory_memory_consolidate:
  tier: "episodic"   # 或 "semantic" 或 "procedural"

# 运行知识图谱反思
agentmemory_memory_reflect:
  project: "aris-proxy-api"
  maxClusters: 10
```

---

## 常见问题速查

| 问题 | 解决方案 |
|------|---------|
| MCP 工具列表中没有 agentmemory 工具 | 检查 opencode.json 中 mcp.agentmemory 配置是否正确，服务器是否运行 |
| 插件不自动捕获事件 | 确认 ~/.config/opencode/plugins/agentmemory-capture.ts 存在且服务器运行 |
| 搜索结果不相关 | 尝试更具体的查询词，或使用英文关键词 |
| memory_save 后搜索不到 | agentmemory 需要几秒钟建立索引，稍等再搜 |
| LLM 压缩/摘要功能不可用 | 正常现象，缺少 LLM provider key 时不影响核心存取功能 |
