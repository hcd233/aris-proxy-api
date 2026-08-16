# 触发词捕获（Trigger Word Capture）设计文档

- 日期：2026-08-15
- 状态：已与需求方逐项确认（决策记录见 §2）
- 分支规划：`feature/trigger-word-capture-2026-08-15`

## 1. 背景与目标

LLM 代理网关已有「敏感词（Blocked Words）」机制：管理员配置词表，请求全文命中 `deny` 词时短路返回 403，命中 `omit` 词时照常转发但不落库。

新需求：当用户发送包含特定触发词的消息时，**保存该消息之前的对话历史至 session，不请求上游，直接返回固定回复**。典型场景：在 Claude Code / Codex 等 agentic 客户端里用一个标记词（如 `/save-context`）把当前对话上下文沉淀到平台，供后续训练数据管线消费。

同时，「敏感词」的语义已从纯屏蔽扩展为「按 action 分流的多用途词表」，本次将全局语义升级为「触发词（Trigger Word）」，并全量重命名 `blocked → trigger`。

## 2. 决策记录（需求方已拍板）

| # | 决策点 | 结论 |
|---|--------|------|
| D1 | 配置方式 | 复用 Blocked Words 基础设施，新增第三种 action `capture`；语义整体升级为「触发词」 |
| D2 | 匹配语义 | AC 自动机全文子串匹配（与 deny/omit 共用同一 matcher） |
| D3 | 协议覆盖 | OpenAI Chat / OpenAI Response / Anthropic Message 三协议全覆盖 |
| D4 | 保存范围 | 仅保存触发消息**之前**的历史对话；不保存触发消息本身、不保存固定回复（不生成 assistant 消息） |
| D5 | 无历史边界 | 触发消息是第一条消息（无历史）时，不建 session，返回特殊固定回复 |
| D6 | 生效位置 | 仅当触发词出现在「最后一条 `role=user` 且无 `tool_call_id` 的消息」中才触发；只出现在历史消息 / system prompt 中不触发，照常转发 |
| D7 | 固定回复 | 统一硬编码两段英文文案（见 §4.5）；stream=true 时按协议 SSE 格式吐出，stream=false 返回普通 JSON |
| D8 | 优先级 | `deny` > `capture` > `omit`；同时命中 deny 与 capture 词时 deny 优先（403，不保存不回复） |
| D9 | 改名范围 | **全量重命名**：DB 表、API 路由、Go 包/符号、Redis key、前端路由与文案，`blocked → trigger` |
| D10 | 命中计数 / 审计 | 命中计数照常递增；capture 短路时照 deny 模式提交 ModelCallAuditTask（remark 标注触发词） |

## 3. 术语与重命名映射（D9）

领域术语统一为「触发词」。代码符号、存储、接口全量改名：

### 3.1 数据库与 Redis

| 现状 | 目标 | 迁移方式 |
|------|------|---------|
| 表 `blocked_words` | 表 `trigger_words` | 部署前手动 `ALTER TABLE blocked_words RENAME TO trigger_words;`（沿用 fix/blocked-word-recreate 的手动迁移先例） |
| 复合唯一索引 `idx_blocked_word_deleted` | `idx_trigger_word_deleted` | 可选：`ALTER INDEX ... RENAME`（纯美观，非必须） |
| Redis `blocked:hit:{id}` | `trigger:hit:{id}` | 直接切新 key；旧 key 中未同步的 pending 命中数会丢失（量级小，可接受） |
| Redis `blocked:change:channel` / `blocked:version` | `trigger:change:channel` / `trigger:version` | 直接切；切换后各 pod 首轮低频兜底重建自然收敛 |

表结构无 DDL 变更：`action` 列为 `varchar(16)`，新增值 `capture`（7 字符）直接写入，存量行为不变（空值兜底 `deny`）。

### 3.2 Go 符号（用 Serena 跨文件重命名）

| 现状 | 目标 |
|------|------|
| 包 `internal/domain/blocked`、`internal/application/blocked` | `internal/domain/trigger`、`internal/application/trigger` |
| 聚合 `aggregate.Blocked` / `CreateBlocked` | `aggregate.Trigger` / `CreateTrigger` |
| 服务 `BlockedService`、接口 `BlockedChecker`、`BlockedRepository` | `TriggerService`、`TriggerChecker`、`TriggerRepository` |
| 枚举 `BlockedAction`、`BlockedActionDeny`、`BlockedActionOmit` | `TriggerAction`、`TriggerActionDeny`、`TriggerActionOmit`，新增 `TriggerActionCapture = "capture"` |
| DTO `CreateBlockedReq` 等、handler `blocked.go`、router `blocked.go` | `CreateTriggerReq` 等、`trigger.go`、`trigger.go` |
| 常量 `BlockedTableName`、`BlockedWordSeparator`、`BlockedAuditRemarkTemplate`、`BlockedChangeChannel`、`BlockedVersionKey` 等 | `Trigger*` 前缀 |
| cron 模块 `blocked_hit_sync` | `trigger_hit_sync`（`cron_call_audits` 历史行保留旧名，新记录用新名） |
| context key `CtxKeySkipStore` | 不变 |

### 3.3 API 与前端

| 现状 | 目标 |
|------|------|
| 路由 `/api/v1/block`（list/create/update/delete） | `/api/v1/trigger`（breaking change，前后端同版本发布） |
| 前端页面 `/blocked`、菜单「敏感词」 | `/trigger`、菜单「触发词」 |
| 前端 `types.ts` `BlockedItem` 等、`api-client.ts` blocked 方法组 | `TriggerItem`、trigger 方法组 |
| i18n 全部「敏感词」文案 | 「触发词」 |
| action 展示：`deny`（拦截）/ `omit`（忽略）徽章 | 新增 `capture`（捕获）徽章，新增/行内切换/批量操作支持三值 |

### 3.4 文档

- `CONTEXT.md`：**Blocked** 词条改写为 **Trigger（触发词）**，含三种 action 语义；`BlockedService` → `TriggerService`；`SoftDeletePurge` 等引用处同步。

## 4. 后端详细设计

### 4.1 action 语义总表

| action | 命中后行为 | 上游 | 存储 | 响应 |
|--------|-----------|------|------|------|
| `deny`（默认） | 拦截 | 不请求 | 跳过 | 403 ContentBlocked |
| `capture`（新） | 捕获保存 | 不请求 | 保存触发消息之前的历史（无 assistant 回复） | 200 固定回复（JSON/SSE 跟随 stream） |
| `omit` | 放行 | 请求 | 跳过（`CtxKeySkipStore`） | 正常转发 |

优先级（同一请求命中多种 action 的词时）：`deny` > `capture` > `omit`。判定顺序即现有短路结构的扩展。

### 4.2 短路位置（三协议入口一致）

插入点与现有 deny 检查同级（`anthropic.go` `CreateMessage`、`openai.go` `CreateChatCompletion` / `CreateResponse`，均在 `resolver.Resolve` 成功之后，`m`/`ep` 可用于审计与 store）：

```
Resolve → Check(全文) → IncrementHits(照常)
  ├─ DenyIDs 非空 → 403（现有逻辑不动）
  ├─ CaptureIDs 非空 且 触发词位于最后一条用户提问消息（§4.3）
  │     ├─ 该消息之前有消息 → storeTriggerContext(历史) → 200 固定回复
  │     └─ 无历史 → 200 特殊回复
  ├─ OmitIDs 与 CaptureIDs 同时非空（capture 词未落在最后一条用户提问中、未短路）
  │     → storeTriggerContext(最后一条用户提问之前的历史) + SkipStore → 正常转发
  └─ 否则 →（omit 命中则 SkipStore）→ 正常转发
```

注意：capture 词命中但**不在**最后一条用户提问消息中（只在历史/system 里）时，capture 视为未生效——继续走 omit 判定与正常转发，请求行为与不含该词完全一致（除命中计数已递增）。

**2026-08-16 补充（omit+capture 混合 bug 修复）**：原实现中「capture 词只在历史 + omit 词命中最后一条用户提问」时只执行 omit 的跳过存储，capture 的上下文保存完全丢失。修复为：同请求同时命中 omit 与 capture 词（capture 词未短路）时，**两个逻辑都执行**——保存最后一条用户提问之前的历史上下文（与短路保存同一路径 `storeOpenAIChatHistory` / `storeAnthropicHistory` / `captureResponseHistory`，同样提交 capture 审计）、omit 的 SkipStore 照常生效、请求照常转发（不短路，避免打断对话）。仅命中 capture（无 omit）时行为不变：未落在最后一条用户提问则不保存（D6）。

### 4.3 触发位置判定（D6）

对每个协议实现「提取最后一条用户提问消息文本」：

- **Anthropic**：倒序找最后一条 `role=user` 的 message，提取其文本（text + blocks 中 text/thinking，复用 `extractAnthropicMessageText` 的单条变体）。tool_result-only 的 user 消息提取文本为空，`Check("")` 不命中，天然不误触发。
- **OpenAI Chat**：倒序找最后一条 `role=user` 且 `tool_call_id` 为空的消息，同法提取。
- **OpenAI Response**：`input` 为字符串时取该字符串；为 items 时倒序找最后一条 `role=user` 的 message item 提取。

对提取文本单独跑一次 `TriggerChecker.Check`，与 `CaptureIDs` 求交集，非空即真正触发。触发词具体是哪个：`MatchedWords(交集)` 用于审计 remark 与日志。

### 4.4 历史上下文保存（D4/D5）

复用 `MessageStoreTask`（消费端对 Messages 列表无「必须含 assistant」的约束；`RoleAssistant` 才写 `model_id`，全历史消息照常按 checksum 去重落库、建 session）：

- **Anthropic**：`req.Body.Messages[:触发消息索引]` 经 `dto.FromAnthropicMessage` 转 Unified；tools 照常转换；metadata 照常提取；`ModelID = m.ModelID()`。
- **OpenAI Chat**：`req.Body.Messages[:触发消息索引]` 经 `dto.FromOpenAIMessage` 转 Unified。
- **OpenAI Response**：`Instructions` + `input.Items[:索引]`（含字符串 input 场景：字符串 input 本身即触发消息，历史仅剩 Instructions）。
- 触发消息索引 = 0（无历史，D5）→ 不提交 store，直接返回特殊回复。
- `InputTokens/OutputTokens = 0`（无上游调用）。
- **best-effort 语义**：store 任务提交失败（队列满等）仅记录 Error 日志，不改变响应——客户端仍收到固定回复。历史丢失不阻断请求；如需强一致需引入同步落库与错误回复形态，暂不做（YAGNI）。

### 4.5 固定回复（D7）

常量（放 `internal/common/constant`，与现有错误文案同层）：

```go
TriggerCaptureSavedReply  = "Context saved."               // 有历史
TriggerCaptureEmptyReply  = "No conversation history to save." // 无历史
```

六种构造（3 协议 × stream/非 stream），实现放 `internal/application/llmproxy/util`（与 `SendAnthropicContentBlockedError` 同目录），返回 `port.JSONResult` 或 `port.StreamResult`：

- **Anthropic JSON**：`AnthropicMessage{Role: assistant, Content: [text], Model: exposedModel, StopReason: "end_turn", Usage: {0,0}}`。
- **Anthropic SSE**：`message_start → content_block_start → content_block_delta(text_delta) → content_block_stop → message_delta(stop_reason) → message_stop`，一次性吐完结束。
- **OpenAI Chat JSON**：`OpenAIChatCompletion{Choices: [{Message: assistant, FinishReason: stop}], Usage: {0,0}}`。
- **OpenAI Chat SSE**：role chunk → content chunk → `finish_reason=stop` chunk → `data: [DONE]`。
- **OpenAI Response JSON**：`OpenAICreateResponseRsp{Status: completed, Output: [message + output_text], Usage: {0,0}}`。
- **OpenAI Response SSE**：`response.created → response.output_item.added → response.content_part.added → response.output_text.delta → response.content_part.done → response.output_item.done → response.completed`（对齐 `converter/response_stream.go` 的事件形态）。

SSE 形态统一用一个「预生成事件列表」的 `port.Stream` 实现（Read 依序 `sink.WriteEvent` 后返回 nil），不建上游连接。

### 4.6 审计（D10）

capture 短路时照 deny 模式提交 `ModelCallAuditTask`：`UpstreamStatusCode=200`、`APIProtocol` 按入口、`UpstreamProtocol` 按 compatRoute（与 deny 分支同法）、`ErrorMessage = fmt.Sprintf(TriggerCaptureAuditRemarkTemplate, words)`、tokens 全 0。不进 `recordModelCall`（那是上游调用收尾 seam）。

## 5. 部署顺序（D9 全量改名）

1. 发布前在生产库执行：`ALTER TABLE blocked_words RENAME TO trigger_words;`
2. 推送 master → CI 构建镜像 → K8s 滚动部署（代码 `TableName()` 已指向 `triggers`）
3. 部署后跑 `test/e2e/trigger/` Go 用例回归（不再只 curl）
4. 若 e2e 失败取 `X-Trace-Id` 回 CLS 排障

## 6. 测试计划

- **单测**：
  - `TriggerService.CaptureIDs` 过滤逻辑（deny/omit/capture 混合）。
  - 三协议触发位置判定：触发词在最后一条 user 消息 / 仅在历史 / 仅在 system / tool_result-only user 消息。
  - 三协议短路：有历史（store 提交内容 = 历史、无 assistant）/ 无历史（不提交）/ deny 优先 / stream 与非 stream 的响应形态（SSE 事件序列、JSON 字段）。
  - 重命名后现有 blocked 单测全量迁移并保持绿。
- **e2e**（`test/e2e/trigger/`）：
  1. 管理端创建 capture 触发词。
  2. 三协议各发一条带历史的触发请求 → 断言固定回复 + session/message 落库且无触发消息与 assistant 回复。
  3. 发一条无历史的触发请求 → 断言特殊回复。
  4. 发一条含 deny+capture 词的请求 → 断言 403。

## 7. 非目标（YAGNI）

- 不做 per-word 可配置回复文案（D1 已确认统一硬编码；需要时再加列）。
- 不改 `ModelCallAudit` / 限流 / 监控口径（capture 无上游调用，tokens=0 自然不进吞吐统计）。
- 不做 capture 的 dry-run / 预览。
- 不迁移 `cron_call_audits` 历史行的旧模块名。

## 8. 风险与边界

| 风险 | 缓解 |
|------|------|
| 表/路由改名是 breaking change | 前后端同版本发布；e2e 覆盖；部署顺序见 §5 |
| Redis key 切换丢失 pending 命中计数 | 量级小，接受；部署前可手动触发一次 hit 同步 cron |
| 全文匹配 + 「仅最后一条 user 消息生效」的两段判定增加一次 Check 调用 | AC 匹配 O(n)，单条消息文本短，开销可忽略 |
| agentic 客户端对固定回复的 SSE 解析兼容性 | 事件序列对齐现有 converter 产物；e2e 用真实客户端协议形态断言 |
