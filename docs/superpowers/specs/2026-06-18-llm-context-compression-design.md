# LLM 上下文压缩设计

## 背景与目标

aris-proxy-api 作为 LLM 代理网关，转发请求时 body 中的 tool output（工具调用返回的 JSON、日志、搜索结果等）往往占据绝大部分 token 预算。参考 [headroom](https://github.com/chopratejas/headroom) 项目的上下文压缩理念，在 Go 中复刻其确定性压缩算法（跳过 ML 模型），嵌入 usecase 层，在 marshal body 之后、转发上游之前执行压缩。

**一期目标**：对 OpenAI Chat / Anthropic Messages / OpenAI Responses 三种协议路径的 tool output 进行压缩，预期 tool output 密集型请求节省 60-90% 的 tool output token，且压缩失败永远不影响请求正常转发。

## 关键决策

| 决策项 | 选择 | 说明 |
|--------|------|------|
| 触发方式 | 全局开关 + 阈值 | 通过 Viper 环境变量控制，无需改数据库 schema |
| 压缩范围 | 仅 tool output | 不碰 system prompt、tools 定义、历史消息、用户输入 |
| 协议覆盖 | Chat + Messages + Responses | 三种协议全做，Responses 的 input item 结构单独处理 |
| Token 估算 | input_tokens 锚点 + bytes_saved/4 | 用真实 input_tokens 做"压缩后"基准，bytes_saved/4 估算节省量，反推"压缩前" |
| Body 修改策略 | raw map 修改 + 重新 marshal | 与现有 `ReplaceModelInBody` 模式一致，`MarshalUpstreamBody` 保证 key 字典序稳定 |
| 错误处理 | 压缩永不失败 | 任何异常回退原始 body，记录 warn 日志 |
| ML 压缩 | 不引入 | 一期只做确定性算法，不引入 ONNX/tiktoken 等外部依赖 |

## 架构设计

### 包结构

```
internal/application/llmproxy/compression/
├── compressor.go          # Compressor 接口 + Dispatcher
├── detector.go            # ContentDetector（纯正则）
├── result.go              # CompressionResult / CompressionStats
├── locator.go             # ToolOutputLocator 接口 + 通用入口
├── locator_openai.go      # OpenAI Chat: 扫描 messages[role=tool]
├── locator_anthropic.go   # Anthropic: 扫描 messages.content[type=tool_result]
├── locator_responses.go   # OpenAI Responses: 扫描 input[type=function_call_output]
├── smart_crusher.go       # JSON 数组 → lossless CSV / lossy 采样
├── log_compressor.go      # 日志模板提取 + 去噪
├── search_compressor.go   # grep 结果去重 + 摘要
└── passthrough.go         # 不识别类型的兜底
```

### 核心接口

```go
// Compressor 压缩单个 tool output 的文本内容
type Compressor interface {
    Compress(content string) (compressed string, stats CompressionStats, err error)
}

// Dispatcher 按 ContentType 路由到具体压缩器
type Dispatcher struct {
    detector    *ContentDetector
    compressors map[ContentType]Compressor
}

// ToolOutputLocator 按协议定位 body 中的 tool output，执行压缩，返回新 body
type ToolOutputLocator interface {
    LocateAndCompress(body []byte, dispatcher *Dispatcher) (newBody []byte, stats CompressionStats, err error)
}

// CompressionStats 记录单个请求的压缩统计
type CompressionStats struct {
    ToolOutputBytesBefore int      // 仅 tool output 部分的原始字节
    ToolOutputBytesAfter  int      // 压缩后字节
    ItemsCompressed       int      // 压缩的 tool output 条数
    ItemsSkipped          int      // 跳过的条数（太小/不识别/膨胀）
    StrategiesUsed        []string // 使用的压缩策略
}
```

### 数据流与集成点

```
Handler → UseCase.CreateChatCompletion
  ├─ resolve endpoint/model
  ├─ check blocked content
  ├─ marshal body (MarshalXxxBodyForModel)    ← body []byte 生成
  ├─ [NEW] compress body                      ← 插入点
  │    └─ if config.CompressionEnabled && len(body) >= minBodyBytes:
  │         locator.LocateAndCompress(body, dispatcher)
  ├─ forward to upstream (ForwardXxx)
  └─ audit task (+compression stats)
```

压缩在 usecase 层执行，Locator 选择由 **上游协议**（即 marshal 后的 body 结构）决定，而非客户端 API 类型：

| usecase 方法 | compatRoute | 上游 body 结构 | Locator |
|-------------|-------------|---------------|---------|
| `CreateChatCompletion` | `Native` | OpenAI Chat | `OpenAIChatLocator` |
| `CreateChatCompletion` | `ViaAnthropicMessage` | Anthropic Messages | `AnthropicMessagesLocator` |
| `CreateResponse` | `Native` | OpenAI Responses | `OpenAIResponsesLocator` |
| `CreateResponse` | `ViaOpenAIChat` | OpenAI Chat | `OpenAIChatLocator` |
| `CreateResponse` | `ViaAnthropicMessage` | Anthropic Messages | `AnthropicMessagesLocator` |
| `CreateMessage` (Anthropic) | `Native` | Anthropic Messages | `AnthropicMessagesLocator` |

关键原则：Locator 匹配的是 **marshal 后实际发往上游的 body 结构**，与客户端调用的 API 无关。跨协议转换后 body 已是目标协议格式，用目标协议的 Locator。

### 三种协议的 Locator 工作方式

**OpenAI Chat**（`locator_openai.go`）：
- `sonic.Unmarshal` body 到 `map[string]any`
- 遍历 `messages` 数组，找 `role == "tool"` 的消息
- 提取 `content` 字段（string）→ 检测类型 → 压缩 → 替换
- `MarshalUpstreamBody` 重新序列化

**Anthropic Messages**（`locator_anthropic.go`）：
- 遍历 `messages` 数组
- 每条 message 的 `content` 是数组，找 `type == "tool_result"` 的 block
- 提取 `content` 字段（可能是 string 或数组）→ 压缩 → 替换

**OpenAI Responses**（`locator_responses.go`）：
- 遍历 `input` 数组
- 找 `type == "function_call_output"` 的 item
- 提取 `output` 字段 → 压缩 → 替换

## 压缩算法

### ContentDetector（纯正则，无 ML）

检测 7 种内容类型，一期只对其中 3 种做实际压缩：

| 类型 | 检测特征 | 一期处理 |
|------|---------|---------|
| `JsonArray` | JSON 解析成功且为 array | SmartCrusher |
| `SearchResults` | 多行匹配 `^\S+:\d+:` | SearchCompressor |
| `BuildOutput` | 编译器/测试日志特征 | LogCompressor |
| `SourceCode` | `func`/`def`/`class` 等 | Passthrough（二期） |
| `GitDiff` | `diff --git`/`@@` hunk | Passthrough（二期） |
| `Html` | HTML 标签 | Passthrough |
| `PlainText` | 兜底 | Passthrough |

### SmartCrusher（JSON 数组）

```
输入: [{"name":"error","ts":"...","code":500}, ...]

Step 1: 尝试无损路径 (lossless)
  → 提取 schema (所有 key 的并集)
  → 转为 CSV: name,ts,code\nerror,...,500\n...
  → 如果 CSV 字节数 < 原始 * 0.7 → 采用 lossless

Step 2: lossless 不够好 → lossy 采样
  → 按 max_items (默认 20) 采样代表性行
  → 前 N 行 + 尾部 N 行 + "...省略 M 行..." 摘要
  → 保留含 error/exception/fail 关键词的行（约束条件）
```

### LogCompressor（日志模板提取 + 去噪）

```
输入: 多行构建/测试日志

Step 1: 按行分割
Step 2: 模板化每行
  → 正则替换数字/时间戳/路径/ID 为占位符
  → [2024-01-01 10:00:00] ERROR fetch failed → [TIMESTAMP] ERROR fetch failed
Step 3: 按模板分组，统计出现次数
Step 4: 输出
  → 保留 ERROR/WARN/FATAL 行（全部保留）
  → INFO/DEBUG 行按模板去重，保留 1 行 + "(重复 N 次)"
  → 尾部附加统计摘要
```

### SearchCompressor（grep 结果去重 + 摘要）

```
输入: file:line:content 格式的搜索结果

Step 1: 解析每行为 (file, line, content)
Step 2: 按文件分组
Step 3: 每文件
  → 如果匹配数 <= max_per_file (默认 5) → 全部保留
  → 否则 → 前 max_per_file 行 + "...(省略 M 行)" + 后 2 行
Step 4: 尾部附加 "共 N 个文件, M 处匹配"
```

### 通用兜底规则

- 单个 tool output 内容 < `COMPRESSION_MIN_TOOL_OUTPUT_BYTES`（默认 512）→ 跳过
- 压缩后 > 压缩前（膨胀）→ 回退原文
- 任何解析/压缩错误 → 回退原文，记录 warn 日志

## 配置项

通过 Viper 环境变量配置，在 `internal/bootstrap/` 初始化时注入 usecase：

```env
# env/api.env 新增
COMPRESSION_ENABLED=false                    # 默认关闭，运维显式开启
COMPRESSION_MIN_BODY_BYTES=2048              # 整个 body 小于此值跳过
COMPRESSION_MIN_TOOL_OUTPUT_BYTES=512        # 单个 tool output 小于此值跳过
```

## Token 估算方案

采用 **字节比例外推** 方案，在 audit 提交时（上游响应已返回）计算：

```
压缩时记录:
  tool_output_bytes_before  = 仅 tool output 部分的原始字节
  tool_output_bytes_after   = 压缩后字节

上游返回后 (audit 时):
  usage.input_tokens = 真实值（压缩后）

计算:
  CompressedTokens = input_tokens * (bytes_before - bytes_after) / bytes_after
```

公式假设：tool output 的字节压缩比等比例映射到 token 压缩比，用真实 `input_tokens` 锚定"压缩后"基准，外推"压缩前"会消耗多少 token，差值即为节省量。

边界保护：`bytes_after == 0` 或 `bytes_after >= bytes_before` 时 `CompressedTokens = 0`。

时机上无问题——audit task 在上游响应返回后提交，此时 `usage.input_tokens` 已可用。

## Audit Task 集成

在现有 `ModelCallAuditTask` 中增加压缩统计字段：

```go
type ModelCallAuditTask struct {
    // ... 现有字段 ...
    CompressionEnabled    bool     // 是否启用了压缩
    CompressedTokens      int      // 估算节省的 token 数 = input_tokens * (before - after) / after
    CompressionStrategies []string // 使用的压缩策略
}
```

压缩时记录 `tool_output_bytes_before` / `tool_output_bytes_after`（中间值，不持久化），audit 提交时结合 `usage.input_tokens` 按公式计算 `CompressedTokens`。

## 错误处理（铁律：压缩永远不能让请求失败）

```
压缩流程
  ├─ body < min_body_bytes → 跳过，透传原始 body
  ├─ 解析 body 失败 → 回退原始 body，warn 日志
  ├─ locator 定位失败 → 回退原始 body，warn 日志
  ├─ 单个 tool output < min_tool_output_bytes → 跳过该条
  ├─ ContentDetector 检测失败 → passthrough 该条
  ├─ Compressor 压缩失败 → 回退该条原文，warn 日志
  ├─ 压缩后膨胀 (after > before) → 回退该条原文
  └─ 重新 marshal 失败 → 回退整个原始 body，warn 日志
```

所有 warn 日志带 `[Compression]` 前缀 + request_id，可通过 CLS 追踪。

## 测试策略

| 层级 | 位置 | 内容 |
|------|------|------|
| 单元测试 | `test/unit/compression/` | 每个压缩器独立测试 |
| 单元测试 | `test/unit/compression/` | ContentDetector 准确性 |
| 单元测试 | `test/unit/compression/` | Locator 定位准确性 |
| 集成测试 | `test/e2e/compression/` | 压缩后 body 能被上游接受 |
| 回归测试 | `test/unit/compression/` | 非 tool output 区域字节不变 |

测试 fixtures 放 `test/unit/compression/fixtures/`，包含：
- 各内容类型的真实 tool output 样本
- 预期压缩结果
- 边界情况（空数组、单行、超大、畸形 JSON）

## 二期展望（不在一期范围）

- **CCR（Compress-Cache-Retrieve）**：原始内容缓存 + 检索工具注入，实现可逆压缩
- **CodeCompressor**：基于 AST 的源码压缩
- **DiffCompressor**：Git diff hunk 过滤
- **CacheAligner**：prefix 稳定化（我们已有 `MarshalUpstreamBody` 基础）
- **Per-model 配置覆盖**：Viper 配置文件按 model alias 覆盖全局压缩设置
