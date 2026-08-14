# Blocked Words deny 返回协议内容拦截消息设计

- 日期：2026-08-15
- 状态：已与需求方确认（决策记录见 §2）
- 分支规划：`feature/blocked-deny-content-filter-2026-08-15`

## 1. 背景与目标

当前敏感词（Blocked Words）命中 `deny` action 时，LLM 代理请求被短路为 **HTTP 403** + 业务错误体（`SendOpenAIContentBlockedError` / `SendAnthropicContentBlockedError`，见 `internal/application/llmproxy/util/sse.go`）。

新需求：deny 命中时不再返回 403，而是返回 **OpenAI / Anthropic 协议对应的内容拦截消息**（HTTP 200），使 Claude Code / Codex 等 agentic 客户端能按协议原生语义处理拦截（而非把 403 当网关错误）。

## 2. 决策记录（需求方已拍板）

| # | 决策点 | 结论 |
|---|--------|------|
| D1 | 响应形态 | HTTP 200 + 协议原生内容拦截消息（对齐 docs 协议文档中的 content_filter / refusal 语义），替代 403 |
| D2 | 拦截文案 | 统一固定英文文案 `Content blocked by policy`；OpenAI `message.content`、Anthropic text block 与 `stop_details.explanation` 共用 |
| D3 | OpenAI Response output 类型 | 使用协议原生 `refusal` content part（`{"type":"refusal","refusal":"..."}`） |
| D4 | stream 覆盖 | 三协议 × stream/非 stream 共六种构造全部覆盖；stream=true 时按协议 SSE 事件序列吐出后结束 |
| D5 | 审计 / 计数 | 照旧：命中计数递增 + `ModelCallAuditTask`（remark 标注命中词）不变 |
| D6 | 上游与存储 | 不触达上游；不落库 session/message（与现状一致） |
| D7 | 兼容 | `BizErrorCodeContentBlocked`(10010) 与前端 `api-errors.ts` 保留（向后兼容，不再由 LLM 代理触发）；`SendOpenAIContentBlockedError` 等 403 构造保留（其他调用方/回归参考） |

## 3. 协议依据（docs 协议文档）

- **OpenAI Chat**（`docs/openai/create_chat_completions.md`）：`finish_reason` 枚举含 `"content_filter"` —— "content was omitted due to a flag from our content filters"。
- **OpenAI Response**（`docs/openai/create_response.md`）：`incomplete_details.reason` 含 `"content_filter"`（"The reason why the response is incomplete"）；output message content 支持 `{"type":"refusal","refusal":"..."}`（"A refusal from the model"）。
- **Anthropic**（`docs/anthropic/create_message.md`）：`stop_reason` 含 `"refusal"`（"when streaming classifiers intervene to handle potential policy violations"）；`stop_details` 为 `RefusalStopDetails{type, category, explanation}`。

## 4. 后端详细设计

### 4.1 响应形态（六种构造）

实现放 `internal/application/llmproxy/util`（与 `SendAnthropicContentBlockedError` 同目录），返回 `*port.JSONResult` / `*port.StreamResult`（复用现有 adapter 通路）。

**① OpenAI Chat JSON（200）**

```json
{
  "id": "chatcmpl-<uuid>",
  "object": "chat.completion",
  "created": <unix>,
  "model": "<请求模型>",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "Content blocked by policy"},
    "finish_reason": "content_filter"
  }],
  "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
}
```

**② OpenAI Chat SSE**：`role` chunk（`delta.role=assistant`）→ `content` chunk（`delta.content=文案`）→ `finish_reason=content_filter` chunk（空 delta）→ `data: [DONE]`。

**③ OpenAI Response JSON（200）**

```json
{
  "id": "resp_<uuid>",
  "object": "response",
  "created_at": <unix>,
  "status": "completed",
  "model": "<请求模型>",
  "output": [{
    "type": "message",
    "id": "msg_<resp-id>",
    "status": "completed",
    "role": "assistant",
    "content": [{"type": "refusal", "refusal": "Content blocked by policy", "annotations": []}]
  }],
  "incomplete_details": {"reason": "content_filter"},
  "usage": {"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}
}
```

**④ OpenAI Response SSE**（对齐 `converter/response_stream.go` 事件形态）：`response.created`（response 对象 output 为空）→ `response.output_item.added`（message item）→ `response.content_part.added`（refusal part 携带全文）→ `response.content_part.done` → `response.output_item.done` → `response.completed`（payload 带 `incomplete_details`）。refusal 内容一次性在 `content_part.added` 给出（不发送 delta 事件，协议允许）。

**⑤ Anthropic JSON（200）**

```json
{
  "id": "msg_<uuid>",
  "type": "message",
  "role": "assistant",
  "content": [{"type": "text", "text": "Content blocked by policy"}],
  "model": "<请求模型>",
  "stop_reason": "refusal",
  "stop_sequence": null,
  "stop_details": {"type": "refusal", "category": null, "explanation": "Content blocked by policy"},
  "usage": {"input_tokens": 0, "output_tokens": 0}
}
```

**⑥ Anthropic SSE**：`message_start`（message 对象含 usage/role/model）→ `content_block_start`（text block）→ `content_block_delta`（text_delta）→ `content_block_stop` → `message_delta`（`stop_reason=refusal` + `stop_details`）→ `message_stop`。

SSE 统一用「预生成事件列表」的 `port.Stream` 实现（`Read` 依序 `sink.WriteEvent` 后返回 nil，`Close` 返回 nil），不建上游连接。

### 4.2 常量与枚举

- `internal/common/constant/string.go`：新增 `BlockedContentFilterMessage = "Content blocked by policy"`。
- `internal/common/enum/anthropic_stop_reason.go`：新增 `AnthropicStopReasonRefusal = "refusal"`。
- `internal/common/enum/anthropic.go`：新增 `AnthropicStopDetailsTypeRefusal = "refusal"`（stop_details.type）。
- `internal/common/enum/openai_response.go`：新增 `ResponseIncompleteReasonContentFilter = "content_filter"`（incomplete_details.reason）。
- `internal/common/constant/upstream.go`：新增 `ResponseStreamFieldRefusal = "refusal"`、`ResponseStreamFieldIndex = "index"`（SSE payload 键）。
- 复用现有 ID 模板：`OpenAIChunkIDTemplate("chatcmpl-%s")`、`ResponseIDTemplate("resp_%s")`、`ResponseItemIDTemplate("msg_%s")`、`AnthropicMessageIDTemplate("msg_%s")`，以及 `constant.AnthropicMessageType`、`enum.AnthropicDeltaTypeTextDelta`。

### 4.3 UseCase 分支改动

`openai.go` `CreateChatCompletion` / `CreateResponse` 与 `anthropic.go` `CreateMessage` 的 deny 分支：审计任务照旧提交，返回由 `SendXxxContentBlockedError()` 改为对应协议的「内容拦截消息结果」（stream 由请求 `stream` 字段决定构造 JSON 或 SSE 形态）。`exposedModel`（`req.Body.Model`）作为响应 `model` 字段。

### 4.4 边界

- deny 命中且请求 `stream=true` 但返回的是 200 SSE → 客户端按正常流解析（事件序列对齐现有 converter 产物）。
- 无 token 消耗：`usage` 全 0，不进吞吐统计（与现有 deny 行为一致）。

## 5. 测试计划

- **单测**（`test/unit/llmproxy_usecase/`）：
  - 更新 `openai_response_blocked_test.go` 断言：deny 命中返回 200 结果（`*port.JSONResult` 或 `*port.StreamResult`），而非 `ProxyError`；upstream 不触达；命中计数照常。
  - 新增三协议 × stream/非 stream 的响应形态断言：JSON 字段（`finish_reason=content_filter` / `incomplete_details.reason` / `stop_reason=refusal` / `stop_details`）与 SSE 事件序列（事件类型 + payload 关键字段）。
  - Anthropic deny 单测当前缺失 → 补充（`fakeBlockedChecker` 复用）。
- **e2e**（`test/e2e/blocked/`）：deny 回归用例断言由 403 改为 200 + content_filter/refusal 形态；stream 用例断言 SSE 事件序列。

## 6. 非目标（YAGNI）

- 不做 per-word 可配置拦截文案（D2 已确认统一固定文案）。
- 不改 `ModelCallAudit` / 限流 / 监控口径。
- 不删除 403 构造函数与 `BizErrorCodeContentBlocked`（保留兼容）。
- 不涉及 trigger-word-capture 的全量重命名（另行任务）。

## 7. 风险与边界

| 风险 | 缓解 |
|------|------|
| agentic 客户端对 200 拦截响应的解析兼容性 | 事件/字段对齐 docs 协议与现有 converter 产物；e2e 用真实协议形态断言 |
| 既有依赖 403 的调用方（前端 api-errors.ts） | 保留错误码与前端处理（向后兼容）；LLM 代理不再触发 403 |
| stream 请求误判为非流 | 由请求 `stream` 字段决定构造形态（六种构造全覆盖） |
