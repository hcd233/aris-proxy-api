# 三接口 Mock 模型与生产压测设计

> 日期：2026-10-14 ｜ 状态：待评审 ｜ 相关：`2026-10-10-perf-qps-benchmark-design.md`

## 1. 背景与目标

为 chat completion / response / message 三个 LLM 转发接口各提供一个 **mock 模型**，随机输出长度随机的字符流；用它压测**生产环境** `api.lvlvko.top` 的真实处理链路；压测产生的数据写入真实库，事后按可识别标记**精确清理**。

要压测的「正式环节」= 完整转发链路：

```
API Key 鉴权 → 请求令牌桶限流 → 触发词扫描 → 模型解析(EndpointResolver)
  → 跨协议转换 → 响应下发(SSE) → 审计入账 → 会话/消息落库 → Token 扣减
```

**仅「向上游 LLM 发起 HTTP 请求」这一步由 mock 生成器替代**，其余全部真实执行。

**量化目标**：三接口在上述完整链路下能**同时维持的 SSE 长连接数**（核心容量指标，见 §4.3 口径定义）、延迟分位（p50/p90/p99）、首字节延迟、流式持续时间、错误率与状态码分布、输出字符吞吐。

## 2. 术语澄清

| 术语 | 定义 |
|------|------|
| **Mock 模型** | `Model` 表中 `upstream_model` 为固定标记值 `mock-llm` 的模型别名。命中该标记时网关不向上游发 HTTP，改由进程内随机流生成器产出响应。 |
| **随机字符流** | 长度在 `[1, 16384]` 随机的可打印 ASCII 字符序列，按随机大小分片以 SSE 吐出。 |
| **压测目标** | 生产 `api.lvlvko.top` 的**线上服务真实端点**，区别于 perf-qps-benchmark 的独立被测实例。 |
| **压测标识** | 压测流量的双重可判定标记：专用 API Key（主）+ 专用 User-Agent（辅），用于事后精确清理。 |

## 3. 与 `2026-10-10-perf-qps-benchmark-design.md` 的关系

两份设计目标不同，**不合并**，但共享一套数据隔离方法学：

| 维度 | perf-qps-benchmark（已有） | 本设计 |
|------|---------------------------|--------|
| 目的 | 量化 7 项优化的 QPS 收益 | 压测三接口的真实链路表现 |
| 压测对象 | 独立被测实例 `127.0.0.1:18080`（明令不压线上端口） | **线上服务真实端点** |
| mock 形态 | 独立 HTTP 上游服务（`127.0.0.1:18090`），返回固定体积 JSON/SSE | 进程内 Proxy Port 装饰器，返回**随机长度随机内容**字符流 |
| 压测工具 | wrk | **wrk**（同一套工具链，见 §4.3） |
| 数据隔离 | 专用 Key `perf-bench-2026-10` + 按 `api_key_id` 清理 | **沿用同一方法学**，见 §4.4 |

**沿用项**：专用 API Key 标记写入、按 `api_key_id` 精确清理、清理后留存语句与证据、warmup + 多轮取中位数、低峰期执行。

## 4. 核心设计

### 4.1 Mock 模型接入 —— Proxy Port 装饰器

**选型**：在 `OpenAIProxyPort` / `AnthropicProxyPort`（`internal/application/llmproxy/usecase/port.go`）外包一层装饰器，检测到 mock 标记即本地生成响应，不发 HTTP。

**选型理由**（对比另两案）：

| 案 | 做法 | 判定 |
|----|------|------|
| **A. Proxy Port 装饰器（采用）** | 包装两个上游端口接口 | usecase **零改动**，7 条跨协议路径自动全覆盖；审计/存储/限流/触发词全部真实跑完 |
| B. usecase dispatch 层短路 | 在 `dispatchXxx` 前构造 `port.Result` | 需改 3 个 usecase 且各自重接审计/存储，易漏；跨协议转换压不到 |
| C. 独立 mock HTTP 上游 | 配 baseURL 指向本地 mock server | 已否：真发 HTTP 混入网络噪声，且需额外进程 |

**标记载体**：`vo.UpstreamEndpoint.Model` 即 `Model.UpstreamModel()`（`toTransportEndpoint` 构造），装饰器判定 `ep.Model == "mock-llm"` 即走 mock。`upstream_model` 的既有语义就是「向上游发送的模型名」，用它承载标记天然无侵入。

**数据侧配置**（非硬编码）：

1. **1 个 mock Endpoint**：能力标记开 `supportOpenAIChat` / `supportOpenAIResponse` / `supportAnthropicMessage`；`openaiBaseURL` / `anthropicBaseURL` 填占位地址（如 `http://mock.invalid`）——**不可留空**，受 586414fe 的「能力标记与 baseURL 不变量」约束，留空会导致该行读不出来。
2. **3 个 Model 别名**：`mock-chat` / `mock-response` / `mock-messages`，`upstream_model` 统一为 `mock-llm`。三个别名便于按接口分别压测与统计（审计 `model_id` 天然分组）。

**装饰器需实现的方法**（覆盖 7 条转发路径 + Decision/CountTokens）：

- `OpenAIProxyPort`：`ForwardChatCompletion`、`OpenChatCompletionStream` + `ReadChatCompletionStream`、`ForwardCreateResponse`、`OpenCreateResponseStream` + `ReadCreateResponseStream`、`ForwardCreateDecision`
- `AnthropicProxyPort`：`ForwardCreateMessage`、`OpenCreateMessageStream` + `ReadCreateMessageStream`、`ForwardCountTokens`

非 mock 请求原样透传给内层真实 proxy，零行为改变。

**安全开关**：装饰器仅在配置 `MOCK_UPSTREAM_ENABLED=true` 时启用，默认 `false`。防止生产上误配 mock 模型后拿到假响应而不自知；压测期间临时开启，压测后关闭。这是本设计唯一的生产配置改动。

### 4.2 随机字符流生成器

| 参数 | 取值 | 说明 |
|------|------|------|
| 长度 | `min(max_tokens, 随机(1..16384))` | 上限 16384（用户确认）；请求带 `max_tokens` 时取小 |
| 字符集 | 可打印 ASCII `0x20–0x7E` + `\n` | 默认纯 ASCII；可选含中文以覆盖多字节路径（后续扩展） |
| **TTFT**（首 token 延迟） | 随机 `0~1s` | **请求级随机**：每次请求抽一次，流开始前等待该时长 |
| **TPS**（输出速率） | 随机 `20~200 tok/s` | **请求级随机**：每次请求抽一次，全程按此速率匀速吐出 |
| 分片 | 每片 4 字符 ≈ 1 token | 与 TPS 口径对齐；片间间隔 = `1/TPS` 秒 |
| usage 估算 | `output_tokens = max(1, len(chars)/4)` | 1 token ≈ 4 字符的粗略口径，保证审计入账有值、计费链路可跑 |
| input_tokens | 从请求 body 拼接文本长度 `/4` 估算 | 同上 |

**时序模型**（每次请求独立抽参）：

```
T = 0        收到请求
T = TTFT     吐出首片（首 token）          ← TTFT ∈ [0, 1s] 随机
T = TTFT + k/TPS   吐出第 k+1 片          ← TPS ∈ [20, 200] tok/s 随机，匀速
T = TTFT + (N-1)/TPS  流结束              ← N = 总 token 数 = 字符数 / 4
```

总流时长 = `TTFT + (N-1)/TPS`。**长流边界与分布**（均匀抽样下）：

| 量 | 取值 |
|----|------|
| 字符数 | U(1, 16384)，期望 ≈ 8192 |
| token 数 | 字符/4，期望 ≈ 2048 |
| TPS | U(20, 200)，期望 ≈ 110 |
| **流时长期望** | ≈ **19 秒**（输出段 18.6s + TTFT 0.5s） |
| **流时长最长** | ≈ **205 秒**（16384 字符 → 4096 token ÷ 20 tok/s） |

流时长是**明显长尾**分布。服务侧 `WRITE_TIMEOUT` 已是 30m，不会切断该流（见 `fiber-write-timeout-sse-disconnect` 教训）。

三种协议分别构造流式与非流式响应，字段严格对齐 `internal/dto` 的既有结构：

- **Chat Completions**：流式吐 `OpenAIChatCompletionChunk`，`Read` 聚合出 `OpenAIChatCompletion`
- **Responses**：流式吐 `response.output_text.delta` 等事件（`onEvent(event, data)`）
- **Messages**：流式吐 `AnthropicSSEEvent`（`content_block_delta`），`Read` 聚合出 `AnthropicMessage`

非流式直接返回完整响应。Decision 不属本次范围：mock Endpoint 不开 `supportOpenAIDecision` 能力。`ForwardCountTokens` 返回按同样口径估算的 token 数——它是 `POST /messages/count_tokens` 独立端点的实现，mock 模型被调用时也须给出合法值，否则该端点对 mock 模型报错。

**压测 prompt 必须带固定标记前缀**（如 `__LOADTEST__`）：`messages` / `tools` 按 checksum 内容寻址去重，若压测 prompt 与某条真实历史消息内容相同，会复用同一行，清理时将误删真实消息。带标记前缀保证 checksum 与任何真实消息不同，从源头杜绝误删。

### 4.3 压测工具 —— wrk

**选型 wrk**（C 实现，业界成熟高性能标准），与 `2026-10-10-perf-qps-benchmark` 保持同一套工具链。

**选型理由**：

1. **工具链统一**：perf-qps-benchmark 已用 wrk，两份设计共享脚本约定、指标口径与复现方式。
2. **极致性能**：C 实现 + 多线程，能压满服务端极限；被测侧是生产 2 pod × 300m CPU（实测 `/health` ~1650 QPS），需要压测端不成为瓶颈。
3. **Lua 构造三协议**：`request()` 构造 POST JSON body 与自定义头，`response()` 校验响应，`done()` 出自定义汇总。

**脚本形态**：`test/loadtest/` 下 wrk Lua 脚本，三个脚本（或一个按参数切换）分别压三接口。

- 参数（环境变量注入 Lua）：`TARGET_URL`、`API_KEY`、`MODEL`、`STREAM`、`MAX_TOKENS`
- 请求头：`X-API-Key`（压测专用 Key）、`User-Agent: aris-loadtest/1.0`（清理辅标识）
- 请求体：user 消息带固定标记前缀 `__LOADTEST__`（见 §4.2 末段，防 checksum 复用真实消息）
- 执行：`wrk -t4 -c<并发> -d<时长> --latency -s <script> <target>`

**指标来源分两侧**（关键设计）：

| 指标 | 来源 | 说明 |
|------|------|------|
| **并发 SSE 长连接数**（核心） | **wrk 侧** | wrk `-c` 即同时维持的连接数；逐档加压找稳定拐点 |
| 完整延迟、p50/75/90/99、错误数 | **wrk 侧** | `--latency` 输出；SSE 流式的 latency = 整个流完成时间 |
| 连接建立速率、状态码分布、响应字节数 | **wrk 侧** | Lua `response()` 累计、`done()` 输出 |
| **TTFT（首 token 延迟）** | **服务侧审计表** | `model_call_audits.first_token_latency_ms` |
| **流式持续时间** | **服务侧审计表** | `model_call_audits.stream_duration_ms` |

**为什么 TTFT/流持续取服务侧**：wrk 的 latency 是「请求发出 → 响应读完」的整体时长，Lua 层拿不到精确的首字节时间戳，测 SSE 的 TTFB 不可靠。而审计表已在 `recordModelCall` 收尾时落了 `first_token_latency_ms` 与 `stream_duration_ms` 两列真值（口径见 CONTEXT.md「CallLatency」），压测后按 `api_key_id` 查询统计即可，比压测端反推更准。

**执行方法学**：**采用「延长压测窗口」而非约束 `max_tokens`**（用户确认），保证随机长度 1~16384 全覆盖。

- **加压方式（容量测试）**：逐档递增并发找容量拐点——从 50 并发起按 50/100/200/400/… 加压，每档 warmup 60s → 正式 **5 分钟 × 3 轮**取中位数并报极差，直到出现拐点（错误率上升 / 延迟陡增 / 连接被拒）。
- **核心指标口径（用户定）**：本场景是 SSE 长连接流式，一个请求可持续 205 秒，传统「每秒完成请求数」完全不反映承载能力。**QPS 的口径 = 服务器同时维持的 SSE 长连接数**（并发流数），**能稳定维持的最大并发流数即核心结论**。
- 窗口 5 分钟 > 最长流 205 秒，保证极端长流也能在窗口内完成。
- 流时长呈长尾（期望 19s，最长 205s，见 §4.2），故另按流时长分层报完成数与延迟（短流 <30s / 中流 30~120s / 长流 >120s），观察长流占比对并发承载的影响。
- 连接超时须 > 205 秒（wrk 侧无此限制，但中间层/反代需确认）。

### 4.4 数据清理

**双标识，可精确删除**：

- **主标识**：压测专用 API Key（单独创建，记录 `api_key_id`）。审计表 `api_key_id` 有复合索引 `idx_mca_apikey_created`，sessions 表 `api_key_id` 有索引 `idx_sessions_api_key_id`，查询与删除都走索引。
- **辅标识**：User-Agent `aris-loadtest/1.0`，审计表 `user_agent` 列可二次校验，防误删。

**清理顺序**（`scripts/loadtest-cleanup.sql`，事务包裹，先预览后执行）：

1. **预览核对**：`SELECT count(*)` 统计待删的 `model_call_audits` 与 `sessions` 行数，与压测预期量级比对。
2. 删 `model_call_audits WHERE api_key_id = :api_key_id`。
3. 软删 `sessions WHERE api_key_id = :api_key_id`（`deleted_at` 置非 0 值；活跃判定为 `deleted_at = 0`，见 `BaseModel`）。
4. **硬删孤儿 `messages` / `tools`**：从被软删 sessions 的 `message_ids` / `tool_ids`（JSON 数组列，用 `jsonb_array_elements_text` 展开）中，减去仍被**其他活跃 session** 引用的 ID，剩余即孤儿，硬删。逻辑等价 `SoftDeletePurge` cron 的差集算法，只是按 `api_key_id` 定向。
5. 硬删已软删的 sessions（可选，或留给 `SoftDeletePurge` 自然处理）。

**幂等**：重复执行无副作用（第二次删 0 行）。

**Redis 无需清理**：请求令牌桶按 `api_key_id` 独立且带 TTL，token 扣减桶同理，endpoint 亲和 TTL 5 分钟，均自然过期；触发词计数因压测文本为随机字符、不含触发词，风险可忽略（如需彻底可压测前记录 `trigger:hit:*` 快照后比对恢复）。

**异步落库时序**：审计/会话走 Pond 协程池异步写，压测结束后需等落库完成再清理。cleanup 提供 `--wait`（默认等 30s）或重复执行至删除行数为 0。

### 4.5 压测资产沉淀（每次压测留档）

**用户要求**：每次压测的脚本都要沉淀起来。压测不是一次性动作，必须可追溯、可复现、可跨轮对比。

**两层沉淀**：

| 层 | 位置 | 内容 | 生命周期 |
|----|------|------|---------|
| **脚本资产** | `test/loadtest/*.lua` | 三协议 wrk Lua 脚本 + 通用参数注入 | 进 git 长期维护，改动走正常提交 |
| **压测档案** | `docs/loadtest/YYYY-MM-DD-<topic>.md` | 该次压测的完整记录 | 每次压测一份，只增不改 |

**压测档案必备字段**（缺一不可，保证可复现）：

1. **压测参数**：目标 URL、模型别名、并发档、`--duration`、`stream` 开关、`max_tokens`、wrk 完整命令行
2. **环境信息**：git commit、被测服务镜像/版本、pod 数与 CPU 配额、执行时间（低峰期证据）
3. **脚本引用**：所用 Lua 脚本的 git commit 或版本号（脚本本身在 `test/loadtest/`，档案只引用不复制）
4. **原始数据**：wrk stdout 原文存 `docs/loadtest/raw/YYYY-MM-DD-<topic>/`（含 `--latency` 分位输出）
5. **服务侧数据**：审计表统计（TTFT、流持续、状态码分布、token 量），附查询 SQL
6. **结论**：并发 SSE 长连接数（稳定拐点）、延迟分位、TTFT/流持续分布、错误率，及与历史轮次的对比

**命名约定**：`YYYY-MM-DD-<topic>`，topic 用小写连字符（如 `2026-10-20-baseline-three-apis`），同一主题多轮用 `-v2`、`-v3` 后缀。

**与 perf-qps-benchmark 的关系**：那份设计用 `docs/perf/raw/` 存原始 wrk 输出，本设计用 `docs/loadtest/raw/`，两者独立不混（目的不同：那边量化 7 项优化收益，这边测三接口链路表现），但档案字段结构保持一致风格，便于横向阅读。

## 5. 测试与验证

**单测**（`test/unit/`）：
- 随机流生成器：长度分布落在 `[1,16384]`、字符集合法、`max_tokens` 约束生效、usage 估算口径
- **时序参数**：TTFT 落在 `[0,1s]`、TPS 落在 `[20,200]`、片间间隔 = `1/TPS`、总流时长 = `TTFT+(N-1)/TPS` 的时序正确性
- mock 装饰器：三协议流式/非流式响应结构正确、非 mock 请求零行为改变、`MOCK_UPSTREAM_ENABLED=false` 时不生效
- 清理 SQL：选择条件精确（不误删他 Key 数据）、孤儿差集正确、幂等

**E2E**（`test/e2e/loadtest_mock/`）：
- 走真实链路调三个接口的 mock 模型，断言响应是随机字符流且长度合规
- 断言审计行与 session/message 确实落库（`api_key_id` 正确）
- 执行清理后断言相关表归零、他 Key 数据不变

**验证命令**（沿用 `docs/agents/commands.md`）：`make test`、`make lint`，前端无改动。

## 6. 安全与风险

| 风险 | 缓解 |
|------|------|
| 压测挤占生产资源，影响真实用户 | 生产仅 2 pod × 300m CPU；低峰期执行、并发与时长可控、异常立即停止；执行前公告 |
| 写真实库，清理失败污染数据 | 双标识精确匹配 + 预览核对行数 + 事务包裹 + 幂等可重跑 + 清理后留存语句与证据 |
| 误删真实数据 | 清理 SQL 强制带 `api_key_id` 条件；预览先行；辅以 `user_agent` 校验；清理前备份待删行数 |
| **压测 prompt 与真实历史消息 checksum 撞车，复用同一行致误删** | 压测 prompt 固定带 `__LOADTEST__` 前缀，保证 checksum 不与任何真实消息相同（见 §4.2） |
| mock 模型被误用于真实场景 | `MOCK_UPSTREAM_ENABLED` 默认关闭；mock 别名带 `mock-` 前缀易识别；压测后删除 mock 模型/Endpoint 配置 |
| 异步落库未完成即清理，残留数据 | cleanup 内置等待 + 幂等重跑至删除行数为 0 |
| 长流长时间占满并发连接，干扰承载能力测量 | 窗口 5 分钟容纳最长流 205s（已定方案 b）；核心指标用**并发 SSE 长连接数**而非完成速率；另按流时长分层报数据 |
| 压测文本偶含触发词 | 随机 ASCII 字符集不含中文触发词；如命中 deny 会返回 content_filter，wrk Lua `response()` 校验状态码与响应形态会暴露 |

**生产凭据**：遵循 `login-prod-server` skill 安全基线，凭据不进仓库、不进文档。

## 7. 交付物与验收

**交付物**：
1. mock 装饰器 + 随机流生成器（Go，含配置开关）
2. 3 个 mock 模型的配置说明（Endpoint + Model 别名，含 `upstream_model` 标记值）
3. wrk 压测脚本（`test/loadtest/`，三协议 Lua 脚本）
4. 清理 SQL 脚本（`scripts/loadtest-cleanup.sql`，含预览）
5. 单测 + E2E 用例
6. 压测结果报告（并发 SSE 长连接数 / 延迟分位 / 错误率 ← wrk 侧；TTFT / 流持续 ← 审计表侧；三接口各一份）
7. **压测档案**：`docs/loadtest/` 每次压测一份，含参数、环境、原始数据、服务侧统计、结论（见 §4.5）

**验收标准**：
- [ ] 三个接口均能通过 mock 模型拿到随机字符流（流式 + 非流式），TTFT/TPS 时序符合设计
- [ ] 非 mock 请求链路零行为改变，全量测试通过
- [ ] `MOCK_UPSTREAM_ENABLED=false` 时 mock 不生效
- [ ] 压测脚本能对生产三接口出数，核心指标（**并发 SSE 长连接数**）与流时长分层口径齐全
- [ ] **每次压测的脚本与档案均沉淀入库**，档案字段齐全可复现
- [ ] 清理脚本能精确删除压测数据且不动真实数据，幂等可重跑
- [ ] 清理后生产库无残留，留存清理语句与行数证据
- [ ] mock 模型/Endpoint 配置**保留**，仅靠 `MOCK_UPSTREAM_ENABLED=false` 关闭

## 8. 已定决策（2026-10-14 用户确认）

1. **长流 vs 压测窗口** → 选 **(b) 延长压测窗口**：`--duration` 5 分钟（> 最长流 205s），**不约束 `max_tokens`**，保证随机长度 1~16384 全覆盖。另按流时长分层统计（见 §4.3）。
2. **随机字符集** → **纯 ASCII**（`0x20–0x7E` + `\n`），不含中文/多字节。
3. **执行时机** → 低峰期执行并提前公告。
4. **mock 配置留存** → **保留** mock 模型 / Endpoint 配置，长期复用做回归压测；靠 `MOCK_UPSTREAM_ENABLED=false`（默认关）控制生效，避免生产误用假模型。
5. **QPS 口径** → **服务器同时维持的 SSE 长连接数**（并发流数），**不是**「每秒完成请求数」。SSE 长连接场景下一个请求可持续 205 秒，完成速率完全不反映承载能力。

另两项早前已定：压测工具 = **wrk**（与 perf-qps-benchmark 工具链统一）；mock 标记值 = **`mock-llm`**。

**无遗留未决问题**，可进入实施计划阶段。
