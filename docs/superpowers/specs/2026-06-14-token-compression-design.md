# Token 压缩功能设计

> 2026-06-14 · centonhuang

## 背景

LLM 代理请求中的 messages（尤其是 tool 调用返回的大量 JSON 数据、搜索结果、构建日志等）存在大量冗余，直接发给上游造成 token 浪费。参考 [headroom](https://github.com/chopratejas/headroom) 的压缩思路，在 aris-proxy-api 中实现 Go 原生的上下文压缩管线。

## 一、总体架构

压缩管线作为 usecase 层的可选组件，在请求体序列化之前拦截并压缩 messages：

```
Handler → UseCase → EndpointResolver.Resolve → CompressPipeline.Compress(messages) → Marshal → ProxyPort
```

覆盖全部 7 条转发路径：

| API | 转发路径 | 压缩插入点 |
|-----|---------|-----------|
| OpenAI Chat | `forwardChatNative` | `MarshalOpenAIChatCompletionBodyForModel` 前 |
| OpenAI Chat | `forwardChatViaAnthropic` | `conv.FromOpenAIRequest` 后 |
| OpenAI Response | `forwardResponseNative` | `MarshalOpenAIResponseBodyForModel` 前 |
| OpenAI Response | `forwardResponseViaChat` | `conv.FromResponseRequest` 后 |
| OpenAI Response | `forwardResponseViaAnthropic` | `conv.FromResponseAPIRequest` 后 |
| Anthropic Message | `forwardMessageNative` | `MarshalAnthropicMessageBodyForModel` 前 |
| Anthropic Message | `forwardMessageViaChat` | `conv.FromAnthropicRequest` 后 |

设计原则：
- 通过环境变量 `COMPRESSION_ENABLED` 控制开关，默认关闭
- 压缩失败时 **fault-open**：原样透传，不中断请求
- 压缩结果（节省的 token 数、策略名）记录到审计

## 二、压缩管线内部设计

```
CompressPipeline.Compress(ctx, messages) {
    遍历 messages:
      1. 跳过 system/user/assistant 角色消息（只压缩 tool 返回）
      2. 跳过错误输出（检测到 traceback/stack trace 的消息，错误不压缩让模型自行修复）
          例外：内容超过 8000 字符的仍压缩（LogCompressor 会保留错误行）
      3. 跳过小于 min_chars（默认 500）的内容块
      4. 跳过排除工具名称的输出（可配置排除列表）
      5. 靠后的 N 条消息中的代码不压缩（避免压缩正在分析的目标代码，默认 4）
      6. ContentDetector.Detect(content) → ContentType + confidence
      7. Router 按类型 + 置信度选择压缩器（置信度低于阈值则透传）：
           JSON_ARRAY     → SmartCrusher
           SOURCE_CODE    → 透传（Phase 1 不压缩代码，避免破坏语义）
           SEARCH_RESULTS → SearchCompressor
           BUILD_OUTPUT   → LogCompressor
           GIT_DIFF       → DiffCompressor
           HTML           → 透传（Phase 1）
           PLAIN_TEXT     → 透传
      8. 压缩成功 → 替换 message content + 追加 digest marker
      9. 压缩失败/透传 → 原样保留
    返回 compressedMessages, Result{TokensBefore, TokensAfter, Strategies}
}
```

Phase 1 实现范围：SmartCrusher + ContentDetector + SearchCompressor + LogCompressor。

## 三、Go 模块结构

新增包 `internal/application/llmproxy/compression/`：

```
internal/application/llmproxy/compression/
  pipeline.go          // CompressPipeline 主入口，Pipeline 接口定义
  content_detector.go  // 内容类型检测（置信度评分）
  smart_crusher.go     // JSON 数组统计采样压缩
  search_compressor.go // 搜索结果压缩（解析、评分、选择）
  log_compressor.go    // 构建日志压缩（格式检测、分级、去重）
  diff_compressor.go   // Git diff 压缩
  config.go            // 从 viper 读取配置
```

依赖注入：在 `internal/bootstrap/container.go` 中注册，注入到 `openAIUseCase` 和 `anthropicUseCase`：

```go
type openAIUseCase struct {
    resolver         service.EndpointResolver
    modelsQuery      ListOpenAIModels
    openAIProxy      OpenAIProxyPort
    anthropicProxy   AnthropicProxyPort
    taskSubmitter    TaskSubmitter
    compressPipeline compression.Pipeline // 新增
}
```

`Pipeline` 接口：

```go
type Pipeline interface {
    Compress(ctx context.Context, messages []Message) ([]Message, *Result)
}

type Result struct {
    TokensBefore int
    TokensAfter  int
    Strategies   []string
}
```

## 四、SmartCrusher 算法

对 JSON 数组做统计采样，保留信息密度最高的行，丢弃冗余行。参数与 headroom `SmartCrusherConfig` 一致。

```
SmartCrusher.Crush(jsonStr) {
    1. 解析 JSON 数组 → items[]
    2. len(items) < min_items_to_analyze (5) → 透传
    3. 字符估算 < min_tokens_to_crush * 4 (200*4=800) → 透传
    4. 计算每个 item 统计特征：
       - 字段值方差（数值字段，variance_threshold=2.0）
       - 相邻 item 结构唯一性（uniqueness_threshold=0.1）
       - 相邻 item 相似度（similarity_threshold=0.8）
    5. 保留策略：
       a) 前 first_fraction (0.3) → 保留（开头最重要）
       b) 后 last_fraction (0.15) → 保留（结尾最新数据）
       c) 中间部分 → 保留变化点（方差/唯一性超过阈值的行）
       d) 相同 item 去重（dedup_identical_items=true）
    6. 压缩后保留最多 max_items_after_crush (15) 个 item
    7. 输出：压缩后 JSON 数组 + 丢弃行数标记
}
```

Go 实现要点：
- 用 `sonic` 做 JSON 解析/序列化
- 去重用 `map[string]bool`，key 为 sonic 序列化结果
- 变化点检测：相邻 item 字段值差异超过 variance_threshold 时标记
- 纯统计计算，不依赖外部 ML 模型

## 五、ContentDetector 内容类型检测

按优先级依次检测，每种检测返回 `(ContentType, confidence)`，confidence 低于阈值则透传。检测逻辑与 headroom `content_detector.py` 一致。

```
ContentDetector.Detect(content) → (ContentType, confidence):

    1. JSON 检测（最高优先级）:
       - 短内容: 以 [ 开头 → 尝试 sonic 解析
       - 解析成功且所有元素为 object → JSON_ARRAY, confidence=1.0
       - 解析成功但非全 object → JSON_ARRAY, confidence=0.8

    2. Diff 检测:
       - 扫描前 500 行，匹配 diff header（diff --git / --- a/ / @@ 行号）和变更行（^[+-][^+-]）
       - 有 header 匹配 → GIT_DIFF, confidence = min(1.0, 0.5 + headerMatches*0.2 + changeMatches*0.05)
       - 置信度 >= 0.7 才返回

    3. HTML 检测:
       - 扫描前 3000 字符，匹配 doctype/html/head/body 标签 + 统计结构标签（div/span/script等）
       - 至少有 3 个结构标签或 doctype → HTML, confidence = 累积权重
       - 置信度 >= 0.5 才返回

    4. 搜索结果检测:
       - 扫描前 100 行，统计 ^\S+:\d+: 格式匹配比例
       - 匹配比例 >= 30% → SEARCH_RESULTS, confidence = min(1.0, 0.4 + ratio*0.6)
       - 置信度 >= 0.6 才返回

    5. 构建日志检测:
       - 扫描前 200 行，匹配日志关键词（ERROR/WARN/INFO/DEBUG/FATAL/FAIL/PASS/timestamp/test results）
       - 匹配比例 >= 10% → BUILD_OUTPUT, confidence = min(1.0, 0.3 + ratio*0.5 + errorMatches*0.05)
       - 置信度 >= 0.5 才返回

    6. 代码检测:
       - 扫描前 100 行，按语言匹配代码特征（Python: def/class/import, Go: func/type/package, Rust: fn/impl 等）
       - 最佳语言匹配 >= 3 次 → SOURCE_CODE, confidence = min(1.0, 0.4 + ratio*0.4 + bestScore*0.02)
       - 置信度 >= 0.5 才返回

    7. 默认: PLAIN_TEXT, confidence=0.5
```

## 六、SearchCompressor 算法

解析 grep/ripgrep 输出，按文件汇总匹配行，评分后选择最重要的条目。算法与 headroom `search_compressor.py` 一致。

```
SearchCompressor.Compress(content, context) {
    1. 解析：按 ^文件:行号:内容 格式解析为 map[file][]SearchMatch
    2. 评分（为每个 match）：
       - 上下文关键词匹配：每个词 +0.3
       - 错误关键词匹配（Error/Exception/Fatal等）：+0.5 递减至 0.1
       - 配置关键词：+0.4
       - score = min(1.0, 各项之和)
    3. 选择（按文件 + 全局限制）：
       - 文件按总得分排序，取前 max_files (15) 个文件
       - 每个文件：always_keep_first + always_keep_last，中间按得分取满 max_matches_per_file (5)
       - 全局总计不超过自适应 max_total_matches (max 30)
    4. 输出：按文件+行号排序的压缩结果 + 省略摘要
}
```

## 七、LogCompressor 算法

检测日志格式、分级分类、保留错误和关键行。算法与 headroom `log_compressor.py` 一致。

```
LogCompressor.Compress(content) {
    1. 格式检测：识别 pytest/npm/cargo/make/jest/generic
    2. 逐行解析 → LogLine{level, isStackTrace, isSummary, score}:
       - level: ERROR/FAIL/WARN/INFO/DEBUG/TRACE → 对应得分 1.0/1.0/0.5/0.1/0.05/0.02
       - isStackTrace: 匹配 Traceback/at...\(/File "..." 等模式（状态机，穿过多行缩进）
       - isSummary: 匹配 ===/---/数字 passed/failed/TOTAL 等
       - 最终 score = min(1.0, levelScore + stackTrace*0.3 + summary*0.4)
    3. 分类汇总：
       - errors: 按得分选前 max_errors (10)，保留首末
       - fails: 同上
       - warnings: 保守去重后取前 max_warnings (5)
         去重算法：在第一个 :/= 处分割，仅标准化后缀的数字/路径，保留前缀消息标识
       - stackTraces: 取前 max_stack_traces (3)，每个最多 stack_trace_max_lines (20)
       - summaries: 保留所有
    4. 添加上下文行：error_context_lines (3) 前后行
    5. 自适应截断：总计不超过 max_total_lines (100)
    6. 输出：保留行按原始行号排序 + 省略统计摘要
}
```

## 八、审计集成

### ModelCallAuditTask 新增字段

```go
type ModelCallAuditTask struct {
    // ... 现有字段 ...
    CompressionEnabled  bool   // 是否启用压缩
    CompressedTokens    int    // 节省的 token 数
    CompressionStrategy string // 使用的压缩策略
}
```

### 节省 token 计算公式

在 `SetCompressionResult` 中，用上游返回的精确 `InputTokens` 换算：

```
CompressedTokens = InputTokens * (1 - len(compressed)/len(original))
```

即字符级压缩比 × 上游精确 token 数，避免单独引入 tokenizer。

### 写入时机

`CompressPipeline.Compress()` 内部记录 `compressRatio`，上游返回后在各 forward 方法中用公式计算 `CompressedTokens`，仅在 `CompressedTokens > 0` 时写入审计。

## 九、配置

在 `internal/config/config.go` 的 `initEnvironment()` 中新增：

```go
config.SetDefault("compression.enabled", false)
CompressionEnabled = config.GetBool("compression.enabled")
```

- viper key: `compression.enabled`
- 环境变量: `COMPRESSION_ENABLED`
- 默认: `false`
- `true` 时实例化 `Pipeline`，`false` 时注入 `noopPipeline`（`Compress()` 直接返回原 messages）
