# Session 列表按模型筛选实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Web 端 Sessions 列表支持按模型多选筛选，后端 `/api/v1/session/list` 支持 `model:xxx|yyy` filter 表达式，`/api/v1/session/option/list` 支持 `field=model`。

**Architecture:** 扩展通用 `internal/common/filter` 包新增 JSONB 数组精确包含筛选能力；在 session 应用层注册 `model` 字段；仓储层新增 `ListDistinctModels` 供前端下拉选项；Web 端新增 Model `MultiSelectPill` 并与 Score 组合生成 filter 表达式。

**Tech Stack:** Go 1.25, GORM, PostgreSQL, Huma, React + TypeScript + Tailwind v4, shadcn/ui

---

## File Map

| 文件 | 职责 |
|---|---|
| `internal/common/filter/parser.go` | 扩展 `FieldConfig` 与条件生成，支持 JSONB 数组 `@>` 语义 |
| `test/unit/filter/filter_test.go` | 新增：验证 JSONB 数组 filter 条件生成 |
| `internal/common/constant/sql.go` | 新增 session model filter 与去重查询常量 |
| `internal/application/session/query/jwt_session_queries.go` | 在 `sessionFieldConfigs` 注册 `model` 字段 |
| `internal/application/session/query/option_list.go` | 支持 `field=model` 分发到 `ListDistinctModels` |
| `internal/domain/session/repository.go` | `SessionReadRepository` 接口新增 `ListDistinctModels` |
| `internal/infrastructure/repository/session_repository.go` | 实现 `ListDistinctModels` |
| `test/unit/session_option_list/session_option_list_test.go` | 新增：验证 option_list 字段分发 |
| `internal/dto/session.go` | 更新 `SessionOptionListReq.Field` doc |
| `internal/router/session.go` | 更新 `listSessionOptions` 接口描述 |
| `web/src/lib/types.ts` | `SessionOptionListReq.field` 联合类型增加 `"model"` |
| `web/src/app/(dashboard)/sessions/page.tsx` | 新增 Model 筛选器 UI 与 filter 组合逻辑 |
| `test/e2e/session_list_filter_model/session_list_filter_model_test.go` | 新增：端到端验证筛选与选项接口 |

---

## Task 1: 扩展 filter 包支持 JSONB 数组筛选

**Files:**
- Create: `test/unit/filter/filter_test.go`
- Modify: `internal/common/filter/parser.go`

- [ ] **Step 1: 写 filter 单元测试**

创建 `test/unit/filter/filter_test.go`：

```go
package filter

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
)

func TestBuildJSONBArrayCondition_SingleEqual(t *testing.T) {
	t.Parallel()
	f := filter.Filter{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o"}}
	cfg := filter.FieldConfig{SQLColumn: "models"}
	// 通过 FilterCriteria 走 ToSQL 验证完整链路
	criteria := &filter.FilterCriteria{
		Filters:      []filter.Filter{f},
		FieldConfigs: map[string]filter.FieldConfig{"model": {SQLColumn: "models", IsJSONBArray: true}},
	}
	sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "models::jsonb @> jsonb_build_array(?)"
	if sql != want {
		t.Errorf("sql mismatch\nwant: %q\ngot:  %q", want, sql)
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args mismatch, want [gpt-4o], got %v", args)
	}
}

func TestBuildJSONBArrayCondition_MultiEqual(t *testing.T) {
	t.Parallel()
	criteria := &filter.FilterCriteria{
		Filters:      []filter.Filter{{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o", "claude-3-5-sonnet"}}},
		FieldConfigs: map[string]filter.FieldConfig{"model": {SQLColumn: "models", IsJSONBArray: true}},
	}
	sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain jsonb_build_array condition, got %q", sql)
	}
	if !strings.Contains(sql, " OR ") {
		t.Errorf("multi-value equal should use OR, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}

func TestBuildJSONBArrayCondition_SingleNotEqual(t *testing.T) {
	t.Parallel()
	criteria := &filter.FilterCriteria{
		Filters:      []filter.Filter{{Field: "model", Operator: enum.OpNotEqual, Values: []string{"gpt-4o"}}},
		FieldConfigs: map[string]filter.FieldConfig{"model": {SQLColumn: "models", IsJSONBArray: true}},
	}
	sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "NOT models::jsonb @> jsonb_build_array(?)"
	if sql != want {
		t.Errorf("sql mismatch\nwant: %q\ngot:  %q", want, sql)
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args mismatch, want [gpt-4o], got %v", args)
	}
}

func TestBuildJSONBArrayCondition_MultiNotEqual(t *testing.T) {
	t.Parallel()
	criteria := &filter.FilterCriteria{
		Filters:      []filter.Filter{{Field: "model", Operator: enum.OpNotEqual, Values: []string{"gpt-4o", "claude-3-5-sonnet"}}},
		FieldConfigs: map[string]filter.FieldConfig{"model": {SQLColumn: "models", IsJSONBArray: true}},
	}
	sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "NOT models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain NOT condition, got %q", sql)
	}
	if !strings.Contains(sql, " AND ") {
		t.Errorf("multi-value not-equal should use AND, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}

func TestBuildJSONBArrayCondition_UnsupportedComparison(t *testing.T) {
	t.Parallel()
	criteria := &filter.FilterCriteria{
		Filters:      []filter.Filter{{Field: "model", Operator: enum.OpGreater, Values: []string{"gpt-4o"}}},
		FieldConfigs: map[string]filter.FieldConfig{"model": {SQLColumn: "models", IsJSONBArray: true}},
	}
	_, _, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err == nil {
		t.Fatal("expected error for unsupported operator, got nil")
	}
}

func TestBuildJSONBArrayCondition_CombinedWithOtherField(t *testing.T) {
	t.Parallel()
	criteria := &filter.FilterCriteria{
		Filters: []filter.Filter{
			{Field: "score", Operator: enum.OpEqual, Values: []string{"5"}},
			{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o"}},
		},
		FieldConfigs: map[string]filter.FieldConfig{
			"score": {SQLColumn: "score"},
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "score = ?") {
		t.Errorf("sql should contain score condition, got %q", sql)
	}
	if !strings.Contains(sql, "models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain model condition, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -v -count=1 ./test/unit/filter/`

Expected: 编译失败或测试失败，提示 `IsJSONBArray` / `buildJSONBArrayCondition` 未定义。

- [ ] **Step 3: 实现 filter JSONB 数组支持**

修改 `internal/common/filter/parser.go`：

1. `FieldConfig` 增加字段：

```go
type FieldConfig struct {
	SQLColumn    string
	IsFuzzy      bool
	IsNumeric    bool
	IsJSONBArray bool
	ValueMap     map[string]*string
}
```

2. `buildCondition` 增加分支：

```go
func buildCondition(f Filter, config FieldConfig) (sql string, args []any, err error) {
	column := config.SQLColumn

	if !isMultiValueAllowed(f.Operator) && len(f.Values) > 1 {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrMultiValueWithComparison, f.Operator)
	}

	if config.IsJSONBArray {
		return buildJSONBArrayCondition(column, f)
	}

	if config.ValueMap != nil {
		return buildValueMapCondition(column, f, config)
	}
	// ... 后续不变
}
```

3. 新增 `buildJSONBArrayCondition`：

```go
// buildJSONBArrayCondition 构建 JSONB 数组包含条件
// 单值：models::jsonb @> jsonb_build_array(?)
// 多值：OR 连接多个 @> 条件
// 不等：NOT models::jsonb @> jsonb_build_array(?)
func buildJSONBArrayCondition(column string, f Filter) (sql string, args []any, err error) {
	if !isMultiValueAllowed(f.Operator) {
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}

	jsonbColumn := column + "::jsonb"
	frag := jsonbColumn + " @> jsonb_build_array(?)"

	switch f.Operator {
	case enum.OpEqual:
		if len(f.Values) == 1 {
			return frag, []any{f.Values[0]}, nil
		}
		parts := make([]string, len(f.Values))
		args = make([]any, len(f.Values))
		for i, v := range f.Values {
			parts[i] = frag
			args[i] = v
		}
		return "(" + strings.Join(parts, constant.FilterSQLOR) + ")", args, nil
	case enum.OpNotEqual:
		if len(f.Values) == 1 {
			return "NOT " + frag, []any{f.Values[0]}, nil
		}
		parts := make([]string, len(f.Values))
		args = make([]any, len(f.Values))
		for i, v := range f.Values {
			parts[i] = "NOT " + frag
			args[i] = v
		}
		return "(" + strings.Join(parts, constant.FilterSQLAND) + ")", args, nil
	default:
		return "", nil, ierr.Newf(ierr.ErrBadRequest, constant.FilterErrUnsupportedOp, f.Operator)
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -v -count=1 ./test/unit/filter/`

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/common/filter/parser.go test/unit/filter/filter_test.go
git commit -m "feat(filter): add JSONB array contains filter support"
```

---

## Task 2: Session 后端注册 model 筛选字段

**Files:**
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/application/session/query/jwt_session_queries.go`

- [ ] **Step 1: 新增常量**

修改 `internal/common/constant/sql.go`，在 `SessionDistinctScoreOrder` 附近新增：

```go
	SessionFilterFieldModel       = "model"
	SessionFilterModelSQLColumn   = "models"

	SessionDistinctModelSelect    = "DISTINCT jsonb_array_elements_text(models::jsonb) AS model"
	SessionDistinctModelWhere     = "models IS NOT NULL AND models::jsonb <> '[]'::jsonb"
	SessionDistinctModelOrder     = "model ASC"
	SessionDistinctModelLimit     = 50
```

- [ ] **Step 2: 注册 model 字段**

修改 `internal/application/session/query/jwt_session_queries.go`，更新 `sessionFieldConfigs`：

```go
var sessionFieldConfigs = map[string]filter.FieldConfig{
	constant.FieldScore: {
		SQLColumn: constant.FieldScore,
		ValueMap: map[string]*string{
			constant.SessionOptionScoreValueUnscored: nil,
		},
	},
	constant.SessionFilterFieldModel: {
		SQLColumn:    constant.SessionFilterModelSQLColumn,
		IsJSONBArray: true,
	},
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/common/constant/sql.go internal/application/session/query/jwt_session_queries.go
git commit -m "feat(session): register model field in session filter config"
```

---

## Task 3: Session 选项接口支持 model

**Files:**
- Modify: `internal/application/session/query/option_list.go`
- Modify: `internal/domain/session/repository.go`
- Modify: `internal/infrastructure/repository/session_repository.go`
- Create: `test/unit/session_option_list/session_option_list_test.go`

- [ ] **Step 1: 扩展 option_list 字段分发**

修改 `internal/application/session/query/option_list.go`：

```go
func (h *listSessionOptionHandler) Handle(ctx context.Context, q sessionport.ListSessionOptionQuery) ([]string, error) {
	switch q.Field {
	case constant.FieldScore:
		items := []string{constant.SessionOptionScoreValueUnscored}
		scores, err := h.readRepo.ListDistinctScores(ctx, q.StartTime, q.EndTime)
		if err != nil {
			return nil, err
		}
		for _, s := range scores {
			if s >= 1 && s <= 5 {
				items = append(items, strconv.Itoa(s))
			}
		}
		if q.Keyword != "" {
			filtered := lo.Filter(items, func(item string, _ int) bool {
				return strings.Contains(item, q.Keyword)
			})
			return filtered, nil
		}
		return items, nil
	case constant.SessionFilterFieldModel:
		return h.readRepo.ListDistinctModels(ctx, q.Keyword, q.StartTime, q.EndTime)
	default:
		return []string{}, nil
	}
}
```

- [ ] **Step 2: 扩展 SessionReadRepository 接口**

修改 `internal/domain/session/repository.go`，在 `ListDistinctScores` 下方新增：

```go
	// ListDistinctModels 查询去重的模型列表（支持时间范围与关键字过滤）
	ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
```

- [ ] **Step 3: 实现 ListDistinctModels**

修改 `internal/infrastructure/repository/session_repository.go`，在 `ListDistinctScores` 之后新增：

```go
// ListDistinctModels 查询去重的模型列表
func (r *sessionReadRepository) ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	db := r.db.WithContext(ctx)

	var models []string
	query := db.Model(&dbmodel.Session{}).
		Select(constant.SessionDistinctModelSelect).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.SessionDistinctModelWhere)

	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where("jsonb_array_elements_text(models::jsonb) ILIKE ?", "%"+keyword+"%")
	}

	if err := query.Order(constant.SessionDistinctModelOrder).Limit(constant.SessionDistinctModelLimit).Scan(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct models")
	}

	return models, nil
}
```

- [ ] **Step 4: 写单元测试**

创建 `test/unit/session_option_list/session_option_list_test.go`：

```go
package session_option_list

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

type fakeSessionReadRepo struct {
	listDistinctModelsCalled bool
	listDistinctScoresCalled bool
}

func (r *fakeSessionReadRepo) ListAllSessions(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *fakeSessionReadRepo) ListSessionsByOwnerNames(ctx context.Context, ownerNames []string, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*session.SessionSummaryProjection, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *fakeSessionReadRepo) GetSessionDetail(ctx context.Context, id uint) (*session.SessionDetailProjection, error) {
	return nil, nil
}
func (r *fakeSessionReadRepo) GetSessionMeta(ctx context.Context, id uint) (*session.SessionMetaProjection, error) {
	return nil, nil
}
func (r *fakeSessionReadRepo) FindMessagesByIDs(ctx context.Context, ids []uint) ([]*session.MessageDetailProjection, error) {
	return nil, nil
}
func (r *fakeSessionReadRepo) FindToolsByIDs(ctx context.Context, ids []uint) ([]*session.ToolDetailProjection, error) {
	return nil, nil
}
func (r *fakeSessionReadRepo) ListDistinctScores(ctx context.Context, startTime, endTime time.Time) ([]int, error) {
	r.listDistinctScoresCalled = true
	return nil, nil
}
func (r *fakeSessionReadRepo) ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	r.listDistinctModelsCalled = true
	return []string{"gpt-4o", "claude-3-5-sonnet"}, nil
}

func TestListSessionOptionHandler_FieldModel(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.SessionFilterFieldModel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listDistinctModelsCalled {
		t.Error("expected ListDistinctModels to be called")
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestListSessionOptionHandler_FieldScore(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	_, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: constant.FieldScore})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.listDistinctScoresCalled {
		t.Error("expected ListDistinctScores to be called")
	}
}

func TestListSessionOptionHandler_UnknownField(t *testing.T) {
	t.Parallel()
	repo := &fakeSessionReadRepo{}
	handler := query.NewListSessionOptionHandler(repo)
	items, err := handler.Handle(context.Background(), port.ListSessionOptionQuery{Field: "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty items, got %d", len(items))
	}
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test -v -count=1 ./test/unit/session_option_list/`

Expected: 全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/application/session/query/option_list.go internal/domain/session/repository.go internal/infrastructure/repository/session_repository.go test/unit/session_option_list/session_option_list_test.go
git commit -m "feat(session): support model option list and distinct models query"
```

---

## Task 4: 更新 DTO 与 Router 文档

**Files:**
- Modify: `internal/dto/session.go`
- Modify: `internal/router/session.go`

- [ ] **Step 1: 更新 DTO doc**

修改 `internal/dto/session.go` 中 `SessionOptionListReq`：

```go
// SessionOptionListReq 获取 Session 筛选选项请求（JWT 认证）
//
//	@author centonhuang
//	@update 2026-06-16 14:00:00
type SessionOptionListReq struct {
	Field     string    `query:"field" required:"true" doc:"筛选字段，可选值：score, model"`
	Keyword   string    `query:"keyword" maxLength:"100" doc:"搜索关键词"`
	StartTime time.Time `query:"startTime" doc:"开始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
}
```

- [ ] **Step 2: 更新 Router 描述**

修改 `internal/router/session.go` 中 `listSessionOptions` 的 Description：

```go
Description: "Get available options for session filter fields (score, model)",
```

- [ ] **Step 3: Commit**

```bash
git add internal/dto/session.go internal/router/session.go
git commit -m "docs(session): update option list field enum docs"
```

---

## Task 5: Web 前端新增 Model 筛选器

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 更新 TypeScript 类型**

修改 `web/src/lib/types.ts` 中 `SessionOptionListReq`：

```ts
export interface SessionOptionListReq {
  field: "score" | "model";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}
```

- [ ] **Step 2: 更新 Sessions 页面**

修改 `web/src/app/(dashboard)/sessions/page.tsx`：

1. 新增 state：

```ts
const [filterModel, setFilterModel] = useState<string[]>([]);
const [modelOptions, setModelOptions] = useState<string[]>([]);
```

2. 新增获取 model 选项回调（放在 `fetchScoreOptions` 附近）：

```ts
const fetchModelOptions = useCallback(async (range: TimeRangeKey, cs: string, ce: string) => {
  const { startTime, endTime } = computeRange(range, cs, ce);
  try {
    const rsp = await api.listSessionOptions({ field: "model", startTime, endTime });
    if (!rsp.error && rsp.items) setModelOptions(rsp.items);
  } catch (err) {
    console.error("Failed to load model options:", err);
  }
}, []);
```

3. 在 time range effect 中同时拉取 model options：

```ts
useEffect(() => {
  fetchScoreOptions(timeRange, customStart, customEnd);
  fetchModelOptions(timeRange, customStart, customEnd);
}, [timeRange, customStart, customEnd, fetchScoreOptions, fetchModelOptions]);
```

4. 修改 `buildSessionFilter`：

```ts
const buildSessionFilter = (scores: string[], models: string[]): string | undefined => {
  const parts: string[] = [];
  if (scores.length > 0) parts.push(`score:${scores.join("|")}`);
  if (models.length > 0) parts.push(`model:${models.join("|")}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
};
```

5. 修改 `fetchSessions` 签名，增加 `models: string[]` 参数：

```ts
const fetchSessions = useCallback(
  async (
    page: number,
    pageSize: number,
    range: TimeRangeKey,
    cs: string,
    ce: string,
    sortState: { field: string; dir: SortDir },
    kw: string,
    score: string[],
    models: string[],
  ) => {
    setLoading(true);
    try {
      const { startTime, endTime } = computeRange(range, cs, ce);
      const rsp = await api.listSessions({
        page,
        pageSize,
        sort: sortState.dir,
        sortField: sortState.field,
        startTime,
        endTime,
        keyword: kw || undefined,
        filter: buildSessionFilter(score, models),
      });
      // ... 后续不变
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  },
  [setPersistedPage, setPersistedPageSize],
);
```

6. 更新所有 `fetchSessions` 调用点，传入 `filterScore` 和 `filterModel`：
   - 初始加载：`fetchSessions(persistedPage, persistedPageSize, "30d", "", "", { field: "created_at", dir: "desc" }, "", [], [])`
   - `refresh` 函数
   - `handleSort`
   - `handleSearch`
   - `handleDelete` 后的刷新
   - `handleBatchDelete` 后的刷新
   - TimeRangePicker onChange
   - Score MultiSelectPill onChange

7. 在 Score `MultiSelectPill` 后新增 Model `MultiSelectPill`：

```tsx
<MultiSelectPill
  label="Model"
  options={modelOptions}
  value={filterModel}
  onChange={(v) => {
    setFilterModel(v);
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, filterScore, v);
  }}
/>
```

8. 更新 Clear filters 按钮条件与行为：

```tsx
{(filterScore.length > 0 || filterModel.length > 0) && (
  <Button
    variant="ghost"
    size="sm"
    className="gap-1 text-muted-foreground"
    onClick={() => {
      setFilterScore([]);
      setFilterModel([]);
      fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, [], []);
    }}
  >
    <X className="size-3.5" />
    Clear
  </Button>
)}
```

- [ ] **Step 3: 跑前端 lint 与 build**

Run:
```bash
cd web && npm run lint && npm run build
```

Expected: 无 lint 错误，build 成功。

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/app/(dashboard)/sessions/page.tsx
git commit -m "feat(web): add model filter to sessions list"
```

---

## Task 6: E2E 测试

**Files:**
- Create: `test/e2e/session_list_filter_model/session_list_filter_model_test.go`

- [ ] **Step 1: 写 E2E 测试**

创建 `test/e2e/session_list_filter_model/session_list_filter_model_test.go`：

```go
package session_list_filter_model

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/test/e2e/helper"
)

func TestSessionListFilterByModel(t *testing.T) {
	t.Parallel()
	ctx := helper.NewE2EContext(t)

	// 准备数据：创建 3 个不同 models 的 session
	// 具体 helper 根据项目现有 e2e helper 调整
	s1 := ctx.CreateSessionWithModels(t, []string{"gpt-4o"})
	s2 := ctx.CreateSessionWithModels(t, []string{"claude-3-5-sonnet"})
	s3 := ctx.CreateSessionWithModels(t, []string{"gpt-4o", "claude-3-5-sonnet"})

	// 按单个 model 筛选
	rsp := ctx.ListSessions(t, helper.ListSessionsParams{Filter: "model:gpt-4o"})
	ctx.AssertSessionIDs(t, rsp, []uint{s1.ID, s3.ID})

	// 按多个 model 筛选（OR）
	rsp = ctx.ListSessions(t, helper.ListSessionsParams{Filter: "model:gpt-4o|claude-3-5-sonnet"})
	ctx.AssertSessionIDs(t, rsp, []uint{s1.ID, s2.ID, s3.ID})

	// 按 NOT model 筛选
	rsp = ctx.ListSessions(t, helper.ListSessionsParams{Filter: "model:!gpt-4o"})
	ctx.AssertSessionIDs(t, rsp, []uint{s2.ID})

	// 验证选项接口
	options := ctx.ListSessionOptions(t, "model")
	ctx.AssertContains(t, options, "gpt-4o")
	ctx.AssertContains(t, options, "claude-3-5-sonnet")
}
```

> 注意：上面的 helper 方法（`CreateSessionWithModels`、`ListSessions`、`ListSessionOptions` 等）需根据项目实际 `test/e2e/helper` 包调整。如果 helper 不存在，需要在本任务中先扩展 helper 或直接在测试里调用底层 API。

- [ ] **Step 2: 跑 E2E 测试**

Run: `go test -v -count=1 ./test/e2e/session_list_filter_model/`

Expected: 全部 PASS。

- [ ] **Step 3: Commit**

```bash
git add test/e2e/session_list_filter_model/session_list_filter_model_test.go
git commit -m "test(e2e): add session list model filter tests"
```

---

## Task 7: 全量回归验证

**Files:**
- 视修复情况而定

- [ ] **Step 1: 跑 lint**

Run: `make lint`

Expected: PASS。如果有新错误，修复后再跑。

- [ ] **Step 2: 跑全量测试**

Run: `make test`

Expected: PASS。如果失败，定位并修复。

- [ ] **Step 3: Commit（如有修复）**

```bash
git add -A
git commit -m "fix: address lint/test regressions for session model filter"
```

---

## Plan Self-Review

**Spec coverage:**
- filter JSONB 数组支持：Task 1
- session model 字段注册：Task 2
- model 选项接口：Task 3
- DTO/Router 文档：Task 4
- Web 前端筛选：Task 5
- E2E 测试：Task 6
- 回归验证：Task 7

**Placeholder scan:** 无 TBD/TODO/"later"/"similar to"。

**Type consistency:**
- `SessionOptionListReq` 在 Go DTO 中保持 `string`，doc 说明枚举；TS 类型使用 `"score" | "model"` 联合类型。
- `ListDistinctModels` 签名在接口、实现、fake repo 中一致。
- `buildSessionFilter` 参数顺序与所有调用点一致：`(scores, models)`。
