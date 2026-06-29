# 基于 samber/lo 和 samber/mo 的代码重构设计

**日期**: 2025-06-15
**状态**: 已批准
**类型**: 代码重构（质量改进）

## 目标

使用 `samber/lo`（已引入，v1.39.0）和 `samber/mo`（新引入）的函数式工具简化项目代码，消除手动 for 循环和样板代码，提升可读性和可维护性。

**全量扫描结果**: 覆盖 `internal/` 下全部 357 个 .go 文件，共识别 35 个可重构项。

## 范围

仅涉及 `internal/` 目录下的业务代码，不涉及 `test/`、`docs/`、`web/`。

## 分 5 批实施

### 第 1 批：lo.FilterMap 替换手动 filter+append 模式（17 项）

**1. `session_repository.go` L475-490 — BuildOrderedMessageProjections**
```go
items := make([]*session.MessageDetailProjection, 0, len(ids))
for _, id := range ids {
    m, ok := msgMap[id]
    if !ok { continue }
    items = append(items, &session.MessageDetailProjection{ID: m.ID, ...})
}
```
→ `lo.FilterMap(ids, func(id uint, _ int) (*session.MessageDetailProjection, bool) { ... })`

**2. `session_repository.go` L496-510 — BuildOrderedToolProjections**
同上模式（Tool 版本）→ `lo.FilterMap`

**3. `blocked/service.go` L57-67 — MatchedWords**
```go
words := make([]string, 0, len(ids))
for _, id := range ids {
    if w, ok := s.wordByID[id]; ok {
        words = append(words, w)
    }
}
```
→ `lo.FilterMap(ids, func(id uint, _ int) (string, bool) { w, ok := s.wordByID[id]; return w, ok })`

**4. `base/dao.go` L251-264 — Paginate query fields**
```go
expressions := make([]clause.Expression, 0, len(param.QueryFields))
for _, field := range param.QueryFields {
    if field == "" { continue }
    expressions = append(expressions, clause.Like{...})
}
```
→ `lo.FilterMap(param.QueryFields, func(f string, _ int) (clause.Expression, bool) { ... })`

**5. `audit_repository.go` L305-311 — applyKeywordSearch**
同上字段过滤模式 → `lo.FilterMap`

**6. `filter/parser.go` L56-68 — Parse**
```go
filters := make([]Filter, 0, len(parts))
for _, part := range parts {
    part = strings.TrimSpace(part)
    if part == "" { continue }
    f, err := parsePart(part)
    if err != nil { return nil, err }
    filters = append(filters, f)
}
```
→ 先用 `lo.Filter` 过滤空白，保留 `lo.Map` 尾部的 error 检查

**7. `converter/anthropic.go` L203-211 — resolveContentText**
```go
var texts []string
for _, part := range content.Parts {
    if part.Type == enum.ContentPartTypeText {
        texts = append(texts, lo.FromPtr(part.Text))
    }
}
return strings.Join(texts, "\n")
```
→ `lo.FilterMap + strings.Join`

**8. `converter/anthropic.go` L433-443 — content blocks to texts/thinking**
```go
var textParts []string; var thinkingParts []string
for _, block := range blocks {
    switch block.Type {
    case enum.AnthropicContentBlockTypeText:
        textParts = append(textParts, lo.FromPtr(block.Text))
    case enum.AnthropicContentBlockTypeThinking:
        thinkingParts = append(thinkingParts, lo.FromPtr(block.Thinking))
    }
}
```
→ 两个独立的 `lo.FilterMap` 调用

**9. `converter/openai.go` L271-276 — resolveAnthropicSystem**
```go
var texts []string
for _, block := range system.Blocks {
    if block.Type == enum.AnthropicContentBlockTypeText {
        texts = append(texts, lo.FromPtr(block.Text))
    }
}
```
→ `lo.FilterMap + strings.Join`

**10. `converter/openai.go` L422-426 — resolveAnthropicToolResultContent**
同上 Block.Text 提取 → `lo.FilterMap`

**11. `converter/anthropic_response.go` L308-314 — messageToAnthropicText**
```go
var parts []string
for _, p := range output.FunctionOutput.Parts {
    if p != nil && (p.Type == ...) && p.Text != nil {
        parts = append(parts, lo.FromPtr(p.Text))
    }
}
```
→ `lo.FilterMap`

**12. `converter/anthropic_response.go` L319-329 — reasoningToAnthropicText**
```go
var thinkingParts []string
for _, s := range item.Summary { if s != nil && s.Text != "" { thinkingParts = append(...) } }
for _, c := range item.ReasoningContent { if c != nil && c.Text != "" { thinkingParts = append(...) } }
```
→ 两个 `lo.FilterMap` 调用后 append

**13. `converter/response.go` L186-210 — responseMessageContentToChat**
```go
parts := make([]*dto.OpenAIChatCompletionContentPart, 0, len(content.Parts))
var texts []string
multimodal := false
for _, part := range content.Parts {
    if part == nil { continue }
    switch part.Type { ... }
}
```
→ `lo.FilterMap(content.Parts, ...)` 提取 `parts` 部分

**14. `jwt_session_queries.go` L99-104 — 提取 lastQuestionIDs**
```go
var lastQuestionIDs []uint
for _, p := range projections {
    if len(p.Questions) > 0 {
        lastQuestionIDs = append(lastQuestionIDs, p.Questions[len(p.Questions)-1])
    }
}
```
→ `lo.FilterMap(projections, ...)`

**15. `store_pool.go` L120-127 — upgradeReasoningContent**
```go
var needsUpgradeIDs []uint
msgByID := make(map[uint]*vo.UnifiedMessage)
for i, m := range messages {
    if m.Message.ReasoningContent != "" {
        needsUpgradeIDs = append(needsUpgradeIDs, messageIDs[i])
        msgByID[messageIDs[i]] = m.Message
    }
}
```
→ 提取为 `lo.FilterMap` + 独立的 map 构建

**16. `transport/header_passthrough.go` L41-47 — capturePassthroughResponseHeaders**
```go
headers := make(map[string]string, 4)
for k := range header {
    if isPassthroughResponseHeader(k) {
        headers[k] = header.Get(k)
    }
}
```
→ `lo.PickByKeys(lo.SliceToMap(...))` 或 `lo.FilterMap`

**17. `middleware/header_passthrough.go` L39-44 + `util/header_log.go` L21-29**
```go
masked := make(map[string]any, len(headers))
for key, values := range headers {
    if isSensitiveHTTPHeaderForLog(key) {
        masked[key] = constant.MaskSecretPlaceholder
        continue
    }
    masked[key] = headerValuesForLog(values)
}
```
→ `lo.MapValues(headers, ...)` + 敏感字段单独处理

### 第 2 批：lo.Map 替换手动 Transform（4 项）

**18. `converter/openai.go` L398-401 — contentParts 转 texts**
```go
var texts []string
for _, p := range contentParts {
    texts = append(texts, lo.FromPtr(p.Text))
}
```
→ `lo.Map(contentParts, func(p *dto.OpenAIChatCompletionContentPart, _ int) string { return lo.FromPtr(p.Text) })`

**19. `usecase/blocked_check.go` L66-74 — formatBlockedWords**
```go
quoted := make([]string, len(words))
for i, w := range words {
    quoted[i] = "`" + w + "`"
}
```
→ `lo.Map(words, func(w string, _ int) string { return "`" + w + "`" })`

**20. `jwt_session_queries.go` L117-133 — projections 转 views**
```go
for _, p := range projections {
    summary := ""
    if len(p.Questions) > 0 { ... }
    views = append(views, &sessionport.SessionSummaryView{...})
}
```
→ `lo.Map(projections, ...)` 或 `lo.FilterMap`

**21. `middleware/log.go` L74-82 + L120-128 — 请求头 map 转换**
```go
reqHeaders := make(map[string]any)
for k, v := range c.Request().Header.All() {
    key := string(k); value := string(v)
    if isSensitiveHeader(key) { value = constant.MaskSecretPlaceholder }
    reqHeaders[key] = value
}
```
→ `lo.MapValues(lo.SliceToMap(...), ...)` 或直接提取为辅助函数

### 第 3 批：lo.Keys / lo.SliceToMap / lo.Invert（3 项）

**22. `blocked/matcher.go` L75-79 — map keys to slice**
```go
result := make([]uint, 0, len(matched))
for id := range matched { result = append(result, id) }
```
→ `lo.Keys(matched)` 或 `slices.Collect(maps.Keys(matched))`

**23. `blocked/service.go` L44-47 — Rebuild slice-to-map**
```go
words := make(map[uint]string, len(all))
for _, b := range all { words[b.AggregateID()] = b.Word() }
```
→ `lo.SliceToMap(all, func(b *blocked.Blocked) (uint, string) { return b.AggregateID(), b.Word() })`

**24. `blocked/service.go` L25-30 — Rebuild 构建双倒排索引**
```go
ids := make(map[string]uint, len(words))
byID := make(map[uint]string, len(words))
for id, word := range words { ids[word] = id; byID[id] = word }
```
→ `byID = words; ids = lo.Invert(words)`（`lo.Invert` 反转 map）

### 第 4 批：lo.Ternary / lo.FromPtr 替换条件赋值（4 项）

**25. `base/dao.go` L267-274 — 分页参数默认值**
```go
if param.Sort == "" { param.Sort = enum.SortAsc }
if param.SortField == "" { param.SortField = constant.FieldID }
```
→ `param.Sort = lo.Ternary(param.Sort != "", param.Sort, enum.SortAsc)`

**26. `fill_series.go` L146-154 — 聚合槽 nil 检查**（`internal/application/audit/query/fill_series.go` + `port/fill_series.go` 各一处，共 2 处相同代码）
```go
s := aggregated[t]
if s == nil { s = &throughputSlot{}; aggregated[t] = s }
```
→ `s := lo.Ternary(aggregated[t] != nil, aggregated[t], &throughputSlot{}); aggregated[t] = s`

**27. `usecase/openai.go` L143-150 — baseURL 条件判断**
```go
var baseURL string
if isAnthropic { baseURL = ep.AnthropicBaseURL() } else { baseURL = ep.OpenaiBaseURL() }
```
→ `baseURL := lo.Ternary(isAnthropic, ep.AnthropicBaseURL(), ep.OpenaiBaseURL())`

**28. `usecase/openai.go` L103, 130 — Stream 指针解引用**
```go
stream := req.Body.Stream != nil && *req.Body.Stream
```
→ `stream := lo.FromPtr(req.Body.Stream)`

### 第 5 批：lo.GroupBy + lo.Reduce（1 项）

**29. `audit/query/token_usage.go` L73-87 — aggregateModelUsage**
```go
totals := make(map[string]*dto.ModelUsageItem)
order := make([]string, 0)
for _, p := range points {
    if _, ok := totals[p.Model]; !ok {
        order = append(order, p.Model)
        totals[p.Model] = &dto.ModelUsageItem{Model: p.Model}
    }
    t := totals[p.Model]
    t.InputTokens += p.InputTokens; t.OutputTokens += p.OutputTokens; ...
}
```
→ 先用 `lo.GroupBy(points, func(p *modelcall.TokenThroughputPoint, _ int) string { return p.Model })` 分组，再 `lo.Map` 每组执行 `reduce` 求和

**30. `fill_series.go` L66-86 — indexSeries group-by**
与上面类似的分组累加模式，可引入 `lo.GroupBy` + `lo.Reduce` 重构

### 第 6 批：引入 samber/mo（可选，需添加依赖，4 项）

> 需要先 `go get github.com/samber/mo`

**31. `audit_repository.go` L209-213 — map lookup + 字段赋值**
```go
if user, ok := userByID[relation.UserID]; ok {
    relation.UserName = user.Name
    relation.UserEmail = user.Email
}
```
→ `mo.TupleToOption(userByID[relation.UserID]).ForEach(func(u *dbmodel.User) { relation.UserName = u.Name; relation.UserEmail = u.Email })`

**32. `cron/session_dedup.go` L480-483 — map lookup + nil check**
```go
msg, ok := msgMap[lastMsgID]
if !ok || msg.Message == nil { return }
```
→ `mo.TupleToOption(msgMap[lastMsgID]).FlatMap(func(m *dbmodel.Message) mo.Option[*dbmodel.Message] { ... })`

**33. `converter/response.go` L510-517 — resolveToolCallOutputType map lookup**
```go
if origType, ok := toolTypeMap[functionName]; ok {
    switch origType { ... }
}
```
→ `mo.TupleToOption(toolTypeMap[functionName]).ForEach(func(t string) { ... })`

**34. `blocked/service.go` L70 — nil 检查**
```go
if s.hitRecorder == nil { return nil }
```
→ `mo.PointerToOption(s.hitRecorder).ForEachOrError(...)`

### 第 7 批：提取公共辅助函数（4 项）

**35. `endpoint_repository.go` — 5 处相同的 model→aggregate 转换循环**
5 处相同模式：`make + for range + toEndpointAggregate/toModelAggregate + err check + append`
→ 提取泛型辅助函数 `mapWithErr[T, U any](items []T, fn func(T) (U, error)) ([]U, error)`，但如果该函数放哪呢？放 `util/` 作为一个通用工具

**36. `cache/session_detail.go` — GetMessages 和 GetTools 相同 values 处理**
```go
hits := make(map[uint]T, len(values))
missing := make([]uint, 0, len(ids))
for i, v := range values {
    if v == nil { missing = append(missing, ids[i]); continue }
    raw, ok := v.(string)
    if !ok { missing = append(missing, ids[i]); continue }
    // unmarshal + 错误处理
}
```
→ 提取泛型辅助函数 `parseRedisValues[T any](values []interface{}, ids []uint, unmarshal func(string) (T, error)) (hits map[uint]T, missing []uint, err error)`

**37. `fill_series.go` — port/ 和 query/ 下两处完全相同的 indexSeries 和 aggretateThroughput**
两个文件 `query/fill_series.go` 和 `port/fill_series.go` 中存在完全相同的代码
→ 提取公共函数到 `port/` 层

**38. `middleware/log.go` L74-82 + L120-128 + `util/header_log.go` L21-29**
两处相似的 HTTP header → map 转换
→ 统一到 `util/` 的共享函数

## 不涉及的范围

- DTO 结构体中的 `*string` → `mo.Option[string]` 替换（影响面大，需单独讨论）
- 涉及 error 返回且无法用 lo 函数替代的循环（如 `toEndpointAggregate` 带 error 的 5 处）
- 测试文件的重构
- `internal/application/llmproxy/converter/anthropic_response.go` 中带 error 返回的循环
- `middleware/header_passthrough.go` 的回调结构不应强行换 lo

## 验证

- 每批改动后运行 `make lint`，确保 lint 通过
- 全量改动完成后运行 `make test`，确保所有测试通过
- 所有改动不改变现有行为，纯重构
