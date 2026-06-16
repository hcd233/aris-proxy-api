# Session 列表按模型筛选设计

## 背景

Web 端 Sessions 列表目前已经展示每个 session 使用过的模型列表（`models` 列），但筛选区只支持按 `score` 筛选。用户希望增加按模型筛选的能力，以便快速定位使用特定模型的会话。

Audit 列表已经具备完整的 model 筛选能力（下拉多选 + 精确匹配模型名），本次设计参考 Audit 的交互模式，同时解决 session `models` 字段为 JSONB 数组带来的筛选差异。

## 目标

1. 后端 `/api/v1/session/list` 的 `filter` 参数支持 `model:xxx|yyy` 表达式。
2. 后端 `/api/v1/session/option/list` 支持 `field=model`，返回可选模型列表。
3. Web 端 Sessions 页面新增 Model 下拉多选筛选器，与 Score 筛选器组合使用。

## 关键决策

- **匹配语义**：精确匹配。用户从下拉选项选择模型名，筛选 `models` JSONB 数组包含所选模型名的 session。
  - 理由：模型名是完整标识符（如 `gpt-4o`、`claude-3-5-sonnet-20241022`），模糊匹配容易误筛；且与 Audit 的下拉多选交互保持一致。
- **多选语义**：OR。`model:gpt-4o|claude-3-5-sonnet` 表示包含任一模型。
- **实现位置**：扩展通用 `internal/common/filter` 包，增加对 JSONB 数组字段的筛选支持。
  - 理由：避免改动 `SessionReadRepository` 接口和多处 repository 方法签名，改动面最小；后续其他 JSONB 数组字段可复用。

## 数据模型

`models` 列已在 `internal/infrastructure/database/model/session.go` 中定义：

```go
Models []string `json:"models" gorm:"column:models;comment:回答模型列表;serializer:json"`
```

在 PostgreSQL 中通过 GORM `serializer:json` 存储为 JSONB 文本，查询时需要 `::jsonb` 强转后使用 JSONB 操作符。

## 筛选表达式与 SQL 映射

| 表达式 | 生成 SQL（示意） | 语义 |
|---|---|---|
| `model:gpt-4o` | `models::jsonb @> jsonb_build_array(?)` | models 数组包含 `gpt-4o` |
| `model:gpt-4o\|claude-3-5-sonnet` | `(models::jsonb @> jsonb_build_array(?) OR models::jsonb @> jsonb_build_array(?))` | 包含任一模型 |
| `model:!gpt-4o` | `NOT models::jsonb @> jsonb_build_array(?)` | 不包含该模型 |
| `model:!gpt-4o\|claude-3-5-sonnet` | `(NOT models::jsonb @> jsonb_build_array(?) AND NOT models::jsonb @> jsonb_build_array(?))` | 不包含任一模型 |
| `score:5 model:gpt-4o` | `score = 5 AND models::jsonb @> jsonb_build_array(?)` | score 与 model 组合 |

> 不支持 `> / < / >= / <=` 比较操作符。

## 后端实现

### 1. 扩展通用 filter 包

文件：`internal/common/filter/parser.go`

- `FieldConfig` 增加 `IsJSONBArray bool` 字段。
- `buildCondition` 中在 `ValueMap` / `IsFuzzy` / `IsNumeric` 之前新增分支：
  ```go
  if config.IsJSONBArray {
      return buildJSONBArrayCondition(column, f)
  }
  ```
- 新增 `buildJSONBArrayCondition(column string, f Filter) (sql string, args []any, err error)`：
  - 单值 equal：`column::jsonb @> jsonb_build_array(?)`
  - 单值 not equal：`NOT column::jsonb @> jsonb_build_array(?)`
  - 多值 equal：`(column::jsonb @> jsonb_build_array(?) OR ...)`
  - 多值 not equal：`(NOT column::jsonb @> jsonb_build_array(?) AND ...)`
  - 比较操作符（`> / < / >= / <=`）返回 `ErrUnsupportedOp`。

使用 `jsonb_build_array(?)` 而非直接拼接 JSON 字符串，可自然复用 GORM 参数化，避免 SQL 注入。

### 2. session filter 字段配置

文件：`internal/application/session/query/jwt_session_queries.go`

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

### 3. 常量

文件：`internal/common/constant/sql.go`

新增：

```go
SessionFilterFieldModel       = "model"
SessionFilterModelSQLColumn   = "models"
SessionDistinctModelSelect    = "DISTINCT jsonb_array_elements_text(models::jsonb) AS model"
SessionDistinctModelWhere     = "models IS NOT NULL AND models::jsonb <> '[]'::jsonb"
SessionDistinctModelOrder     = "model ASC"
SessionDistinctModelLimit     = 50
```

### 4. session 选项接口支持 model

文件：`internal/application/session/query/option_list.go`

```go
switch q.Field {
case constant.FieldScore:
    // 现有逻辑
case constant.SessionFilterFieldModel:
    return h.readRepo.ListDistinctModels(ctx, q.Keyword, q.StartTime, q.EndTime)
default:
    return []string{}, nil
}
```

### 5. 仓储层新增 ListDistinctModels

文件：`internal/domain/session/repository.go`

`SessionReadRepository` 接口新增：

```go
// ListDistinctModels 查询去重的模型列表（支持时间范围与关键字过滤）
ListDistinctModels(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
```

文件：`internal/infrastructure/repository/session_repository.go`

实现：

```go
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

> 权限说明：与现有 `ListDistinctScores` 保持一致，不区分用户/管理员，返回全量可见模型。如需按用户过滤，应作为独立需求后续统一改造。

### 6. DTO / Router 更新

- `internal/dto/session.go`：`SessionOptionListReq.Field` 的 doc 更新为 `"score" | "model"`（类型保持 `string`）。
- `internal/router/session.go`：`listSessionOptions` 的 Description 更新为 `"Get available options for session filter fields (score, model)"`。

## 前端实现

### 1. 类型

文件：`web/src/lib/types.ts`

```ts
export interface SessionOptionListReq {
  field: "score" | "model";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}
```

### 2. Sessions 页面

文件：`web/src/app/(dashboard)/sessions/page.tsx`

新增 state：

```ts
const [filterModel, setFilterModel] = useState<string[]>([]);
const [modelOptions, setModelOptions] = useState<string[]>([]);
```

新增获取选项回调（与 `fetchScoreOptions` 对称）：

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

在 time range 变化时拉取 model options（与 score 选项相同逻辑）。

修改 `buildSessionFilter`：

```ts
const buildSessionFilter = (scores: string[], models: string[]): string | undefined => {
  const parts: string[] = [];
  if (scores.length > 0) parts.push(`score:${scores.join("|")}`);
  if (models.length > 0) parts.push(`model:${models.join("|")}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
};
```

在筛选区 Score `MultiSelectPill` 旁新增 Model `MultiSelectPill`：

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

更新 `fetchSessions` 签名，所有调用点传入 `filterModel`。

更新 Clear filters 逻辑：同时清空 `filterScore` 和 `filterModel`。

## 测试

### 单元测试

1. `test/unit/filter/filter_test.go`（新建）
   - `model:gpt-4o` -> 单值 equal SQL
   - `model:gpt-4o|claude-3-5-sonnet` -> 多值 OR SQL
   - `model:!gpt-4o` -> 单值 NOT SQL
   - `model:!gpt-4o|claude-3-5-sonnet` -> 多值 NOT AND SQL
   - `model:>gpt-4o` -> 返回 unsupported operator 错误
   - 组合 `score:5 model:gpt-4o` -> AND 连接

2. `test/unit/session_option_list/session_option_list_test.go`（新建或扩展）
   - `field=model` 分发到 `ListDistinctModels`
   - `field=score` 保持现有行为
   - 未知 field 返回空数组

### E2E 测试

`test/e2e/session_list_filter_model/session_list_filter_model_test.go`（新建）

- 准备数据：创建 3 个 session，分别包含 models `['gpt-4o']`、`['claude-3-5-sonnet']`、`['gpt-4o', 'claude-3-5-sonnet']`。
- 调用 `/api/v1/session/list?filter=model:gpt-4o`，断言返回 2 条。
- 调用 `/api/v1/session/list?filter=model:gpt-4o|claude-3-5-sonnet`，断言返回 3 条。
- 调用 `/api/v1/session/list?filter=model:!gpt-4o`，断言返回 1 条。
- 调用 `/api/v1/session/option/list?field=model`，断言返回 2 个模型选项。

### Web 验证

- `cd web && npm run lint && npm run build` 通过。
- 本地启动后端与前端，验证 Model 下拉选项、筛选结果、Clear filters 行为正确。

## 性能与索引（可选）

如果 `sessions` 表数据量较大，`models::jsonb @> jsonb_build_array(...)` 需要 GIN 索引才能高效执行：

```sql
CREATE INDEX IF NOT EXISTS idx_sessions_models_gin
  ON sessions USING GIN ((models::jsonb));
```

项目使用 GORM AutoMigrate，可通过在 `models` 字段 tag 增加 gin 索引或 AutoMigrate 后手动执行。是否在本次加索引取决于线上数据量，建议作为独立性能优化项评估。

## 依赖与风险

- 依赖 `2026-06-16-session-list-models-design.md` 已完成的 `models` 列写入与回填。如果存量 session 的 `models` 列仍为 NULL/空数组，则筛选结果不包含这些 session（符合预期）。
- 扩展 `internal/common/filter` 包时，必须确保不影响 audit 现有的 `model` 字段（其 `IsFuzzy=true`，不设置 `IsJSONBArray`）。
