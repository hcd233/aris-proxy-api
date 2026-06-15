# Lo/Mo 代码重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用 samber/lo 和 samber/mo 的函数式工具简化 internal/ 下 38 处手动 for 循环和样板代码

**Architecture:** 纯重构，不改行为。分 7 批按"安全→中等→最小风险"的顺序执行。每批结束后运行 `make lint && go test ./internal/...`

**Tech Stack:** Go 1.25.1, samber/lo v1.39.0 (已引入), samber/mo (新引入)

---

## Task 0：全量验证起点

**验证当前代码完整可运行**

- [ ] **运行全量 lint 和 test 记录基线**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
make lint 2>&1 | cat
go test -count=1 ./internal/... 2>&1 | tail -5
```
Expected: 无 error，tests passing.

---

## 第 1 批：lo.FilterMap 替换手动 filter+append

### Task 1: session_repository.go — BuildOrderedMessageProjections

**File:** `internal/infrastructure/repository/session_repository.go:472-491`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前（L477-490）:
items := make([]*session.MessageDetailProjection, 0, len(ids))
for _, id := range ids {
    m, ok := msgMap[id]
    if !ok {
        continue
    }
    items = append(items, &session.MessageDetailProjection{
        ID:        m.ID,
        Model:     m.Model,
        Message:   m.Message,
        CreatedAt: m.CreatedAt,
    })
}
return items

// 修改后:
return lo.FilterMap(ids, func(id uint, _ int) (*session.MessageDetailProjection, bool) {
    m, ok := msgMap[id]
    if !ok {
        return nil, false
    }
    return &session.MessageDetailProjection{
        ID:        m.ID,
        Model:     m.Model,
        Message:   m.Message,
        CreatedAt: m.CreatedAt,
    }, true
})
```

- [ ] **运行验证**

```bash
go build ./...
```

### Task 2: session_repository.go — BuildOrderedToolProjections

**File:** `internal/infrastructure/repository/session_repository.go:493-511`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前（L498-510）:
items := make([]*session.ToolDetailProjection, 0, len(ids))
for _, id := range ids {
    t, ok := toolMap[id]
    if !ok {
        continue
    }
    items = append(items, &session.ToolDetailProjection{
        ID:        t.ID,
        Tool:      t.Tool,
        CreatedAt: t.CreatedAt,
    })
}
return items

// 修改后:
return lo.FilterMap(ids, func(id uint, _ int) (*session.ToolDetailProjection, bool) {
    t, ok := toolMap[id]
    if !ok {
        return nil, false
    }
    return &session.ToolDetailProjection{
        ID:        t.ID,
        Tool:      t.Tool,
        CreatedAt: t.CreatedAt,
    }, true
})
```

- [ ] **运行验证** `go build ./...`

### Task 3: blocked/service.go — MatchedWords

**File:** `internal/application/blocked/service.go:57-67`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前（L60-66）:
words := make([]string, 0, len(ids))
for _, id := range ids {
    if w, ok := s.wordByID[id]; ok {
        words = append(words, w)
    }
}
return words

// 修改后:
return lo.FilterMap(ids, func(id uint, _ int) (string, bool) {
    w, ok := s.wordByID[id]
    return w, ok
})
```

- [ ] **运行验证** `go build ./...`

### Task 4: base/dao.go — Paginate query fields

**File:** `internal/infrastructure/database/dao/base.go:249-265`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前（L251-264）:
if param.Query != "" && len(param.QueryFields) > 0 {
    like := "%" + param.Query + "%"
    expressions := make([]clause.Expression, 0, len(param.QueryFields))
    for _, field := range param.QueryFields {
        if field == "" {
            continue
        }
        expressions = append(expressions, clause.Like{Column: clause.Column{Name: field}, Value: like})
    }

    if len(expressions) > 0 {
        sql = sql.Where(expressions[0])
        for _, expr := range expressions[1:] {
            sql = sql.Or(expr)
        }
    }
}

// 修改后:
if param.Query != "" && len(param.QueryFields) > 0 {
    like := "%" + param.Query + "%"
    expressions := lo.FilterMap(param.QueryFields, func(field string, _ int) (clause.Expression, bool) {
        if field == "" {
            return nil, false
        }
        return clause.Like{Column: clause.Column{Name: field}, Value: like}, true
    })

    if len(expressions) > 0 {
        sql = sql.Where(expressions[0])
        for _, expr := range expressions[1:] {
            sql = sql.Or(expr)
        }
    }
}
```

- [ ] **运行验证** `go build ./...`

### Task 5: audit_repository.go — applyKeywordSearch

**File:** `internal/infrastructure/repository/audit_repository.go:305-311`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前:
expressions := make([]clause.Expression, 0, len(constant.AuditQueryFields))
for _, field := range constant.AuditQueryFields {
    if field == "" {
        continue
    }
    expressions = append(expressions, clause.Like{Column: clause.Column{Name: field}, Value: like})
}

// 修改后:
expressions := lo.FilterMap(constant.AuditQueryFields, func(field string, _ int) (clause.Expression, bool) {
    if field == "" {
        return nil, false
    }
    return clause.Like{Column: clause.Column{Name: field}, Value: like}, true
})
```

- [ ] **运行验证** `go build ./...`

### Task 6: filter/parser.go — Parse

**File:** `internal/common/filter/parser.go:55-69`

- [ ] **先用 lo.Filter 过滤空白，保留 lo.Map 尾部的 error 检查**

解析器中的 error 传播不允许直接使用 lo.FilterMap，所以用两步：先 Filter 空白，保持 Map 的 error 处理不变

```go
// 修改前（L56-69）:
filters := make([]Filter, 0, len(parts))
for _, part := range parts {
    part = strings.TrimSpace(part)
    if part == "" {
        continue
    }
    f, err := parsePart(part)
    if err != nil {
        return nil, err
    }
    filters = append(filters, f)
}

// 修改后（error 传播必须保留手动 map）:
parts = lo.Filter(parts, func(part string, _ int) bool {
    return strings.TrimSpace(part) != ""
})
filters := make([]Filter, 0, len(parts))
for _, part := range parts {
    f, err := parsePart(part)
    if err != nil {
        return nil, err
    }
    filters = append(filters, f)
}
```

- [ ] **运行验证** `go build ./...`

### Task 7: converter/anthropic.go L203-211

**File:** `internal/application/llmproxy/converter/anthropic.go:203-211`

- [ ] **将 filter+append 循环替换为 lo.FilterMap**

```go
// 修改前:
var texts []string
for _, part := range content.Parts {
    if part.Type == enum.ContentPartTypeText {
        texts = append(texts, lo.FromPtr(part.Text))
    }
}
return strings.Join(texts, "\n")

// 修改后:
texts := lo.FilterMap(content.Parts, func(part *dto.OpenAIChatCompletionContentPart, _ int) (string, bool) {
    if part.Type == enum.ContentPartTypeText {
        return lo.FromPtr(part.Text), true
    }
    return "", false
})
return strings.Join(texts, "\n")
```

- [ ] **运行验证** `go build ./...`

### Task 8: converter/anthropic.go L433-443

**File:** `internal/application/llmproxy/converter/anthropic.go:433-443`

- [ ] **将两个 switch 内 append 替换为独立 lo.FilterMap 调用**

需要识别 context 中的完整代码。若该 switch 在循环中，则提取为函数内嵌 FilterMap。如果 switch 结构太复杂不适合直接替换，跳过此项并注释说明。

- [ ] **读取完整代码判断是否适合替换**

```bash
cat -n internal/application/llmproxy/converter/anthropic.go | tail -n +430 | head -20
```

- [ ] **运行验证** `go build ./...`

### Task 9: converter/openai.go L271-276

**File:** `internal/application/llmproxy/converter/openai.go:271-276`

- [ ] **将 filter+append 替换为 lo.FilterMap + strings.Join**

```go
// 修改前:
var texts []string
for _, block := range system.Blocks {
    if block.Type == enum.AnthropicContentBlockTypeText {
        texts = append(texts, lo.FromPtr(block.Text))
    }
}

// 修改后:
texts := lo.FilterMap(system.Blocks, func(block *dto.AnthropicContentBlock, _ int) (string, bool) {
    if block.Type == enum.AnthropicContentBlockTypeText {
        return lo.FromPtr(block.Text), true
    }
    return "", false
})
```

- [ ] **运行验证** `go build ./...`

### Task 10: converter/openai.go L422-426

**File:** `internal/application/llmproxy/converter/openai.go:422-426`

- [ ] **同一模式替换为 lo.FilterMap**

与 Task 9 完全相同的 pattern，替换方式相同。

- [ ] **运行验证** `go build ./...`

### Task 11: converter/anthropic_response.go L308-314

**File:** `internal/application/llmproxy/converter/anthropic_response.go:308-314`

- [ ] **替换为 lo.FilterMap**

```go
// 修改前:
var parts []string
for _, p := range output.FunctionOutput.Parts {
    if p != nil && (p.Type == enum.ResponseContentTypeInputText || p.Type == enum.ResponseContentTypeOutputText) && p.Text != nil {
        parts = append(parts, lo.FromPtr(p.Text))
    }
}
return strings.Join(parts, "\n")

// 修改后:
parts := lo.FilterMap(output.FunctionOutput.Parts, func(p *dto.ResponseInputContent, _ int) (string, bool) {
    if p != nil && (p.Type == enum.ResponseContentTypeInputText || p.Type == enum.ResponseContentTypeOutputText) && p.Text != nil {
        return lo.FromPtr(p.Text), true
    }
    return "", false
})
return strings.Join(parts, "\n")
```

- [ ] **运行验证** `go build ./...`

### Task 12: converter/anthropic_response.go L319-329

**File:** `internal/application/llmproxy/converter/anthropic_response.go:319-329`

- [ ] **两段循环替换为两个 lo.FilterMap**

```go
// 修改前:
var thinkingParts []string
for _, s := range item.Summary {
    if s != nil && s.Text != "" {
        thinkingParts = append(thinkingParts, s.Text)
    }
}
for _, c := range item.ReasoningContent {
    if c != nil && c.Text != "" {
        thinkingParts = append(thinkingParts, c.Text)
    }
}

// 修改后:
thinkingParts := append(
    lo.FilterMap(item.Summary, func(s *dto.ResponseReasoningSummary, _ int) (string, bool) {
        if s != nil && s.Text != "" {
            return s.Text, true
        }
        return "", false
    }),
    lo.FilterMap(item.ReasoningContent, func(c *dto.ResponseReasoningTextContent, _ int) (string, bool) {
        if c != nil && c.Text != "" {
            return c.Text, true
        }
        return "", false
    })...,
)
```

- [ ] **运行验证** `go build ./...`

### Task 13: converter/response.go L186-210

**File:** `internal/application/llmproxy/converter/response.go:185-210`

- [ ] **提取 parts 构建为 lo.FilterMap**

```go
// 修改前:
parts := make([]*dto.OpenAIChatCompletionContentPart, 0, len(content.Parts))
var texts []string
multimodal := false
for _, part := range content.Parts {
    if part == nil {
        continue
    }
    switch part.Type {
    case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
        texts = append(texts, lo.FromPtr(part.Text))
        parts = append(parts, &dto.OpenAIChatCompletionContentPart{...})
    case enum.ResponseContentTypeRefusal:
        texts = append(texts, lo.FromPtr(part.Refusal))
        ...
    }
}

// 修改后（仅 parts 构建部分）:
parts := lo.FilterMap(content.Parts, func(part *dto.ResponseInputContent, _ int) (*dto.OpenAIChatCompletionContentPart, bool) {
    if part == nil {
        return nil, false
    }
    switch part.Type {
    case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
        texts = append(texts, lo.FromPtr(part.Text))
        return &dto.OpenAIChatCompletionContentPart{Type: enum.ContentPartTypeText, Text: part.Text}, true
    case enum.ResponseContentTypeRefusal:
        texts = append(texts, lo.FromPtr(part.Refusal))
        return &dto.OpenAIChatCompletionContentPart{Type: enum.ContentPartTypeRefusal, Refusal: part.Refusal}, true
    ...
    }
    return nil, false
})
```

注意：`texts` 和 `multimodal` 是 side-effect 变量，需要评估是否能保持。如果 FilterMap 回调解耦不方便，跳过此项。

- [ ] **评估代码可行性**

```bash
cat -n internal/application/llmproxy/converter/response.go | tail -n +185 | head -35
```

- [ ] **运行验证** `go build ./...`

### Task 14: jwt_session_queries.go L99-104

**File:** `internal/application/session/query/jwt_session_queries.go:99-104`

- [ ] **替换为 lo.FilterMap**

```go
// 修改前:
var lastQuestionIDs []uint
for _, p := range projections {
    if len(p.Questions) > 0 {
        lastQuestionIDs = append(lastQuestionIDs, p.Questions[len(p.Questions)-1])
    }
}

// 修改后:
lastQuestionIDs := lo.FilterMap(projections, func(p *session.SessionSummaryProjection, _ int) (uint, bool) {
    if len(p.Questions) > 0 {
        return p.Questions[len(p.Questions)-1], true
    }
    return 0, false
})
```

- [ ] **运行验证** `go build ./...`

### Task 15: store_pool.go L120-127

**File:** `internal/infrastructure/pool/store_pool.go:120-127`

- [ ] **提取 needsUpgradeIDs 构建为 lo.FilterMap**

```go
// 修改前:
var needsUpgradeIDs []uint
msgByID := make(map[uint]*vo.UnifiedMessage)
for i, m := range messages {
    if m.Message.ReasoningContent != "" {
        needsUpgradeIDs = append(needsUpgradeIDs, messageIDs[i])
        msgByID[messageIDs[i]] = m.Message
    }
}

// 修改后（needsUpgradeIDs 用 FilterMap，msgByID 保持手动）:
needsUpgradeIDs := lo.FilterMap(messages, func(m *dbmodel.Message, i int) (uint, bool) {
    if m.Message.ReasoningContent != "" {
        msgByID[messageIDs[i]] = m.Message
        return messageIDs[i], true
    }
    return 0, false
})
```
注意：仍需在循环前声明 `msgByID := make(map[uint]*vo.UnifiedMessage)`。

- [ ] **运行验证** `go build ./...`

### Task 16: transport/header_passthrough.go L41-47

**File:** `internal/infrastructure/transport/header_passthrough.go:41-47`

- [ ] **替换为 lo.FilterMap**

```go
// 修改前:
headers := make(map[string]string, 4)
for k := range header {
    if isPassthroughResponseHeader(k) {
        headers[k] = header.Get(k)
    }
}
return headers

// 修改后:
return lo.FilterMap(lo.Keys(header), func(k string, _ int) (map[string]string, bool) {
    if isPassthroughResponseHeader(k) {
        return lo.SliceToMap([]string{k}, func(_ string, _ int) (string, string) { return k, header.Get(k) }), true
    }
    return nil, false
})
```
如果不能直接用 `FilterMap` 构建 map，保留手动循环。

- [ ] **实际读取文件确认最佳替换方案**

```bash
cat -n internal/infrastructure/transport/header_passthrough.go | tail -n +40 | head -12
```

- [ ] **运行验证** `go build ./...`

### Task 17: header_log.go L21-29

**File:** `internal/util/header_log.go:21-29`

- [ ] **用 lo.MapValues 简化 map 转换**

```go
// 修改前:
masked := make(map[string]any, len(headers))
for key, values := range headers {
    if isSensitiveHTTPHeaderForLog(key) {
        masked[key] = constant.MaskSecretPlaceholder
        continue
    }
    masked[key] = headerValuesForLog(values)
}
return masked

// 修改后:
return lo.MapValues(headers, func(values []string, key string) any {
    if isSensitiveHTTPHeaderForLog(key) {
        return constant.MaskSecretPlaceholder
    }
    return headerValuesForLog(values)
})
```

- [ ] **运行验证** `go build ./...`

- [ ] **第 1 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 2 批：lo.Map 替换手动 Transform

### Task 18: converter/openai.go L398-401

**File:** `internal/application/llmproxy/converter/openai.go:398-401`

- [ ] **替换为 lo.Map**

```go
// 修改前:
var texts []string
for _, p := range contentParts {
    texts = append(texts, lo.FromPtr(p.Text))
}

// 修改后:
texts := lo.Map(contentParts, func(p *dto.OpenAIChatCompletionContentPart, _ int) string {
    return lo.FromPtr(p.Text)
})
```

- [ ] **运行验证** `go build ./...`

### Task 19: usecase/blocked_check.go L66-74

**File:** `internal/application/llmproxy/usecase/blocked_check.go:66-74`

- [ ] **替换为 lo.Map**

```go
// 修改前:
quoted := make([]string, len(words))
for i, w := range words {
    quoted[i] = "`" + w + "`"
}
return strings.Join(quoted, constant.BlockedWordSeparator)

// 修改后:
return strings.Join(lo.Map(words, func(w string, _ int) string {
    return "`" + w + "`"
}), constant.BlockedWordSeparator)
```

- [ ] **运行验证** `go build ./...`

### Task 20: jwt_session_queries.go L117-133

**File:** `internal/application/session/query/jwt_session_queries.go:117-133`

- [ ] **投影循环替换为 lo.Map**

```go
// 修改前:
for _, p := range projections {
    summary := ""
    if len(p.Questions) > 0 {
        if m, ok := msgByID[p.Questions[len(p.Questions)-1]]; ok && m.Message != nil {
            summary = util.ExtractMessageText(m.Message.Content)
        }
    }
    views = append(views, &sessionport.SessionSummaryView{
        ID: p.ID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
        Summary: summary, Score: p.Score, MessageCount: p.MessageCount, ToolCount: p.ToolCount,
    })
}
return views, pageInfo, nil

// 修改后:
views := lo.Map(projections, func(p *session.SessionSummaryProjection, _ int) *sessionport.SessionSummaryView {
    summary := ""
    if len(p.Questions) > 0 {
        if m, ok := msgByID[p.Questions[len(p.Questions)-1]]; ok && m.Message != nil {
            summary = util.ExtractMessageText(m.Message.Content)
        }
    }
    return &sessionport.SessionSummaryView{
        ID: p.ID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
        Summary: summary, Score: p.Score, MessageCount: p.MessageCount, ToolCount: p.ToolCount,
    }
})
return views, pageInfo, nil
```

- [ ] **运行验证** `go build ./...`

### Task 21: middleware/log.go L74-82 + L120-128

**File:** `internal/middleware/log.go:74-82` 和 `:120-128`

- [ ] **两处 header→map 转换统一为 lo.MapValues**

需要先读取确认代码上下文：

```bash
cat -n internal/middleware/log.go | tail -n +74 | head -12
cat -n internal/middleware/log.go | tail -n +120 | head -12
```

- [ ] **运行验证** `go build ./...`

- [ ] **第 2 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 3 批：lo.Keys / lo.SliceToMap / lo.Invert

### Task 22: blocked/matcher.go L75-79

**File:** `internal/application/blocked/matcher.go:75-79`

- [ ] **map keys 获取替换为 lo.Keys**

```go
// 修改前:
result := make([]uint, 0, len(matched))
for id := range matched {
    result = append(result, id)
}
return result

// 修改后:
return lo.Keys(matched)
```

- [ ] **运行验证** `go build ./...`

### Task 23: blocked/service.go L44-47

**File:** `internal/application/blocked/service.go:44-47`

- [ ] **slice→map 替换为 lo.SliceToMap**

```go
// 修改前:
words := make(map[uint]string, len(all))
for _, b := range all {
    words[b.AggregateID()] = b.Word()
}

// 修改后:
words := lo.SliceToMap(all, func(b *blocked.Blocked) (uint, string) {
    return b.AggregateID(), b.Word()
})
```

- [ ] **运行验证** `go build ./...`

### Task 24: blocked/service.go L25-30

**File:** `internal/application/blocked/service.go:25-30`

- [ ] **倒排索引构建用 lo.Invert 简化**

```go
// 修改前:
ids := make(map[string]uint, len(words))
byID := make(map[uint]string, len(words))
for id, word := range words {
    ids[word] = id
    byID[id] = word
}

// 修改后:
byID := words  // words 已经是 map[uint]string
ids := lo.Invert(words)  // 反转得到 map[string]uint
```

- [ ] **确认 lo.Invert 在 v1.39.0 可用**

```bash
grep "func Invert" $(go env GOMODCACHE)/github.com/samber/lo@v1.39.0/*.go
```

- [ ] **运行验证** `go build ./...`

- [ ] **第 3 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 4 批：lo.Ternary / lo.FromPtr

### Task 25: base/dao.go L267-274

**File:** `internal/infrastructure/database/dao/base.go:267-274`

- [ ] **默认值赋值替换为 lo.Ternary**

```go
// 修改前:
if param.Sort == "" {
    param.Sort = enum.SortAsc
}
if param.SortField == "" {
    param.SortField = constant.FieldID
}

// 修改后:
param.Sort = lo.Ternary(param.Sort != "", param.Sort, enum.SortAsc)
param.SortField = lo.Ternary(param.SortField != "", param.SortField, constant.FieldID)
```

- [ ] **运行验证** `go build ./...`

### Task 26: fill_series.go — 两处聚合槽 nil 检查

**Files:**
- `internal/application/audit/query/fill_series.go:146-154`
- `internal/application/audit/port/fill_series.go:146-154`

- [ ] **替换 query/ 版本**

```go
// 修改前:
s := aggregated[t]
if s == nil {
    s = &throughputSlot{}
    aggregated[t] = s
}

// 修改后:
s := aggregated[t]
if s == nil {
    s = lo.Ternary(aggregated[t] != nil, aggregated[t], &throughputSlot{})
    aggregated[t] = s
}
```

- [ ] **替换 port/ 版本**

同一模式，相同修改。

- [ ] **运行验证** `go build ./...`

### Task 27: usecase/openai.go L143-150

**File:** `internal/application/llmproxy/usecase/openai.go:143-150`

- [ ] **if/else 替换为 lo.Ternary**

```go
// 修改前:
var baseURL string
if isAnthropic {
    baseURL = ep.AnthropicBaseURL()
} else {
    baseURL = ep.OpenaiBaseURL()
}

// 修改后:
baseURL := lo.Ternary(isAnthropic, ep.AnthropicBaseURL(), ep.OpenaiBaseURL())
```

- [ ] **运行验证** `go build ./...`

### Task 28: usecase/openai.go L103,130

**File:** `internal/application/llmproxy/usecase/openai.go:103,130`

- [ ] **Stream 指针解引用替换为 lo.FromPtr**

```go
// 修改前:
stream := req.Body.Stream != nil && *req.Body.Stream

// 修改后:
stream := lo.FromPtr(req.Body.Stream)
```

确认两处（L103 和 L130）均替换。

- [ ] **运行验证** `go build ./...`

- [ ] **第 4 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 5 批：lo.GroupBy + lo.Reduce

### Task 29: token_usage.go — aggregateModelUsage

**File:** `internal/application/audit/query/token_usage.go:73-87`

- [ ] **group-by + accumulate 替换为 lo.GroupBy + lo.Map reduce**

```go
// 修改前:
func aggregateModelUsage(points []*modelcall.TokenThroughputPoint) []*dto.ModelUsageItem {
    totals := make(map[string]*dto.ModelUsageItem)
    order := make([]string, 0)
    for _, p := range points {
        if _, ok := totals[p.Model]; !ok {
            order = append(order, p.Model)
            totals[p.Model] = &dto.ModelUsageItem{Model: p.Model}
        }
        t := totals[p.Model]
        t.InputTokens += p.InputTokens
        t.OutputTokens += p.OutputTokens
        t.CacheReadTokens += p.CacheReadTokens
        t.CacheCreationTokens += p.CacheCreationTokens
    }
    items := lo.Map(order, func(m string, _ int) *dto.ModelUsageItem { return totals[m] })
    return items
}

// 修改后:
func aggregateModelUsage(points []*modelcall.TokenThroughputPoint) []*dto.ModelUsageItem {
    groups := lo.GroupBy(points, func(p *modelcall.TokenThroughputPoint, _ int) string {
        return p.Model
    })
    return lo.Map(lo.Keys(groups), func(model string, _ int) *dto.ModelUsageItem {
        item := &dto.ModelUsageItem{Model: model}
        for _, p := range groups[model] {
            item.InputTokens += p.InputTokens
            item.OutputTokens += p.OutputTokens
            item.CacheReadTokens += p.CacheReadTokens
            item.CacheCreationTokens += p.CacheCreationTokens
        }
        return item
    })
}
```

- [ ] **运行验证** `go build ./...`

### Task 30: fill_series.go — indexSeries

**Files:**
- `internal/application/audit/query/fill_series.go:65-86`
- `internal/application/audit/port/fill_series.go:65-86`

- [ ] **同样替换为 lo.GroupBy**

同上模式，indexSeries 是分组索引构建。但这个函数同时构建了 `modelOrder`, `byModel`, `timeSet` 三个输出，分组提取可替换其中一部分。读取确认后决定是否替换。

```bash
cat -n internal/application/audit/query/fill_series.go | tail -n +65 | head -25
```

- [ ] **运行验证** `go build ./...`

- [ ] **第 5 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 6 批：samber/mo 引入

### Task 31: 添加 samber/mo 依赖

- [ ] **添加依赖**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
go get github.com/samber/mo@latest
go mod tidy
```

- [ ] **验证**

```bash
grep "samber/mo" go.mod
go build ./...
```

### Task 32: audit_repository.go L209-213

**File:** `internal/infrastructure/repository/audit_repository.go:208-214`

- [ ] **map lookup + 字段赋值替换为 mo.TupleToOption + ForEach**

```go
// 修改前:
for _, relation := range relations {
    if user, ok := userByID[relation.UserID]; ok {
        relation.UserName = user.Name
        relation.UserEmail = user.Email
    }
}

// 修改后:
for _, relation := range relations {
    mo.TupleToOption(userByID[relation.UserID]).ForEach(func(user *dbmodel.User) {
        relation.UserName = user.Name
        relation.UserEmail = user.Email
    })
}
```

- [ ] **运行验证** `go build ./...`

### Task 33: cron/session_dedup.go L480-483

**File:** `internal/cron/session_dedup.go:480-483`

- [ ] **map lookup + nil check 替换为 mo.Option 链**

```go
// 修改前:
lastMsgID := s.MessageIDs[len(s.MessageIDs)-1]
msg, ok := msgMap[lastMsgID]
if !ok || msg.Message == nil {
    return
}

// 修改后:
lastMsgID := s.MessageIDs[len(s.MessageIDs)-1]
hasMessage := mo.TupleToOption(msgMap[lastMsgID]).
    FlatMap(func(m *dbmodel.Message) mo.Option[*dbmodel.Message] {
        if m.Message == nil {
            return mo.None[*dbmodel.Message]()
        }
        return mo.Some(m)
    }).
    IsPresent()
if !hasMessage {
    return
}
```

- [ ] **运行验证** `go build ./...`

### Task 34: converter/response.go L510-517

**File:** `internal/application/llmproxy/converter/response.go:510-517`

- [ ] **map lookup + switch 替换为 mo.TupleToOption + ForEach**

```go
// 修改前:
if origType, ok := toolTypeMap[functionName]; ok {
    switch origType {
    case enum.ResponseToolTypeLocalShell:
        return enum.ResponseInputItemTypeLocalShellCall
    case enum.ResponseToolTypeCustom, enum.ResponseToolTypeApplyPatch, enum.ResponseToolTypeShell:
        return enum.ResponseInputItemTypeCustomToolCall
    }
}
return enum.ResponseInputItemTypeFunctionCall

// 修改后:
return mo.TupleToOption(toolTypeMap[functionName]).
    Map(func(origType string) string {
        switch origType {
        case enum.ResponseToolTypeLocalShell:
            return enum.ResponseInputItemTypeLocalShellCall
        case enum.ResponseToolTypeCustom, enum.ResponseToolTypeApplyPatch, enum.ResponseToolTypeShell:
            return enum.ResponseInputItemTypeCustomToolCall
        default:
            return enum.ResponseInputItemTypeFunctionCall
        }
    }).
    OrElse(enum.ResponseInputItemTypeFunctionCall)
```

- [ ] **运行验证** `go build ./...`

### Task 35: blocked/service.go L70

**File:** `internal/application/blocked/service.go:70`

- [ ] **nil 检查替换为 mo.PointerToOption**

```go
// 修改前:
if s.hitRecorder == nil {
    return nil
}
return s.hitRecorder.IncrementHits(ctx, ids)

// 修改后:
return mo.PointerToOption(s.hitRecorder).
    Map(func(r port.HitRecorder) error { return r.IncrementHits(ctx, ids) }).
    OrElse(nil)
```
注意：`Map` 返回 `mo.Option[error]`，需要额外处理。更合适的写法是保留简单的 nil 检查。

如果发现 mo 包装使代码更复杂，跳过此项并注释说明。

- [ ] **运行验证** `go build ./...`

- [ ] **第 6 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 第 7 批：提取公共辅助函数

### Task 36: endpoint_repository.go — 泛型 mapWithErr 辅助函数

**File:** `internal/infrastructure/repository/endpoint_repository.go`
**新增:** `internal/util/map_with_err.go`

- [ ] **创建泛型 mapWithErr 辅助函数**

```go
// internal/util/map_with_err.go
package util

import "github.com/hcd233/aris-proxy-api/internal/common/ierr"

// MapWithErr 对切片中每个元素应用转换函数，遇到错误立即返回。
// 泛型函数，T = 输入类型, U = 输出类型。
func MapWithErr[T, U any](items []T, fn func(T) (U, error)) ([]U, error) {
    result := make([]U, 0, len(items))
    for _, item := range items {
        out, err := fn(item)
        if err != nil {
            return nil, err
        }
        result = append(result, out)
    }
    return result, nil
}
```

- [ ] **替换 endpoint_repository.go 中 5 处手动循环**

```go
// 修改前（以 L141-148 为例）:
result := make([]*aggregate.Endpoint, 0, len(models))
for _, m := range models {
    ep, err := toEndpointAggregate(m)
    if err != nil {
        return nil, err
    }
    result = append(result, ep)
}

// 修改后:
result, err := util.MapWithErr(models, func(m *dbmodel.Endpoint) (*aggregate.Endpoint, error) {
    return toEndpointAggregate(m)
})
if err != nil {
    return nil, err
}
```

对其他 4 处（L171-178, L200-208, L293-301, L323-330）做相同替换。

- [ ] **运行验证** `go build ./...`

### Task 37: cache/session_detail.go — 泛型 parseRedisValues 辅助函数

**File:** `internal/infrastructure/cache/session_detail.go`
**新增:** `internal/infrastructure/cache/redis_helper.go`

- [ ] **创建泛型 parseRedisValues 辅助函数**

```go
// internal/infrastructure/cache/redis_helper.go
package cache

import (
    "github.com/bytedance/sonic"
    "github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// parseRedisValues 从 Redis MGet 返回值中解析指定类型的记录。
// 返回 hits map 和 miss 的 id 列表。
func parseRedisValues[T any](values []interface{}, ids []uint) (hits map[uint]*T, missing []uint) {
    hits = make(map[uint]*T, len(values))
    missing = make([]uint, 0, len(ids))
    for i, v := range values {
        if v == nil {
            missing = append(missing, ids[i])
            continue
        }
        raw, ok := v.(string)
        if !ok {
            missing = append(missing, ids[i])
            continue
        }
        var record T
        if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
            missing = append(missing, ids[i])
            continue
        }
        hits[ids[i]] = &record
    }
    return hits, missing
}
```

- [ ] **替换 GetMessages 中的逻辑**

```go
// 修改前:
hits := make(map[uint]*sessionport.MessageCacheRecord, len(values))
missing := make([]uint, 0, len(ids))
for i, v := range values {
    if v == nil {
        missing = append(missing, ids[i])
        continue
    }
    // ... 类型断言、反序列化 ...
}

// 修改后:
hits, missing = parseRedisValues[sessionport.MessageCacheRecord](values, ids)
```

- [ ] **替换 GetTools 中的逻辑**

与 GetMessages 相同替换。

- [ ] **运行验证** `go build ./...`

### Task 38: fill_series.go — 消除 query/ 和 port/ 重复代码

**Files:**
- `internal/application/audit/query/fill_series.go`
- `internal/application/audit/port/fill_series.go`

- [ ] **将重复的 indexSeries 和 aggretateThroughput 提取到 port/ 层公共文件**

读取确认两者是否确实完全一致：

```bash
diff \
  internal/application/audit/query/fill_series.go \
  internal/application/audit/port/fill_series.go
```

如果完全一致，移除 query/ 版本并在 query/ 中引用 port/ 版本。

- [ ] **运行验证** `go build ./...`

- [ ] **第 7 批全量验证**

```bash
go build ./...
make lint
go test -count=1 ./internal/...
```

---

## 最终全量验证

- [ ] **全量 lint**

```bash
make lint 2>&1 | cat
```

Expected: 0 warnings, 0 errors.

- [ ] **全量单元测试**

```bash
go test -count=1 ./... 2>&1 | cat
```

Expected: 全部 OK/SKIP，无 FAIL。

- [ ] **编译验证**

```bash
make build 2>&1 | cat
```
