# OpenAI Decision API 配置与代理 — 设计文档

> 参考协议文档：`docs/openai/create_decisions.md`（OpenAI `POST /v1/decisions`）
> 分支：`feature/openai-decision-api-2026-10-09`
> 状态：待用户评审

## 1. 背景与目标

网关已代理 OpenAI Chat Completions、OpenAI Response、Anthropic Messages 三类协议，
并通过 Endpoint 能力开关（`support_openai_chat_completion` / `support_openai_response` /
`support_anthropic_message`）决定某个上游端点能承担哪些接口。

OpenAI Decision API（`POST /v1/decisions`）用于「对同一份输入批量提问分类/打分」：
请求携带 `input`（文本或含文本+图片的 user 消息）与 `questions`（`predicate` / `choice` /
`score` 三类问题），响应按提问顺序返回 `answers`，单题可以是 `predicate` / `choice` / `score`
或 `refusal`（模型拒答）。

**目标**：为 Decision API 补齐「配置」与「代理」两块能力，使客户端可以
`POST /api/openai/v1/decisions` 经网关访问已声明支持该接口的上游端点，并产生与既有接口
同口径的审计、计费与内容安全行为。

## 2. 范围

### 2.1 包含

1. Endpoint 新增能力开关 `support_openai_decision`（DB 列 → 领域聚合 → 仓储 → 应用层命令 → DTO → Web 管理端）。
2. 新增 `POST /api/openai/v1/decisions` 代理路由（API Key 鉴权 + 双限流 + body 上限）。
3. Decision 请求/响应 DTO（联合类型处理）。
4. 上游传输 `ForwardCreateDecision`（unary，复用既有熔断/重试/drain/header 透传）。
5. 用例编排：别名解析、模型名双向改写、触发词拦截、会话存储、审计与计费。
6. 前端：端点弹窗能力复选项、端点分组视图协议徽标、审计页协议标签。
7. 测试：单元测试 + E2E 用例。

### 2.2 明确不做（YAGNI）

| 不做项 | 理由 |
|---|---|
| 跨协议转换（Decision ↔ Chat/Anthropic） | `predicate`/`choice`/`score` 在 Chat/Anthropic 协议中没有等价语义，转换等于自造一套 prompt 协议，收益与维护成本不成比例 |
| 流式（SSE）支持 | 官方文档 Body Parameters 无 `stream` 参数，接口本身只有 unary 形态 |
| Decision 专属响应 DTO 全量建模 | 响应只需读 `model` / `answers` / `usage` 三处，走 raw 透传 + 部分解析更少代码、天然前向兼容（与 `/responses` unary 同范式） |
| 触发词 `capture` 短路（返回固定回复 + 存历史） | `capture` 语义依赖「最后一条用户提问」，Decision 的 `input` 是待评估文本而非多轮对话，无对应概念 |
| `safety_identifier` 的独立落库 | 既有 `MessageStoreTask.Metadata` 为 `map[string]string`，Decision 无 `metadata` 字段；不额外发明存储字段 |

## 3. 术语（需回写 `CONTEXT.md`）

- **DecisionAPI（决策接口）**：OpenAI `POST /v1/decisions`，对同一 `input` 批量回答分类/打分问题，
  返回按提问顺序排列的 `answers`。
- **DecisionQuestion（决策问题）**：`predicate`（估算陈述为真的概率）/ `choice`（从 2~255 个选项中选择）/
  `score`（按有序等级打分）三型，均可带可选 `name`。
- **DecisionAnswer（决策答案）**：与问题同序的 `predicate`（`probability`）/ `choice`（`choice` + `confidence` +
  `probabilities`）/ `score`（`score` + `confidence` + `probabilities`）/ `refusal`（模型拒答）四态。
- **ProtocolType 新增成员**：`openai-decision`（`enum.ProtocolOpenAIDecision`），用于审计的接口层/上游协议标识。

## 4. 设计

### 4.1 Endpoint 能力开关（配置链路）

照抄 `support_openai_response` 的既有模式，新增第 4 个开关，**默认 `false`**
（Decision 是新接口，存量端点不应被隐式认定为支持）。

| 层 | 文件 | 改动 |
|---|---|---|
| DB | `internal/infrastructure/database/model/endpoint.go` | `SupportOpenAIDecision bool`，`gorm:"column:support_openai_decision;not null;default:false;comment:支持/decisions"` |
| 常量 | `internal/common/constant/string.go` | `FieldEndpointSupportOpenAIDecision = "support_openai_decision"` |
| 常量 | `internal/common/constant/sql.go` | `FieldSupportOpenAIDecision`；加入 `EndpointRepoFieldsFull` |
| 领域 | `internal/domain/llmproxy/aggregate/endpoint.go` | 字段 + `CreateEndpoint` 参数 + `Update` 可变参数 + `SupportOpenAIDecision()` getter |
| 领域 | `internal/domain/llmproxy/repository.go` | `EndpointProjection` 新增字段 |
| 仓储 | `internal/infrastructure/repository/endpoint_repository.go` | `toEndpointAggregate` / `toEndpointModel` / `Update` 的 updates map / `toEndpointProjection` |
| 应用层 | `internal/application/endpoint/port/handler.go` | `CreateEndpointCommand` / `UpdateEndpointCommand` 新增字段 |
| 应用层 | `internal/application/endpoint/command/{create,update}_endpoint.go` | 透传字段 |
| 应用层 | `internal/application/upstream/port/handler.go` + `query/list_upstream.go` | `UpstreamEndpointView` 新增字段并赋值 |
| DTO | `internal/dto/endpoint.go` | `CreateEndpointReqBody` / `UpdateEndpointReqBody` 新增 `SupportOpenAIDecision *bool` |
| DTO | `internal/dto/upstream.go` | `UpstreamEndpointItem` 新增 `SupportOpenAIDecision bool` |
| Handler | `internal/handler/endpoint.go` / `upstream.go` | 透传字段 |

迁移：`cmd/server database migrate`（`AutoMigrate` 自动加列，无需手工 SQL）。

### 4.2 路由与兼容路线（native-only）

- `internal/common/enum/llmproxy_compat.go`：`ProxyAPI` 追加 `ProxyAPIOpenAIDecision`（iota 末尾）。
- `internal/common/enum/provider.go`：追加 `ProtocolOpenAIDecision ProtocolType = "openai-decision"`。
- `internal/application/llmproxy/usecase/compat_route.go`：

```go
case enum.ProxyAPIOpenAIDecision:
    if ep.SupportOpenAIDecision() {
        return enum.CompatRouteNative
    }
```

其余情形落回 `CompatRouteUnsupported` → 用例层返回 `SendOpenAIModelNotFoundError`，
与「端点不支持该接口」的既有语义一致（不暴露端点是否存在的差异）。

### 4.3 DTO（`internal/dto/openai/decision.go`）

**请求**：完整建模（huma 需要 typed body 生成 OpenAPI 并做基础校验）。

```go
type OpenAICreateDecisionRequest struct {
    Body *OpenAICreateDecisionReq `json:"body" doc:"请求体"`
}

type OpenAICreateDecisionReq struct {
    Model            string              `json:"model" required:"true" doc:"模型别名"`
    Input            DecisionInput       `json:"input" required:"true" doc:"待评估的文本或 user 消息数组"`
    Questions        []*DecisionQuestion `json:"questions" required:"true" minItems:"1" doc:"问题列表"`
    SafetyIdentifier *string             `json:"safety_identifier,omitempty" doc:"调用方提供的终端用户标识"`
}

// DecisionInput string | []DecisionInputMessage
type DecisionInput struct {
    Text     *string                 `json:"-"`
    Messages []*DecisionInputMessage `json:"-"`
}

type DecisionInputMessage struct {
    Role    string                      `json:"role" doc:"固定 user"`
    Type    *string                     `json:"type,omitempty" doc:"固定 message"`
    Content ResponseInputMessageContent `json:"content" doc:"文本或文本+图片 parts"`
}

type DecisionQuestion struct {
    Type         string                `json:"type" doc:"predicate/choice/score"`
    Name         *string               `json:"name,omitempty" doc:"问题名"`
    Instructions string                `json:"instructions" required:"true" doc:"问题指令"`
    Choices      []*DecisionChoice     `json:"choices,omitempty" doc:"type=choice 时 2~255 个选项"`
    Levels       []*DecisionScoreLevel `json:"levels,omitempty" doc:"type=score 时有序等级"`
}

type DecisionChoice struct {
    Value       DecisionChoiceValue `json:"value" doc:"选项值(string|bool)"`
    Description *string             `json:"description,omitempty" doc:"选项描述"`
}

// DecisionChoiceValue string | bool（同文本的 string 与 bool 是不同选项）
type DecisionChoiceValue struct {
    StringValue  *string `json:"-"`
    BooleanValue *bool   `json:"-"`
}

type DecisionScoreLevel struct {
    Label       string  `json:"label" doc:"等级标签"`
    Description *string `json:"description,omitempty" doc:"等级描述"`
}
```

联合类型实现一律复用 `response.go` / `response_input.go` 的既有模式：
变体字段 + 自定义 `MarshalJSON` / `UnmarshalJSON` / `Schema(huma.Registry)`。

- `DecisionInput`：`UnmarshalJSON` 先试 string，失败则按 `[]DecisionInputMessage`；`MarshalJSON` 按分支；
  `Schema` 用 `OneOf[string, array<DecisionInputMessage>]`。
- `DecisionChoiceValue`：`UnmarshalJSON` 先试 `bool`，失败再按 `string`（必须是 bool 优先，
  否则 `true` 不会被 JSON 解析成字符串）；`MarshalJSON` 按分支；`Schema` 用 `OneOf[boolean, string]`。
- **复用**：`input` 的内容块直接复用既有的 `ResponseInputMessageContent` / `ResponseInputContent`
  （Decision 的 `input_text` / `input_image` 与 Response API 输入内容块形状一致），不重复造联合类型。

**响应**：不做全量建模，只解析审计与存储所需子集：

```go
type OpenAIDecisionRsp struct {
    Model   string                    `json:"model" doc:"上游返回的模型名"`
    Answers sonic.NoCopyRawMessage    `json:"answers" doc:"答案数组(原样保留)"`
    Usage   *OpenAIDecisionUsage      `json:"usage,omitempty" doc:"使用量"`
}

type OpenAIDecisionUsage struct {
    InputTokens         int                               `json:"input_tokens"`
    OutputTokens        int                               `json:"output_tokens"`
    TotalTokens         int                               `json:"total_tokens"`
    InputTokensDetails  *DecisionInputTokensDetails       `json:"input_tokens_details,omitempty"`
    OutputTokensDetails *DecisionOutputTokensDetails      `json:"output_tokens_details,omitempty"`
}

type DecisionInputTokensDetails struct {
    CachedTokens     int `json:"cached_tokens"`
    CacheWriteTokens int `json:"cache_write_tokens"`
}

type DecisionOutputTokensDetails struct {
    ReasoningTokens int `json:"reasoning_tokens"`
}
```

响应体本身走 raw 透传（`ForwardCreateDecision` 返回 `[]byte`），用例把 `model` 改写回别名后
直接下发；`answers` 以 `sonic.NoCopyRawMessage` 解析出来仅用于会话存储的文本化。

触发词命中（deny）时的自造响应体：

```json
{
  "model": "<别名>",
  "answers": [{"type": "refusal", "name": "<question.name 或 null>"}, "..."],
  "usage": {"input_tokens": 0, "output_tokens": 0, "total_tokens": 0,
            "input_tokens_details": {"cached_tokens": 0, "cache_write_tokens": 0},
            "output_tokens_details": {"reasoning_tokens": 0}}
}
```

即：每个 question 回一条协议原生的 `refusal`，HTTP 200，不调用上游。
`usage` 必须存在且为零值（客户端普遍直接取用该对象）。

### 4.4 传输层

- `internal/common/constant/upstream.go`：`UpstreamPathOpenAIDecisions = "/decisions"`。
- `internal/application/llmproxy/usecase/port.go`（`OpenAIProxyPort`）：

```go
ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)
```

- `internal/infrastructure/transport/openai.go`：实现体复用 `doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIDecisions)`，
  读取并返回原始 body（与 `ForwardCreateResponse` 结构一致）。
  熔断/重试/drain/bulkhead/响应头透传全部免费继承。

### 4.5 用例编排

`port.OpenAIUseCase` 新增：

```go
CreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (Result, error)
```

`usecase/openai.go` 实现（编号对应 `CreateChatCompletion` 的既有结构）：

```
CreateDecision(ctx, req):
  1. compatRoute = SelectCompatRoute(ProxyAPIOpenAIDecision, ep)   // resolver matcher 内
     resolve 失败 → SendOpenAIModelNotFoundError(req.Body.Model)
  2. matched = checkDecisionContent(req)
       命中 deny  → IncrementHits
                    → 提交审计（APIProtocol/UpstreamProtocol=openai-decision，ErrorMessage=触发词备注）
                    → 返回 JSONResult(200, BuildDecisionRefusalBody(model, questions))
       命中 omit  → ctx = WithValue(CtxKeySkipStore, true)
       （capture 不处理，见 2.2）
  3. upstream = toTransportEndpoint(m, ep, false)
     body     = MarshalOpenAIDecisionBodyForModel(req.Body, upstream.Model)
  4. respBody, err = openAIProxy.ForwardCreateDecision(ctx, upstream, body)
       失败 → auditFailure(...) + ProxyErrorFromUpstream(err, ProtocolKindOpenAI, openAIInternalErrorBody)
  5. replaced = ReplaceModelInBody(respBody, req.Body.Model)
  6. 解析 OpenAIDecisionRsp（失败仅告警，不影响响应）
       成功 → storeDecisionSession(...)（受 CtxKeySkipStore 约束）
              out.usage = decisionTokenUsage{rsp}
              out.successStatus = true
  7. recordModelCall(ctx, ...)
  8. 返回 JSONResult(200, buildPassthroughHeaders(ctx)+Content-Type: application/json, replaced)
```

触发词文本提取 `extractDecisionText(req)` 覆盖全部用户可控文本：
`input`（字符串，或消息 content 的文本/parts）+ `questions[].instructions` +
`questions[].choices[].description` + `questions[].levels[].label/description`。

会话存储 `storeDecisionSession(ctx, req, rsp, modelID)`：

- 前置：`CtxKeySkipStore` 为真则跳过。
- user 消息：把 `input` 合并为**一条** user 消息（Decision 的 `input` 是单一逻辑载荷）
  - 纯文本 → `UnifiedContent{Text: ...}`
  - 含图片 → `UnifiedContent{Parts: [...]}`，文本块 `{type:text}`、图片块
    `{type:image_url, image_url, image_detail}`
- assistant 消息：`UnifiedContent{Text: string(rsp.Answers)}`（answers 的 JSON 原文，信息无损且不发明格式）
- `InputTokens` / `OutputTokens` 取 `rsp.Usage`；`Tools` 为空；`Metadata` 为空。
- 经 `SubmitMessageStoreTask` 投递（`util.CopyContextValues`）。

### 4.6 审计与计费

- `internal/dto/asynctask.go`：`SetTokensFromDecisionUsage(rsp *OpenAIDecisionRsp)`——
  `input_tokens` 含 `cached_tokens`，故 `InputTokens = input_tokens − cached_tokens`（净输入），
  `CacheReadInputTokens = cached_tokens`，`CacheCreationInputTokens = cache_write_tokens`，
  `OutputTokens = output_tokens`；与 `SetTokensFromResponseUsage` 同口径。
- `internal/application/llmproxy/usecase/recorder.go`：新增 `decisionTokenUsage` 适配器
  （`apply` → `SetTokensFromDecisionUsage`；`reportable` → `InputOutputTokens()`）。
- `internal/infrastructure/database/model/model_call_audit.go`：列注释补 `openai-decision`（仅注释文本）。
- 计费：`PriceModelCall` 无需改动（token 四维已归一化，自行按模型定价规则计算）。
- 前端 `web/src/app/(dashboard)/audit/model/page.tsx`：`formatProtocol` 增加
  `"openai-decision": "Decision"`。

### 4.7 Handler / Router

- `internal/handler/openai.go`：`HandleCreateDecision` → `AdaptProxyResult(ctx, result, err, openAIInternalFallbackBody)`。
  **不挂 `WithStreamLifecycle`**：Decision 无流式形态，挂载只会带来永不触发的死代码
  （该回调由 adapter 在真实 SSE 写入时 bracket，unary JSONResult 路径不会触发）。
- `internal/router/openai.go`：注册

```
OperationID: createDecision
Method:      POST
Path:        /decisions
Tags:        constant.TagOpenAI
MaxBodyBytes: constant.MaxLLMProxyBodyBytes
Middlewares: TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", ...)
             TokenBucketTokenRateLimiterMiddleware(cache, "callProxyLLMToken", ...)
Security:    apiKeyAuth
```

与 `/chat/completions`、`/responses` 完全一致（同限流桶、同 body 上限）。

### 4.8 Web 管理端

| 文件 | 改动 |
|---|---|
| `web/src/lib/types.ts` | `CreateEndpointReqBody` / `UpdateEndpointReqBody` / `UpstreamEndpointItem`（及端点列表项类型）新增 `supportOpenAIDecision` |
| `web/src/app/(dashboard)/upstream/shared.tsx` | 表单类型 + 默认值 `false` |
| `web/src/app/(dashboard)/upstream/endpoint-dialog.tsx` | 第 4 个 checkbox（`endpoints.openai_decision_label`） |
| `web/src/app/(dashboard)/upstream/grouped-view.tsx` | 组头协议徽标：`ep.supportOpenAIDecision && <ProtocolBadge protocol="openai-decision" label={...} />` |
| `web/src/app/(dashboard)/upstream/page.tsx` | 创建/更新请求体映射（各 2 处） |
| `web/src/locales/{zh,en,ja}.json` | `endpoints.openai_decision_label` |

`ProviderIcon` 无需改动：`openai-decision` 命中 `openai` 前缀 → 复用 OpenAI 图标。

## 5. 数据流

```
客户端 POST /api/openai/v1/decisions  {model, input, questions, safety_identifier}
  → APIKey 鉴权中间件（注入 userID / apiKeyID / apiKeyName）
  → 双 TokenBucket 限流
  → handler.HandleCreateDecision
  → usecase.CreateDecision
      ├ resolver.Resolve(alias) → (endpoint, model)   [matcher: SelectCompatRoute == Native]
      ├ 触发词检查
      │    └ deny → 200 refusal answers（不上游）
      ├ MarshalOpenAIDecisionBodyForModel  (model → upstream_model)
      ├ transport.ForwardCreateDecision → 上游 {openaiBaseURL}/decisions
      ├ ReplaceModelInBody                 (model → 别名)
      ├ 解析 usage/answers → 会话存储 + 审计任务/计费 + token 限流上报
      └ JSONResult(200, 上游原始 body)
```

## 6. 错误处理

| 场景 | 行为 |
|---|---|
| 端点未声明 `support_openai_decision` | `SendOpenAIModelNotFoundError(model)`（与其它接口同语义，不泄露端点信息） |
| 模型别名不存在 / 未启用 | 同上 |
| 上游非 200 | `ProxyErrorFromUpstream`：原样透传状态码、可透传响应头与错误体 |
| 上游不可达 / 读体失败 | 502 + OpenAI 错误包络（`openAIInternalErrorBody`） |
| 熔断打开 / 信号量满载 | 503 + `Retry-After`（既有守卫逻辑，免费继承） |
| 响应体解析失败（仅影响审计/存储） | 记 debug/warn 日志，**响应仍原样下发** |
| 触发词 deny | 200 + 全 `refusal` 的 Decision 响应体；审计记触发词备注 |

## 7. 测试策略

### 7.1 单元测试

`test/unit/openai_decision_dto/decision_dto_test.go`

- `DecisionInput`：`"text"` → Text；`[{...}]` → Messages；round-trip 序列化一致。
- `DecisionChoiceValue`：`"a"` / `true` / `false` / 空值 的解析与序列化；
  **`true` 不得被解析为字符串**。
- `DecisionQuestion` 三型（predicate / choice / score）反序列化。
- `OpenAIDecisionRsp`：usage 明细（cached / cache_write）解析。

`test/unit/llmproxy_usecase/decision_forward_test.go`（复用 `mockOpenAIProxy` / `mockResolver` / `mockTaskSubmitter` 骨架）

| 用例 | 断言 |
|---|---|
| native 成功 | 返回 `*port.JSONResult`，`Body` 中 `model` 为别名；上游收到 body 的 `model` 为 upstream_model |
| usage 归一化 | 提交的审计任务 `InputTokens = input − cached`、`CacheReadInputTokens = cached`、`CacheCreationInputTokens = cache_write` |
| 会话存储 | 提交的 `MessageStoreTask` 含 1 条 user + 1 条 assistant，assistant 文本为 answers JSON |
| omit 命中 | 不提交 `MessageStoreTask` |
| deny 命中 | 不调用上游；body 为每问一条 `refusal`，usage 全零；审计含触发词备注 |
| 端点不支持 decision | 返回 model-not-found 错误，不调用上游 |
| 上游错误 | 返回 `*port.ProxyError` 且状态码/响应体透传 |
| 响应体非法 JSON | 仍返回 200 与原始 body（审计降级） |

`test/unit/llmproxy_usecase/openai_forward_test.go` 内 `SelectCompatRoute` 用例补 decision 分支。

因 `OpenAIProxyPort` 与 `aggregate.CreateEndpoint` 签名变化，需同步更新既有测试 mock 与调用点
（`mockOpenAIProxy`、`buildCompatEndpoint`、`endpoint_resolver_test.go`、`domain_llmproxy/endpoint_test.go`、
`endpoint_command`、`endpoint_repository_scope_test.go`、`anthropic_forward_test.go` 等）。

### 7.2 E2E

`test/e2e/openai_decision/openai_decision_test.go` + `fixtures/requests/{predicate,choice,score}.json`

- 沿用 `mustE2EEnv`（`BASE_URL` / `API_KEY`，缺省 skip，打生产）。
- 断言：HTTP 200、`model` 等于请求别名、`answers` 数量与 `questions` 一致、
  每项 `type` 与提问类型对应、`usage.total_tokens > 0`。
- 覆盖：字符串 input 的 predicate、choice（含 bool 选项）、score 三型；
  图片 input 用例按上游能力可选。

### 7.3 静态检查

`make lint`（conv + static）与 `make test` 全绿；前端 `cd web && npm run lint`。

## 8. 风险与协调

1. **与在途分支的冲突**：`feature/endpoint-scheduling-pricing-2026-10-09`（未合并）也修改了
   `internal/application/llmproxy/usecase/recorder.go`、`internal/dto/asynctask.go`、
   `web/src/app/(dashboard)/upstream/shared.tsx`、`web/src/lib/types.ts` 与三个 locale 文件。
   本分支从 `master` 起，若该分支先合并，需按上述文件手工解冲突（改动区域不同，风险可控）。
2. **DB 迁移顺序**：`AutoMigrate` 加列必须先于新版本滚动部署，否则新 pod 读写不存在的列会失败。
   部署流程为推 `master` 触发 `docker-publish.yml`，需确认 migrate 已先执行。
3. **Decision API 上游可用性未知**：当前已知的 OpenAI 兼容上游不一定实现 `/decisions`。
   端点开关默认 `false` 保证了不误伤；E2E 需要用户提供支持该接口的上游模型别名。
4. **兼容性**：不改动既有三个接口的任何行为；`ProxyAPI` / `CompatRoute` 枚举为追加，未重排既有值。

## 9. 变更文件清单

**后端（Go）**

- `internal/common/constant/{sql.go,string.go,upstream.go}`
- `internal/common/enum/{llmproxy_compat.go,provider.go}`
- `internal/domain/llmproxy/aggregate/endpoint.go`
- `internal/domain/llmproxy/repository.go`
- `internal/infrastructure/database/model/{endpoint.go,model_call_audit.go}`
- `internal/infrastructure/repository/endpoint_repository.go`
- `internal/infrastructure/transport/openai.go`
- `internal/dto/{asynctask.go,endpoint.go,upstream.go,aliases.go}`
- `internal/dto/openai/decision.go`（新增）
- `internal/application/llmproxy/port/handler.go`
- `internal/application/llmproxy/usecase/{compat_route.go,openai.go,openai_decision.go,recorder.go,trigger_check.go}`
- `internal/application/llmproxy/util/{model.go,trigger_content_filter.go}`（`MarshalOpenAIDecisionBodyForModel` / `BuildDecisionRefusalBody`）
- `internal/application/endpoint/{port/handler.go,command/create_endpoint.go,command/update_endpoint.go}`
- `internal/application/upstream/{port/handler.go,query/list_upstream.go}`
- `internal/handler/{openai.go,endpoint.go,upstream.go}`
- `internal/router/openai.go`

**前端（web）**

- `web/src/lib/types.ts`
- `web/src/app/(dashboard)/upstream/{shared.tsx,endpoint-dialog.tsx,grouped-view.tsx,page.tsx}`
- `web/src/app/(dashboard)/audit/model/page.tsx`
- `web/src/locales/{zh,en,ja}.json`

**测试**

- `test/unit/openai_decision_dto/decision_dto_test.go`（新增）
- `test/unit/llmproxy_usecase/decision_forward_test.go`（新增）
- `test/unit/llmproxy_usecase/openai_forward_test.go`（补 compat-route 用例 + mock 更新）
- `test/unit/{domain_llmproxy,endpoint_resolver,endpoint_command,llmproxy_repo_scope}/...`（签名同步）
- `test/e2e/openai_decision/`（新增）

**文档**

- `CONTEXT.md`（ProtocolType 与 Endpoint 能力描述补 Decision）
