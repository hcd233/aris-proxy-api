---
name: huma-dto-conventions
description: Use when creating, editing, or reviewing HTTP DTOs in internal/dto/, adding new huma routes, modifying request/response payloads, or debugging "field is always zero / nil body / 422 validation" issues in aris-proxy-api. Mention this skill whenever the work touches `*Req` / `*Rsp` structs, huma path/query/body bindings, or OpenAPI schema fields — even if the user only says "add an API" or "tweak the request payload".
---

# Huma DTO 规范 (huma-dto-conventions)

aris-proxy-api 用 [huma v2](https://huma.rocks) 把 Go 结构体直接映射成 OpenAPI schema 和 HTTP 处理器。huma 的字段绑定语义和 `encoding/json` 的"反射就反序列化"很不一样：**字段必须用 huma 认识的标签或包装结构告诉它来源于哪里（path/query/header/body/cookie）**，否则 huma 会**静默忽略**这部分输入，让字段保持零值。

本 skill 的目标是让任何人在写 / 改 / review `internal/dto/**` 时，一次就遵守 huma 规范，避免 zero-value 数据流进数据库或 redis 触发"返回错位记录""沉默丢字段"这类难排查的 bug。

## 真实事故（必看）

`POST /api/v1/session/share` 的请求体是 `{"sessionId": 13687}`，但服务端把 0 写进了 redis；分享 URL 访问时返回的是 id=2 的会话——和当事用户毫无关系。

根因藏在三层：

1. **DTO 层**：`CreateShareReq` 把 `SessionID uint` 直接平铺在顶层（仅有 `json:"sessionId"`），既没有 `path`/`query` 标签，也没有按 huma 约定包在 `Body *XxxReqBody` 里。huma 看不出这个字段来自哪里，于是直接跳过 body 反序列化，`req.SessionID` 永远是零值 `0`。
2. **Cache 层**：`shareCache.CreateShare` 没有拒绝 `sessionID == 0`，把 `0` 写进 redis。
3. **数据访问层**：GORM 的 `First(&Session{ID: 0})` 经典坑——零值字段被当作"无 where 条件"，于是返回了主键最小的那条记录。

修复主战场是第 1 层。第 2、3 层是防御性补丁，但只要第 1 层的规范没破，后两层永远不会被触发。**这条规范是最便宜的防御，请把它写对一次就别再退化。**

## 核心规则

### 规则 1：根据字段来源选标签或包装

| 字段来源 | 怎么写 | 例 |
|---|---|---|
| URL Path 参数 | `path:"name"` 标签 | `ID uint \`path:"id" required:"true" minimum:"1"\`` |
| URL Query 参数 | `query:"name"` 标签 | `Page int \`query:"page" minimum:"1"\`` |
| HTTP Header | `header:"name"` 标签 | `Auth string \`header:"Authorization"\`` |
| Cookie | `cookie:"name"` 标签 | `SID string \`cookie:"sid"\`` |
| **JSON Body（POST / PATCH / PUT）** | **必须**在外层 Req 上声明 `Body *XxxReqBody` 字段，body 字段写在 `XxxReqBody` 里 | 见下文模板 |

**最容易踩的坑**：只写 `json:"sessionId"` 而不包 Body，huma 会当成无来源的字段忽略掉 body。这正是 `CreateShareReq` 出事的根因。**`json:` 标签本身不是 huma 的"来源"标签**，它只描述序列化时的键名，不告诉 huma 该字段从哪来。

### 规则 2：JSON Body 的标准模板

所有写入 JSON body 的接口（`POST` / `PATCH` / `PUT`）一律按下面两段式写，外层 Req 只承载来源标签的字段（path / Body 包装）：

```go
// CreateModelReq 创建 Model 请求
type CreateModelReq struct {
    Body *CreateModelReqBody `json:"body" doc:"Request body"`
}

// CreateModelReqBody 创建 Model 请求体
type CreateModelReqBody struct {
    Alias      string `json:"alias" required:"true" minLength:"1" doc:"模型别名"`
    ModelName  string `json:"modelName" required:"true" minLength:"1" doc:"上游实际模型名"`
    EndpointID uint   `json:"endpointID" required:"true" minimum:"1" doc:"关联 Endpoint ID"`
}
```

带 path 段的更新接口把 path 字段直接放在外层，body 仍然走包装：

```go
type UpdateModelReq struct {
    ID   uint                `path:"id" required:"true" minimum:"1" doc:"Model ID"`
    Body *UpdateModelReqBody `json:"body" doc:"Request body"`
}

type UpdateModelReqBody struct {
    Alias      *string `json:"alias,omitempty" doc:"模型别名"`
    ModelName  *string `json:"modelName,omitempty" doc:"上游实际模型名"`
    EndpointID *uint   `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
}
```

handler 通过 `req.Body.Field` 取值。需要时加一道 nil 兜底：

```go
if req.Body == nil {
    rsp.Error = ierr.ErrValidation.BizError()
    return apiutil.WrapHTTPResponse(rsp, nil)
}
sessionID := req.Body.SessionID
```

实际上字段加了 `required:"true"` 的话，huma 会在请求层就返回 422，handler 里看到的 `req.Body` 一般非 nil；但显式 nil-guard 让代码不依赖外部框架的隐式行为，是廉价的防御。

### 规则 3：响应类型不需要包装，但要理解 huma 的"unwrap"行为

响应统一用 `*dto.HTTPResponse[BodyT]` 包一层并由 `apiutil.WrapHTTPResponse` 构造：

```go
type HTTPResponse[BodyT any] struct {
    Body BodyT `json:"data"`
}
```

但是 **huma 在响应方向会把 `Body` 字段直接当作 HTTP 响应体写出去**，外层 `data` 包装会被 unwrap 掉。也就是说线上看到的真实 wire 格式是：

```json
{"shareId": "...", "expiresAt": "..."}
```

而**不是**：

```json
{"data": {"shareId": "...", "expiresAt": "..."}}
```

这条对前端调用方和 E2E 测试解码很关键。曾经写 E2E 时按 `data` 包装去解码，结果所有字段为空，浪费一轮线上验证才发现。**写 E2E 测试或前端类型时，参考 `dto.CreateShareRsp` 这类 Body 类型直接 decode，不要再包一层 `data`。**

### 规则 4：DTO 包不依赖基础设施层

`internal/dto/**` 禁止 `import internal/infrastructure/database/model`（dbmodel）。理由：

- DTO 是 API 协议层；dbmodel 是持久化结构。把 dbmodel 暴露到 DTO 会导致协议升级被表结构绑死。
- 项目 lint（`script/lint-conventions.sh` §9.4）会扫这条，违反就 fail。

需要数据库字段时，把**具体字段**作为参数传入 DTO 函数，而不是塞 `*dbmodel.Xxx` 进来。

### 规则 5：禁止用 `encoding/json`、`json.RawMessage`、`any`、`interface{}`

DTO 字段必须有明确类型；所有 marshal/unmarshal 走 `github.com/bytedance/sonic`。需要"任意类型"的 metadata 用具体的 `map[string]string` 或专用结构体表达。`json.RawMessage`、`any`、`interface{}` 在 DTO 里禁用——这条由 `make lint` 扫描。

### 规则 6：时间字段用 `time.Time`，不要在 service 层格式化成字符串

DTO 字段类型直接用 `time.Time`，序列化让 sonic 处理。这样保留时区和精度，跨服务/E2E 反序列化也不会出错。Service 层提前把时间格式化成字符串会丢失类型信息。

### 规则 7：`required` 与 `omitempty` 的常见组合

- 必传：`required:"true"`，类型用值类型，**不要**带 `omitempty`。
- 可选：用指针类型 `*T`，配合 `json:"foo,omitempty"`；不写 `required:"true"`。
- 数值范围：用 `minimum:"1"` / `maximum:"100"`；字符串长度：`minLength:"1"` / `maxLength:"64"`。
- 枚举：`enum:"github,google"`，配合 Go 端的强类型 enum 保证一致。

## 自检清单（写完 / review 前过一遍）

```
□ POST/PATCH/PUT 的请求 DTO 是否声明了 Body *XxxReqBody？
□ 路径参数是否在外层用 path: 标签？query 同理？
□ JSON body 字段是否全部位于 XxxReqBody 中，而不是平铺到外层 Req？
□ 必填字段是否同时具备 required:"true" 和合适的取值约束（minimum / minLength / enum）？
□ 可选字段是否用了指针类型 + json:",omitempty"？
□ handler 是否通过 req.Body.Xxx 访问？是否需要补一层 if req.Body == nil 的兜底？
□ DTO 是否避免 import internal/infrastructure/database/model？
□ 是否禁用了 encoding/json / json.RawMessage / any / interface{}？
□ 时间字段是否用 time.Time 而非 string？
□ E2E / 前端解码方是否按 huma unwrap 后的扁平结构（无 data 外层）来解析？
```

## 反模式速查

### 反模式 1：把 body 字段平铺在外层（线上事故来源）

```go
// ❌ 错：huma 不知道 SessionID 来自哪里，body 反序列化被跳过
type CreateShareReq struct {
    SessionID uint `json:"sessionId" required:"true" minimum:"1"`
}
```

```go
// ✅ 对：用 Body 包装显式声明字段来自 JSON body
type CreateShareReq struct {
    Body *CreateShareReqBody `json:"body" doc:"Request body containing session ID"`
}
type CreateShareReqBody struct {
    SessionID uint `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}
```

### 反模式 2：在 GET 接口上用 Body 包装

GET 没有 body，用 query / path 即可：

```go
// ❌ 错：GET 不该有 Body
type GetSessionByUserReq struct {
    Body *GetSessionByUserReqBody `json:"body"`
}

// ✅ 对：用 query
type GetSessionByUserReq struct {
    SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}
```

### 反模式 3：用 `any` / `json.RawMessage` 偷懒

```go
// ❌ 错：失去类型，会被 lint 拦下
type WebhookReqBody struct {
    Payload any `json:"payload"`
}

// ✅ 对：建模成具体结构
type WebhookReqBody struct {
    Payload *WebhookPayload `json:"payload" required:"true"`
}
```

### 反模式 4：在 DTO 里 import dbmodel

```go
// ❌ 错：跨层依赖，lint 拦下
import "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

func (r *ListUsersRsp) FromModel(u *model.User) { ... }
```

应在 application/repository 层做转换，DTO 函数只接受具体字段或同层 view 类型。

### 反模式 5：写 E2E 时按 `{"data": {...}}` 解码响应

```go
// ❌ 错：huma 已经 unwrap 了 Body，外层没有 data
type rsp struct {
    Data struct{ ShareID string `json:"shareId"` } `json:"data"`
}

// ✅ 对：直接对应 dto.CreateShareRsp 的 Body 类型
type rsp struct {
    ShareID string `json:"shareId"`
}
```

## 已注册接口对照表（供 review 时快速找参考实现）

| 类型 | 接口 | DTO 文件 |
|---|---|---|
| POST + Body | `POST /api/v1/apikey/` | `internal/dto/apikey.go` `CreateAPIKeyReq` |
| POST + Body | `POST /api/v1/token/refresh` | `internal/dto/token.go` `RefreshTokenReq` |
| POST + Body | `POST /api/v1/oauth2/{provider}/callback` | `internal/dto/oauth2.go` `CallbackReq` |
| POST + Body | `POST /api/v1/model/` | `internal/dto/model.go` `CreateModelReq` |
| PATCH + query + Body | `PATCH /api/v1/model/?id=X` | `internal/dto/model.go` `UpdateModelReq` |
| POST + Body | `POST /api/v1/endpoint/` | `internal/dto/endpoint.go` `CreateEndpointReq` |
| PATCH + query + Body | `PATCH /api/v1/endpoint/?id=X` | `internal/dto/endpoint.go` `UpdateEndpointReq` |
| PATCH + Body | `PATCH /api/v1/user/` | `internal/dto/user.go` `UpdateUserReq` |
| POST + Body | `POST /api/v1/session/share` | `internal/dto/session_share.go` `CreateShareReq`（已修复，可作为模板） |
| POST + Body（LLM 代理） | `POST /api/openai/v1/chat/completions` | `internal/dto/openai/chat.go` `OpenAIChatCompletionRequest` |
| POST + Body（LLM 代理） | `POST /api/openai/v1/responses` | `internal/dto/openai/response.go` `OpenAICreateResponseRequest` |
| POST + Body（LLM 代理） | `POST /api/anthropic/v1/messages` | `internal/dto/anthropic/anthropic.go` `AnthropicCreateMessageRequest` |
| POST + Body（LLM 代理） | `POST /api/anthropic/v1/messages/count_tokens` | `internal/dto/anthropic/anthropic.go` `AnthropicCountTokensRequest` |
| GET + query | `GET /api/v1/session/list` 等 | `internal/dto/session.go` `ListSessionsByUserReq` |
| GET + query | `GET /api/v1/session/share/?id=X` | `internal/dto/session_share.go` `GetShareContentReq` |
| DELETE + query | `DELETE /api/v1/apikey/?id=X` 等 | `internal/dto/apikey.go` `DeleteAPIKeyReq` |

## 与其他 skill 的边界

- 出现现有 DTO 字段总是零值、JSON body 看似被忽略、422 校验异常、line response 字段缺失时，**先来本 skill 对照规则 1/2**；如果是线上正在发生的请求异常，再转入 `cls-log-bugfix` 拿 traceId 关联 CLS 日志。
- 写或改 E2E 测试涉及响应解码时，参考规则 3 的 unwrap 说明，避免再加一层错误的 `data` 包装。
- 修复完 DTO bug 必须按 `cls-log-bugfix` skill 的要求补回归用例，并在 `test/unit/<topic>/` 加一个反射断言（参考 `test/unit/session_share/session_share_test.go` 的 `TestCreateShareReq_DTOFollowsHumaBodyConvention`），确保未来任何人改回平铺式都会被立刻打回。
