# Session 列表按消息数区间筛选设计

## 背景

Web 端 Sessions 列表已有 score / model 下拉多选筛选器（`MultiSelectPill` + `filter` 表达式），用户希望新增按消息数（message count）筛选的能力，快速定位消息量多/少的会话。

## 目标

1. 后端 `/api/v1/session/option/list` 新增 `field=messageCount`：返回当前时间范围（`startTime`/`endTime`）的消息数区间桶列表。
2. 后端 `/api/v1/session/list` 的 `filter` 参数支持 `messageCount:0-10|11-50` 区间表达式。
3. Web 端 Sessions 页面新增"消息数"多选筛选器，与 score / model 组合使用。

## 关键决策

- **区间划分：固定边界 + 动态上限**（用户确认）。
  固定边界点 10 / 50 / 100 / 200 / 500，最后一个桶的上限由当前时间范围最大会话消息数 `max` 动态截断：
  - max=82 → `0-10`、`11-50`、`51-82`
  - max=1200 → `0-10`、`11-50`、`51-100`、`101-200`、`201-500`、`501-1200`
  - 只返回有会话的桶（count>0）；时间范围无会话（max=0）→ 空列表
  - 理由：档位跨时间范围稳定可预期；动态上限保证最后一个区间始终有数据，不产生超出数据的空档。
- **多选语义：OR**。`messageCount:0-10|11-50` 表示消息数落在任一选中区间。
- **实现位置：扩展通用 `internal/common/filter` 包**。
  - `FieldConfig` 新增 `SQLExpr`（优先于 `SQLColumn` 的表达式片段，用于非物化计算列）与 `IsRange`（值格式 `min-max`，生成 BETWEEN 条件）。
  - 理由：与 2026-06-16 model filter（`IsJSONBArray`）同一扩展模式；`message_count` 不是真实列而是 `jsonb_array_length(message_ids::jsonb)` 表达式，复用 `SQLExpr` 后无需改 repository 的 list 方法签名。

## 筛选表达式与 SQL 映射

| 表达式 | 生成 SQL（示意） | 语义 |
|---|---|---|
| `messageCount:0-10` | `(jsonb_array_length(message_ids::jsonb) >= 0 AND ... <= 10)` | 消息数在 [0,10] |
| `messageCount:0-10\|11-50` | `((...) OR (...))` | 落在任一区间 |
| `messageCount:!0-10` | `NOT (...)` | 不在该区间 |
| `messageCount:10-5` / `abc` | 400（非法区间） | 校验 min/max 整数且 min<=max |

> 不支持 `> / < / >= / <=` 比较操作符（与 JSONB 数组字段一致）。

## 后端实现

### 1. 扩展通用 filter 包（`internal/common/filter/parser.go`）

- `FieldConfig` 新增 `SQLExpr string`、`IsRange bool`。
- `buildCondition`：`SQLExpr` 优先于 `SQLColumn`；`IsRange` 走新增 `buildRangeCondition`：
  - 单值 equal：`(expr >= ? AND expr <= ?)`
  - 多值 equal：OR 连接；多值 not equal：AND 连接 NOT
  - `parseRangeValue` 校验 "min-max"（非负整数、min<=max），非法返回 400（`FilterErrInvalidRange`）

### 2. session filter 字段配置（`jwt_session_queries.go`）

```go
constant.SessionFilterFieldMessageCount: {
    SQLExpr: constant.SessionMessageCountSQLExpr, // "jsonb_array_length(message_ids::jsonb)"
    IsRange: true,
},
```

### 3. 仓储层新增 ListMessageCountStats（`session_repository.go`）

```go
ListMessageCountStats(ctx, startTime, endTime) (maxCount int, bucketCounts map[int]int64, err error)
```

两次查询（均带 `created_at` 时间范围 + `deleted_at = 0`）：
- max：`COALESCE(MAX(jsonb_array_length(message_ids::jsonb)), 0)`
- 桶计数：`CASE WHEN cnt<=10 THEN 0 ... ELSE 5 END AS bucket_idx, COUNT(*)` 分组

### 4. 桶生成纯函数（`query/message_count_buckets.go`）

```go
BuildMessageCountBuckets(maxCount int, bucketCounts map[int]int64) []string
```

- 定位最后一个有意义的桶（maxCount 落在哪个固定边界内）
- 末桶上限 = maxCount；只保留 count>0 的桶；maxCount<=0 返回 nil
- 导出以便 `test/unit/` 直接测试。

### 5. options 接口（`option_list.go` + `dto/option.go`）

`SessionOptionListReq.Field` enum 增加 `messageCount`；`Handle` 增加对应 case，调用 `ListMessageCountStats` + `BuildMessageCountBuckets`。

## 前端实现（`web/src/app/(dashboard)/sessions/page.tsx`）

- 完全镜像现有 score 过滤器：`filterMessageCount`（persisted）+ `messageCountOptions` state、`fetchMessageCountOptions`、时间范围变化时重拉 options 的 `useEffect` 注册。
- `buildSessionFilter` 增加 `messageCount:...` 段；`fetchSessions` 参数与全部 11 个调用点透传。
- 新增 `MultiSelectPill`；clear 按钮一并重置；i18n 三语新增 `sessions.filter_message_count`。

## 测试

- `test/unit/filter/filter_test.go`：range 单值/多值 SQL、非法值、不支持操作符。
- `test/unit/session_message_count/`：`BuildMessageCountBuckets` 边界（max=82/8/1200/501/0、空桶过滤）。
- `test/unit/session_option_list/`：messageCount 分发（fake repo 补新方法）。
- `test/e2e/session_list_filter_message_count/`：option/list 格式合法、filter 不 500、非法值 400、过滤语义（返回会话落在选中区间并集内）。
