# 端点调度与成本增强（#13~#17）设计文档

> 日期：2026-10-09
> 分支：`feature/endpoint-scheduling-pricing-2026-10-09`
> 来源：对 QuantumNous/new-api 的调研结论，选取其中 5 个高/中高价值功能点迁移（调研报告编号 #13~#17）

## 1. 背景与目标

调研 new-api（自托管 AI 网关）后确定 5 个值得迁移的功能点，补齐 aris-proxy-api 在「端点调度」「成本核算」「配置治理」「调试体验」四方面的缺口：

| 编号 | 功能 | 解决的问题 |
|---|---|---|
| #13 | Claude 缓存计价细分（5m/1h） | Anthropic 缓存创建有 5m/1h 两档单价，现只有单档 `cache_creation_price`，成本估算失真 |
| #14 | 配置健康检测 | 模型未定价/未填规格只能逐个点开发现，缺全局视角 |
| #15 | Playground 模型调试台 | 配置别名/端点/触发词后只能用外部客户端验证，缺内置调试入口 |
| #16 | 端点亲和性路由 | 同一会话的请求随机打到不同端点，上游 prompt cache 命中率低、成本高 |
| #17 | 优先级/权重调度 + 跨端点 fallback | `EndpointResolver` 随机选择、一次定胜负，上游故障直接报错，多端点形同虚设 |

**非目标**（明确不做）：new-api 的支付/订阅/兑换码、任务插件平台、多密钥轮换、API Key 细化管控、端点连通性测试、自动禁用/恢复端点、Webhook 通知、排行榜/模型广场。以上为后续独立任务或明确不迁移。

## 2. 决策记录

| 决策点 | 结论 | 理由 |
|---|---|---|
| 实施组织 | 一份设计文档 + 一份实施计划，5 阶段顺序交付，单 worktree 分支 | 各阶段可独立验证，避免多分支上下文重复加载 |
| 调度参数挂载点 | `models` 表关联行加 `priority`/`weight`（alias↔endpoint 关联行即调度单元） | 不同 alias 对同一端点可有不同偏好；纯增量字段 |
| 重试策略 | **不新造轮子**：复用 `transport/retry.go` 的同端点重试（指数退避、`IsRetryableError`），仅在其上新增跨端点切换 | aris 已有传输层重试；#17 增量 = 跨端点 fallback + 优先级排序 |
| 亲和键 | 组合键：会话头（`x-opencode-session`/`X-Session-Id`）优先，回退 `sha256(alias + 首条 user 消息)` 指纹 | 覆盖 Agentic 客户端与裸 curl 两类流量 |
| Playground 数据归属 | 不落 session/message/tool，仅落审计 | 调试流量不污染会话数据集（ShareGPT 导出质量敏感），但用量可观测 |

## 3. 阶段 1 · Claude 缓存计价细分（5m/1h）

### 3.1 口径

Anthropic 响应 `usage.cache_creation` 新版为对象（含 `ephemeral_5m_input_tokens` / `ephemeral_1h_input_tokens`），旧版只有 `cache_creation_input_tokens` 总量字段。两者都解析，新版优先；旧版读数全部计入 5m 档（Anthropic 默认 TTL 即 5m，语义最接近）。

### 3.2 最小迁移方案

不拆存量列、不改存量语义：

- **审计表只新增一列** `cache_creation_1h_input_tokens`（存量行 = 0）。`cache_creation_input_tokens` 保持「总量」语义不变，5m 量 = 总量 − 1h。统计 SQL 零破坏。
- **定价规则保留 `cache_creation_price`**，语义变更为「5m 缓存创建单价，兼作 1h 未配置时的回落价」；新增可选字段 `cache_creation_1h_price`。存量规则零迁移、零回填。
- **费用计算**：`cacheCreationCost = 5m_tokens × price(5m) + 1h_tokens × price(1h 缺省回落 5m 价)`，仍为请求时计算落库（`cost_micro` 不随改价漂移）。
- **`TokenBreakdown`**（`domain/modelcall/vo`）：新增 `cacheCreation1h` 字段，`CacheCreation()` 保持返回总量（5m+1h），全链路调用方零改动。
- **DTO 兼容**：Anthropic usage DTO 同时接受 `cache_creation` 对象与 `cache_creation_input_tokens` 总量字段；OpenAI/Response 无 1h 概念，1h 恒为 0。
- **models.dev 导入**：`cacheWrite` 单价仍映射到 `cache_creation_price`（5m 档），不猜测 1h 价。
- **前端**：定价弹窗加可选「1h 缓存创建价」输入（留空 = 回落 5m 价，占位文案说明）。

### 3.3 TokenAccounting 口径更新

四维互斥口径扩展为：`净输入 + 缓存创建(5m) + 缓存创建(1h) + 缓存读取 = 上游口径输入总量`。`CONTEXT.md` 的 TokenAccounting / ModelPricing / EstimatedCost 词条需同步更新。

## 4. 阶段 2 · 配置健康检测

不加新接口：`GET /api/web/v1/model/list` 响应每项新增 `configMissing []string`：

- `"pricing"`：`pricing_currency == ""`（未计价）。currency=USD 的免费模型（四价全 0）不算缺失。
- `"spec"`：`context_length == 0`（未填规格）。

前端 models 页：

- 每行徽标显示缺失项（未定价 / 未填规格）。
- 筛选 chip「仅看配置缺失」。
- 补齐动作复用现有 SpecPrefill 按钮与定价弹窗，不新增工作流。

多租户语义不变：user 看自己名下模型，admin 全量并可按用户过滤（列表既有行为）。

## 5. 阶段 3 · 优先级/权重调度 + 跨端点 fallback

### 5.1 数据模型

`models` 表新增：

- `priority int not null default 0`：数字小 = 优先级高，同值归同一档。
- `weight int not null default 1`：同档内加权随机权重（正整数）。

`Model` 聚合、Create/Update 命令与 DTO 同步扩展；models 页弹窗加两个输入（priority 默认 0，weight 默认 1）。

### 5.2 EndpointResolver 重写

`Resolve` 替换为 **`ResolveCandidates`**，返回有序候选列表 `[]{Endpoint, Model}`。3 个 usecase 调用点（`openai.go` ×2、`anthropic.go` ×1）全部迁移，旧 `Resolve` 删除。

排序算法：

1. 收集 alias 候选（沿用现有多租户隔离 + 共享池回退逻辑）。
2. 过滤 `enabled` + 协议 `matcher`。
3. 按 `priority` 升序分档；档内按 `weight` 加权洗牌（A-Res 算法：每个候选取 `key = rand^(1/weight)` 按 key 降序出队，权重越大越靠前，无需预展开）。
4. （阶段 4）亲和命中的候选提到最前。

### 5.3 usecase 候选遍历

逐候选调用现有 `forward*` 路径（transport 内部 `SendUpstreamWithRetry` 同端点退避重试**原样保留**，Guard 熔断租约机制不动）。

**换端点判定**：`transport.IsRetryableError(err)`（连接错误 / 5xx / 429）或 Guard 熔断/满载拒绝（`ProxyError` 503 —— 端点熔断正是换端点的时机）。候选耗尽后返回最后一次错误。

**审计**：仅最终结果进现有审计（最终失败记最后尝试的端点），中间切换不单独记审计；切换过程记日志（见 §8）。

**重试放大权衡（明确取舍）**：最坏尝试次数 = 候选数 × (maxAttempts+1)。熔断打开的端点在 Guard 层快速失败剪枝；429 的同端点退避重试仍有价值。不加预算协调层，如实际观测到延迟放大再引入。

**流式安全**：仅在流建立前（`OpenChatCompletionStream` 等返回 error）可换端点；流一旦交给 handler 开始输出，失败即失败。

## 6. 阶段 4 · 端点亲和性路由

### 6.1 亲和键（组合）

1. 优先：请求头 `x-opencode-session` / `X-Session-Id`（复用 `constant.HTTPHeaderOpencodeSession` / `HTTPHeaderSessionID`，即现有 header_passthrough 机制识别的会话头）。
2. 回退：`sha256(alias + "\x00" + 首条 user 消息文本)` 指纹（多模态内容取 text parts 拼接，无 text 则空串）——同会话多轮稳定（首条 user 消息不变）；纯 curl 单轮每次新指纹，无粘滞但等同现状，可接受。

### 6.2 存储与生命周期

- Redis key：`affinity:{userID}:{alias}:{affinityKey}` → endpointID（string），TTL 5 分钟。
- 写入时机：转发成功后 `SET` 刷新；fallback 换端点成功后改写映射到新端点。
- 读取时机：`ResolveCandidates` 排序阶段——亲和 endpointID 在候选集中则提到最前；不在（被禁用/被 matcher 过滤/已删除）则忽略亲和。

多 Pod 共享 Redis，亲和状态跨 Pod 一致。用户维度前缀保证多租户隔离。

## 7. 阶段 5 · Playground 模型调试台

### 7.1 后端

`POST /api/web/v1/playground/chat`：

- 鉴权：`jwtAuth`，权限 ≥ `PermissionUser`（demo/pending 天然拒绝；DemoConfig 模块白名单不含 playground，fail-closed）。
- Body 复用 `dto.OpenAIChatCompletionRequest`，走现有 llmproxy usecase 全链路（别名解析、跨协议转换、Guard、触发词、限流）。
- **存储分流**：context 注入 `CtxKeyPlayground`（注册到 `constant/ctx.go`），store 路径跳过 session/message/tool 沉淀，审计照常（token/延迟/成本可观测）。
- 响应：流式 SSE 与非流式 JSON 均支持（huma `StreamResponse` 透传）。
- 限流：复用令牌桶组件，按 **userID** 维度（现有 LLM 路径按 APIKeyID，playground 无 API Key）。

### 7.2 前端

新页 `(dashboard)/playground`：

- 别名下拉（复用 `GET /model/list`，仅 enabled 项）。
- 参数面板：stream / temperature / max_tokens。
- 多轮消息编辑器（role=user/assistant，可加 system）。
- SSE 流式渲染、错误展示（错误码 toast 复用 `src/lib` 既有机制）。
- Nav 加入口（user/admin 可见，demo 不可见）。

## 8. 错误处理与边界

- 流式建立后失败不换端点（见 §5.3）。
- 亲和端点被 `matcher` 过滤 → 忽略亲和，按序遍历。
- 1h 用量存在但 `cache_creation_1h_price` 未配置 → 按 5m 价回落计费（与「四价全 0 = 免费」语义不冲突：免费判定仅在 currency 非空且全部四价为 0 时成立）。
- 换端点日志格式：`[OpenAIUseCase]` / `[AnthropicUseCase]` 前缀，含 `from→to` endpoint 名与触发原因（错误类型 + 状态码），供 CLS 排障。
- 亲和 Redis 读写失败 fail-open：忽略亲和按序遍历，不阻塞转发。
- `weight <= 0` 输入校验拒绝（ierr.ErrValidation）；`priority` 允许负数（提前档位）。

## 9. 测试与验证

**单元测试**（`test/unit/`）：

| 主题 | 覆盖点 |
|---|---|
| `pricing_5m1h_cost` | 5m/1h 分档计价、1h 回落 5m 价、旧版总量字段归 5m、免费/未计价判定不变 |
| `endpoint_resolver`（扩展现有） | 候选排序（priority 分档）、weight 加权分布、亲和置顶、亲和失效忽略、共享池回退保留 |
| `affinity_key` | 会话头优先、指纹稳定性（同会话多轮同键）、无头无指纹退化 |
| `model_config_missing` | pricing/spec 缺失判定、免费模型不算缺 |

**E2E**（`test/e2e/`）：

| 主题 | 覆盖点 |
|---|---|
| `endpoint_fallback` | 首候选 5xx → 自动换次候选成功；全失败返回最后错误 |
| `playground_chat` | 流式与非流式调用成功、审计有记录、session/message 无记录 |

**回归**：全量 `go test ./...`、`make lint`、前端 `npm run lint / test / build`。

**DB 迁移**：全部为加列（`models.priority`/`weight`、`model_call_audits.cache_creation_1h_input_tokens`）与 JSON 列新增可选字段（`pricing_rules[].cache_creation_1h_price`），AutoMigrate 覆盖，无需 `migrate-data`。

## 10. 交付顺序与验收标准

| 阶段 | 验收标准 |
|---|---|
| 1 计价细分 | Anthropic 5m/1h usage 分档计价落库；旧格式归 5m；存量统计不受影响；定价弹窗可配 1h 价 |
| 2 配置检测 | model/list 返回 configMissing；前端徽标 + 筛选可用 |
| 3 调度 fallback | priority/weight 生效（排序可测）；E2E 首端点失败自动切换成功 |
| 4 亲和路由 | 同会话多轮同端点（Redis 可查亲和键）；TTL 过期后重新分配 |
| 5 Playground | 页面可流式对话；审计有记录、会话库无记录；demo 被拒 |

每阶段测试全绿后进入下一阶段；全部完成后按项目流程跑 `ponytail-review` 审查 diff 过度工程，询问用户是否提交/合并。
