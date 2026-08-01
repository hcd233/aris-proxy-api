---
name: query-prod-log
description: 查询生产 CLS 日志并做 RCA 根因定位。当用户遇到线上/准线上 bug、生产报错、traceId/X-Trace-Id、E2E 失败、或提供错误信息/异常日志要求排查时使用本 skill。流程为：查 CLS 日志 → 按 traceId 追全链路 → 定位根因（RCA）→ 向用户输出结论与建议修复方向。
---

# query-prod-log

用于线上/准线上 bug、用户报错、E2E 失败和日志排查的**查询与根因分析**。本 skill 的输出是 **RCA 结论 + 建议修复方向**，交付给用户决策，**不直接修改代码**。普通需求开发不要默认进入本流程。CLS 地域固定为 `ap-guangzhou`。

流程闭环：**查日志 → 追 trace → 关联代码 → RCA 定位 → 输出结论与修复建议**。

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
- **HTTP 错误响应**（状态码、错误结构字段）
- **错误日志内容**（错误描述原文）
- **用户操作描述**（触发场景的粗略描述）

从中提取出可搜索的关键词组合：
- 模块标识：日志若带 `[ModuleName]` 前缀，取前缀值
- 错误级别：`level:ERROR`、`level:WARN`
- 错误关键词：`error`、`fail`、`timeout`、`refused`、`panic` 等通用错误词
- HTTP 状态码：`status:4xx`、`status:5xx`
- 外部依赖特征：上游端点、第三方服务标识
- 业务关键词：模型名、账号、Key 特征等与场景相关的词

### 1.2 生成 CQL 查询语句

用 `mcp__cls-mcp-server__TextToSearchLogQuery` 将自然语言查询描述转为 CQL：

```
Region: "ap-guangzhou"
TopicId: <从 GetTopicInfoByName 获取>
Text: "查询最近15分钟 ERROR 级别的日志，包含 <模块> 和 <关键词>"
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
- `message` — 日志消息体（常见格式为 `[ModuleName] English message`）
- `level` — 日志级别
- `timestamp` — ISO 8601 时间戳
- `caller` — 调用位置（文件:行号）
- `stack` — 堆栈信息（仅 ERROR 级别且有 panic 时）
- `traceID` — **关键字段**，请求追踪的唯一标识
- 业务上下文字段 — 用户/账号/Key 等标识字段（若有）

### 1.4 初步分析

阅读搜索结果，识别：
1. **哪些模块出现了错误** — 从 `[ModuleName]` 前缀判断
2. **错误的调用方向** — 按 入口 → 业务层 → 外部依赖 的层级判断错误发生在哪一跳
3. **错误类型** — 网络错误、业务错误、超时、外部拒绝等
4. **是否有明显的 traceId** 可用于链路追踪

---

## Step 2: 按 traceId 追踪全链路 → RCA 定位

### 2.1 提取 traceId

从 Step 1 的错误日志中找到一个 traceId（UUID 格式的请求唯一标识）。

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

按时间升序阅读日志，重建请求的完整路径。**不要假设固定的链路结构**——以日志实际呈现的入口、中间处理层、外部调用、返回路径为准，识别：

- 请求从哪个入口进入，经过哪些处理层；
- 是否调用了外部依赖（网络请求、存储、第三方服务）；
- 错误在哪一层产生、由哪一层抛出；
- 各相邻日志的时间戳差，定位耗时环节。

关键观察点：
- **哪个步骤耗时最长** — 两行相邻日志的时间戳差
- **错误在哪个模块产生** — `[ModuleName]` 的归属
- **错误是否携带外部错误信息** — 上游/依赖返回的原始错误字段
- **是否有数据转换错误** — 转换层/序列化层的日志

### 2.4 根据 `caller` 字段关联代码

日志中的 `caller` 字段记录了代码位置（文件:行号）。打开对应文件阅读相关代码，理解根因：

1. **阅读调用位置的代码** — 理解出错的业务逻辑
2. **阅读上下游调用链** — 理解数据流
3. **关注错误处理路径** — 是忽略错误、重试还是直接返回
4. **关注上下文传递** — context 是否正确传递

如需要，同时排查相关配置：
- 运行时环境变量（服务配置源）
- 配置加载逻辑（如何读取与校验配置）

> 代码浏览用 CodeGraph 定位符号、用 Serena 查引用；需要连接生产服务器查看运行时状态（pod/日志/配置）时，参考 `login-prod-server` 与 `operate-prod-service`。

---

## Step 3: 输出 RCA 结论与修复建议

定位到根因后，**输出 RCA 报告交给用户决策，不直接修改代码**。

### RCA 报告结构

```
## RCA 结论
- 现象：<用户报错 / 错误日志摘要>
- 根因：<模块 / 代码位置 / 触发条件，明确到 caller 与调用链>
- 影响范围：<哪些用户 / 接口 / 数据受影响，是否持续>
- 证据：<关键日志条目（时间、traceId、message）、相关代码位置>

## 建议修复方向
- 方向 1：<具体修改建议：改哪个文件/函数、改成什么样>
- 方向 2：<备选方案，如有>
- 验证方式：<如何验证修复有效：E2E 用例、单测、复现步骤>
- 风险与权衡：<是否影响线上、是否需要回滚预案>
```

### 边界

- **不写代码**：只输出建议，不直接修改源文件、不写测试；
- **用户确认后转入开发流程**：用户采纳修复方向后，按 `docs/agents/workflow.md` 走正常开发流程（加载对应编码 skill、建 worktree、TDD、回归用例、lint/test、提交部署）；
- **不做修复闭环的替代**：修复完成后的 E2E 验证、部署跟踪不属于本 skill 范围（见 `operate-prod-service` / `call-api`）。

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
