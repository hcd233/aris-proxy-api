---
name: cls-log-bugfix
description: >-
  Debug and fix bugs using CLS (Tencent Cloud Log Service) logs for the aris-proxy-api project (region: 广州/ap-guangzhou). 
  MUST use this skill whenever the user reports an error, bug, or unexpected behavior, especially when they mention 
  CLS logs, log entries, error messages from production, traceId, or ask you to investigate a bug "from the logs". 
  This skill guides a focused workflow: extract error keywords → search CLS logs → trace request via traceId → 
  correlate with code → design test cases → fix bug → verify with tests. Even if the user just says "this error happened" 
  without mentioning CLS, check whether this skill applies — any production bug in this project should use CLS logs 
  as the starting point for investigation.
---

# CLS 日志排障流程 (cls-log-bugfix)

当你收到用户报告的错误信息时，按照以下四步流程进行排障。

## 前置准备

### 日志主题查找

每次排障开始时，先用 `mcp__cls-mcp-server__GetTopicInfoByName` 按名称查找日志主题 ID：

```go
// 参数
Region: "ap-guangzhou"
searchText: "{项目名称}"
```

记录返回的 TopicId 供后续查询使用。

### 时间范围确定

用 `mcp__cls-mcp-server__ConvertTimestampToTimeString` 获取当前时间（不传 timestamp 参数），让用户确认报错的大致时间范围。如果用户不确定，默认回看最近 15 分钟。

然后用 `mcp__cls-mcp-server__ConvertTimeStringToTimestamp` 计算出 From/To 的毫秒级时间戳。

---

## Step 1: 拆解关键词 → 搜索 CLS 日志

### 1.1 从错误信息中提取搜索关键词

用户给出的错误信息可能是：
- **HTTP 错误响应**（如 500 Internal Server Error、400 Bad Request）
- **错误日志内容**（如 "connection refused"、"timeout"）
- **用户操作描述**（如 "调用 OpenAI 接口时报错"）

从中提取出可搜索的关键词组合：
- 模块名模式：`[ModuleName]`（如 `[OpenAIProxy]`、`[AnthropicService]`、`[SessionService]`）
- 错误级别：`level:ERROR`、`level:WARN`
- 错误关键词：`error`, `fail`, `timeout`, `refused`, `panic`
- HTTP 状态码：`status:500`、`status:404`
- 上游端点：`api.openai.com`、`api.anthropic.com`
- 模型名/API Key 特征等业务关键词

### 1.2 生成 CQL 查询语句

用 `mcp__cls-mcp-server__TextToSearchLogQuery` 将自然语言查询描述转为 CQL：

```
Region: "ap-guangzhou"
TopicId: <从 GetTopicInfoByName 获取>
Text: "查询最近15分钟 ERROR 级别的日志，包含 [OpenAIProxy] 和 timeout 关键词"
```

### 1.3 执行日志搜索

用 `mcp__cls-mcp-server__SearchLog` 查询日志：

```
Region: "ap-guangzhou"
TopicId: <topicId>
From: <毫秒时间戳>
To: <毫秒时间戳>
Query: <TextToSearchLogQuery 生成的 CQL>
Limit: 50
```

**搜索结果结构：**
搜索结果中的每条日志包含字段：
- `message` — 日志消息体，格式为 `[ModuleName] English message`
- `level` — 日志级别
- `timestamp` — ISO 8601 时间戳
- `caller` — 调用位置（如 `service/session.go:42`）
- `stack` — 堆栈信息（仅 ERROR 级别且有 panic 时）
- `traceID` — **关键字段**，请求追踪的唯一标识
- `userID` / `userName` / `apiKeyID` — 用户和 API Key 上下文

### 1.4 初步分析

阅读搜索结果，识别：
1. **哪些模块出现了错误** — 从 `[ModuleName]` 前缀判断
2. **错误的调用方向** — Service → Proxy → upstream（从外到内）或 反向
3. **错误类型** — 网络错误、业务错误、超时、上游拒绝等
4. **是否有明显的 traceId** 可用于链路追踪

---

## Step 2: 按 traceId 追踪全链路 → 关联代码

### 2.1 提取 traceId

从 Step 1 的错误日志中找到一个 traceId。traceId 是一个 UUID v4 格式的 36 位字符串（如 `a1b2c3d4-e5f6-7890-abcd-ef1234567890`）。

### 2.2 按 traceId 搜索全链路日志

用 traceId 搜索所有相关的日志记录，获取完整的请求生命周期：

```
Region: "ap-guangzhou"
TopicId: <topicId>
From: <适当放宽时间范围，比 Step 1 稍宽>
To: <毫秒时间戳>
Query: "traceID:\"<traceId>\""
Sort: "asc"  // 按时间升序排列，查看请求的完整流程
Limit: 100
```

### 2.3 分析请求链路

按时间升序阅读日志，重建请求的完整路径。典型的 LLM 代理请求链路如下：

```
[进入]  Fiber 中间件 → APIKeyMiddleware 验证 Key
  ↓
[进入]  Service 层 — 端点查找（ModelEndpoint 表）
  ↓
[进入]  Converter 层 — 协议转换（如有跨协议调用）
  ↓
[进入]  Proxy 层 — HTTP 请求构建 → 发往上游
  ↓                         ↕ (网络耗时)
[返回]  Proxy 层 — 收到上游响应/错误
  ↓
[返回]  Service 层 — 回调处理、消息存储（异步任务）
  ↓
[返回]  Handler 层 — 响应写入客户端
```

关键观察点：
- **哪个步骤耗时最长** — 两行相邻日志的时间戳差
- **错误在哪个模块产生** — `[ModuleName]` 的归属
- **错误是否携带上游错误信息** — `upstreamError` 字段
- **是否有跨协议转换错误** — Converter 层的日志

### 2.4 根据 `caller` 字段关联代码

日志中的 `caller` 字段记录了代码位置（如 `service/session.go:42`）。打开对应文件阅读相关代码：

1. **阅读调用位置的代码** — 理解出错的业务逻辑
2. **阅读上下游调用链** — 理解数据流
3. **关注错误处理路径** — 是忽略错误、重试还是直接返回
4. **关注上下文传递** — context 是否正确传递

### 2.5 阅读相关配置文件

如有需要，同时排查相关配置：
- `env/api.env` — 运行时环境变量
- `internal/config/config.go` — 配置加载逻辑

---

## Step 3: 设计测试用例

### 3.1 确定测试类型

根据错误类型选择测试方式：

| 错误类型 | 测试方式 | 说明 |
|---------|---------|------|
| 工具函数错误 | 单元测试 | 直接测试 `util/` 中的函数 |
| DTO 序列化错误 | 序列化/反序列化往返测试 | 测试 `dto/` 中的自定义 JSON 序列化 |
| 业务逻辑错误 | 单元测试或集成测试 | 测试 `service/` 中的方法 |
| 回归测试（修复旧 bug） | 回归测试 | 覆盖触发 bug 的场景 |

### 3.2 编写测试用例

遵循项目的测试规范（详见 CLAUDE.md 测试章节）：

**测试目录：** `test/<主题>/xxx_test.go`
**测试数据：** `test/<主题>/fixtures/cases.json`
**JSON 库：** `github.com/bytedance/sonic`（禁止 `encoding/json`）
**断言：** 标准库 `testing`（禁止 testify）
**依赖：** 每个测试独立，无相互依赖
**同步：** 禁止 `time.Sleep()`，使用 channel/WaitGroup

```go
// 测试用例结构体
type testCase struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    // 输入字段
    // 期望输出字段
}

// 数据加载 helper
func loadCases(t *testing.T) []testCase {
    t.Helper()
    data, err := os.ReadFile("./fixtures/cases.json")
    if err != nil {
        t.Fatalf("failed to read fixture: %v", err)
    }
    var cases []testCase
    if err := sonic.Unmarshal(data, &cases); err != nil {
        t.Fatalf("failed to unmarshal fixture: %v", err)
    }
    return cases
}
```

### 3.3 创建回归测试（Bug 修复必须）

Bug 修复必须附带回归测试，确保：
1. 测试能**复现原始 bug**（修复前失败，修复后通过）
2. 覆盖**正常路径和边界条件**
3. 从 `fixtures/` 文件中加载测试数据

---

## Step 4: 修复 Bug → 验证通过

### 4.1 编码修复

修复时遵循 CLAUDE.md 的开发规范：

- 错误处理用 `ierr.Wrap` / `ierr.New`，禁止 `fmt.Errorf` / `errors.New`
- 日志格式 `[ModuleName] English message`
- 敏感信息用 `util.MaskSecret()`
- 注解用 godoc 格式
- 禁止 `json.RawMessage` / `any` / `interface{}`
- JSON 统一用 `sonic`

### 4.2 运行规范扫描

```bash
make lint-conv
```

修复所有 ERROR，评估所有 WARN。

### 4.3 运行全量测试

```bash
make test
```

**全部测试必须 PASS 才能提交。**

如果新增测试失败：
1. 检查测试用例数据是否正确
2. 检查修复代码逻辑
3. 检查是否有其他依赖未满足

### 4.4 沉淀测试用例到规范

如果排障过程中发现了 CLS 查询规律、常见错误模式等知识，用 Write 工具保存到 memory 系统供后续参考。

---

## CLS MCP 工具速查

| 工具 | 用途 | 在本 skill 中的典型调用时机 |
|------|------|---------------------------|
| `GetTopicInfoByName` | 按名称查找日志主题 ID | Step 1 开始时 |
| `ConvertTimestampToTimeString` | 获取/转换时间字符串 | 确定时间范围 |
| `ConvertTimeStringToTimestamp` | 时间字符串 → 时间戳 | 计算 From/To 参数 |
| `TextToSearchLogQuery` | 自然语言 → CQL | Step 1.2 |
| `SearchLog` | 搜索日志内容 | Step 1.3, Step 2.2 |
| `DescribeLogContext` | 查看某条日志上下文（前后 N 条） | Step 2.2（如需更完整链路） |
| `DescribeIndex` | 查看日志主题索引配置 | Step 1.2（如需了解字段类型） |

---

## 常见错误模式速查

以下是本项目常见的错误模式及其排查方向：

### LLM 代理相关

| 错误模式 | 排查方向 | 常见模块 |
|---------|---------|---------|
| 上游请求超时 | 检查 Proxy 层超时配置、上游网络 | `[OpenAIProxy]`, `[AnthropicProxy]` |
| protocol 转换异常 | 检查 Converter 层 DTO 字段映射 | `[OpenAIProtocolConverter]`, `[AnthropicProtocolConverter]` |
| model endpoint 未找到 | 检查 ModelEndpoint 表、Provider 配置 | `[OpenAIService]`, `[AnthropicService]` |
| API Key 无效 | 检查 ProxyAPIKey 表 | `[APIKeyMiddleware]` |
| 消息存储失败 | 检查 storePool 配置、Redis | `[StorePool]` |

### 中间件相关

| 错误模式 | 排查方向 | 常见模块 |
|---------|---------|---------|
| 限流触发 | 检查 RateLimiter 配置 | `[TokenBucketRateLimiter]` |
| JWT 验证失败 | 检查 Token 密钥、过期时间 | `[JWTModule]` |
| 分布式锁超时 | 检查 Lock 中间件、Redis | `[LockMiddleware]` |
