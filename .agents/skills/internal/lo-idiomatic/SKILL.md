---
name: lo-idiomatic
description: |
  使用 samber/lo 泛型工具库简化 Go 代码风格，替代手写 for 循环、if 判断和样板代码。
  在编写或审查 Go 代码时自动加载此 skill，识别可以用 lo 简化的冗长模式，并给出惯用写法。
  当用户进行 Go 开发、编写或审查 Go 代码、重构冗长的切片/Map/指针/错误处理逻辑时，
  或提到 "lo"、"samber/lo"、"简化代码"、"Go 泛型工具" 时触发。
  即使用户没有显式提到 lo，只要你在写 Go 代码且发现有冗长的 for+if 模式，就应该主动应用此 skill。
---

# lo-idiomatic: 用 samber/lo 写出惯用 Go 代码

## 核心理念

Go 缺乏泛型切片/Map 操作的标准库，导致大量重复的 for+if 样板代码。`samber/lo` 用 Go 1.18+ 泛型填补了这一空白，提供 200+ 类型安全的工具函数。

**原则**：当一段代码在做"遍历 + 过滤/映射/查找/去重"时，优先用 `lo` 的一行调用替代手写循环。但不要为了用 `lo` 而把简单的单步操作包装成多层链式调用——可读性始终优先。

## 何时用 lo，何时不用

| 场景 | 用 lo | 不用 lo |
|------|-------|---------|
| 切片映射/过滤 | `lo.Map`, `lo.Filter` | — |
| 查找/判断 | `lo.Contains`, `lo.Find`, `lo.Some` | — |
| 去重/集合运算 | `lo.Uniq`, `lo.Difference` | — |
| Slice ↔ Map 转换 | `lo.SliceToMap`, `lo.MapToSlice` | — |
| 指针零值处理 | `lo.FromPtr`, `lo.ToPtr`, `lo.Coalesce` | — |
| 初始化必须成功 | `lo.Must`, `lo.Must0`/`1`/`2` | — |
| 简单的单元素访问 | — | 直接 `s[i]` |
| 需要中断/break 的复杂循环 | — | 手写 for + break |
| 性能热点（百万级元素） | — | 手写 for 避免闭包开销 |
| 只需遍历做副作用 | — | 手写 for range |

## 高频模式速查

下面按场景列出"你可能会写的冗长代码"→"lo 惯用写法"。完整函数列表见 `references/cheatsheet.md`。

### 1. 切片映射：for 循环 → lo.Map

```go
// ❌ 冗长
names := make([]string, len(users))
for i, u := range users {
    names[i] = u.Name
}

// ✅ 惯用
names := lo.Map(users, func(u User, _ int) string { return u.Name })
```

注意回调签名是 `(T, int) R`，第二个参数是索引。不需要索引时用 `_`。

### 2. 切片过滤：for+append → lo.Filter

```go
// ❌ 冗长
var admins []User
for _, u := range users {
    if u.Role == "admin" {
        admins = append(admins, u)
    }
}

// ✅ 惯用
admins := lo.Filter(users, func(u User, _ int) bool { return u.Role == "admin" })
```

### 3. 映射+过滤合一 → lo.FilterMap

```go
// ❌ 两步
active := lo.Filter(users, func(u User, _ int) bool { return u.Active })
emails := lo.Map(active, func(u User, _ int) string { return u.Email })

// ✅ 一步
emails := lo.FilterMap(users, func(u User, _ int) (string, bool) {
    return u.Email, u.Active
})
```

### 4. Slice → Map：for 循环 → lo.SliceToMap

```go
// ❌ 冗长
userMap := make(map[uint]*User)
for _, u := range users {
    userMap[u.ID] = &u
}

// ✅ 惯用
userMap := lo.SliceToMap(users, func(u User) (uint, *User) { return u.ID, &u })
```

### 5. Map → Slice：for 循环 → lo.MapToSlice / lo.Keys / lo.Values

```go
// ❌ 冗长
ids := make([]uint, 0, len(userMap))
for id := range userMap {
    ids = append(ids, id)
}

// ✅ 惯用
ids := lo.Keys(userMap)
users := lo.Values(userMap)
```

### 6. 去重：手写 seen map → lo.Uniq

```go
// ❌ 冗长
seen := make(map[string]struct{})
var uniq []string
for _, s := range items {
    if _, ok := seen[s]; !ok {
        seen[s] = struct{}{}
        uniq = append(uniq, s)
    }
}

// ✅ 惯用
uniq := lo.Uniq(items)
```

按字段去重用 `lo.UniqBy`。

### 7. 集合运算：手写遍历 → lo.Difference / lo.Intersect

```go
// ❌ 冗长：求两个切片的差集需要手写双重循环或 map

// ✅ 惯用
onlyInA, onlyInB := lo.Difference(a, b)  // A-B, B-A
common := lo.Intersect(a, b)              // A∩B
```

### 8. 查找/判断：手写循环 → lo.Contains / lo.Find / lo.Some / lo.Every

```go
// ❌ 冗长
found := false
for _, s := range items {
    if s == target {
        found = true
        break
    }
}

// ✅ 惯用
found := lo.Contains(items, target)

// 按条件查找
user, ok := lo.Find(users, func(u User) bool { return u.ID == id })

// 至少一个满足
hasAdmin := lo.Some(users, func(u User, _ int) bool { return u.Role == "admin" })

// 全部满足
allActive := lo.EveryBy(users, func(u User) bool { return u.Active })
```

### 9. 指针零值处理：if nil → lo.FromPtr / lo.ToPtr / lo.Coalesce

```go
// ❌ 冗长
var name string
if req.Name != nil {
    name = *req.Name
}

// ✅ 惯用
name := lo.FromPtr(req.Name)

// 反向：取指针
model := lo.ToPtr("gpt-4o")

// 链式回退：第一个非零值
val := lo.Coalesce(ptr1, ptr2, lo.ToPtr("default"))
```

### 10. 初始化必须成功：if err != nil { panic } → lo.Must

```go
// ❌ 冗长
db, err := gorm.Open(dialector, &gorm.Config{})
if err != nil {
    panic(err)
}

// ✅ 惯用
db := lo.Must(gorm.Open(dialector, &gorm.Config{}))

// 多返回值
sqlDB := lo.Must(db.DB())

// 指定保留前 N 个返回值
lo.Must0(c.Start())          // 忽略所有返回值，只检查 error
val := lo.Must1(fn())        // 保留第 1 个返回值
a, b := lo.Must2(fn())      // 保留前 2 个返回值
```

`Must` 系列适用于**启动阶段**（初始化 DB、Redis、MinIO 等），业务逻辑中不应使用——业务错误应走 `ierr` 体系。

### 11. 分组：手写 map[slice] → lo.GroupBy

```go
// ❌ 冗长
groups := make(map[string][]User)
for _, u := range users {
    groups[u.Dept] = append(groups[u.Dept], u)
}

// ✅ 惯用
groups := lo.GroupBy(users, func(u User) string { return u.Dept })
```

### 12. 展平嵌套：手写双重循环 → lo.Flatten

```go
// ❌ 冗长
var all []uint
for _, s := range sessions {
    all = append(all, s.MessageIDs...)
}

// ✅ 惯用
all := lo.Flatten(lo.Map(sessions, func(s Session, _ int) []uint { return s.MessageIDs }))
```

### 13. Map 合并：手写 for range → lo.Assign

```go
// ❌ 冗长
merged := make(map[string]Field)
for k, v := range defaults {
    merged[k] = v
}
for k, v := range overrides {
    merged[k] = v
}

// ✅ 惯用
merged := lo.Assign(defaults, overrides)  // 后者覆盖前者
```

### 14. 条件赋值：if-else → lo.Ternary / lo.If

```go
// ❌ 冗长
var limit int
if isVip {
    limit = 100
} else {
    limit = 10
}

// ✅ 惯用（简单二选一）
limit := lo.Ternary(isVip, 100, 10)

// 延迟求值版本（避免不必要的计算）
limit := lo.TernaryF(isVip, func() int { return expensiveCalc() }, func() int { return 10 })

// 多分支链式（比 switch 更函数式）
result := lo.If(cond1, val1).
    ElseIf(cond2, val2).
    Else(defaultVal)
```

### 15. 分区：两次 Filter → lo.Partition

```go
// ❌ 两次遍历
active := lo.Filter(users, func(u User, _ int) bool { return u.Active })
inactive := lo.Filter(users, func(u User, _ int) bool { return !u.Active })

// ✅ 一次遍历
active, inactive := lo.Partition(users, func(u User, _ int) bool { return u.Active })
```

## 链式调用指南

`lo` 函数可以链式组合，但注意可读性：

```go
// ✅ 合理链式：2-3 层，逻辑清晰
orphanIDs, _ := lo.Difference(
    lo.Uniq(lo.Flatten(lo.Map(softDeleted, func(s View, _ int) []uint { return s.IDs }))),
    lo.Uniq(lo.Flatten(lo.Map(active, func(s View, _ int) []uint { return s.IDs }))),
)

// ❌ 过度链式：5+ 层嵌套，难以阅读
// 拆成带命名变量的中间步骤
```

**经验法则**：链式超过 3 层时，提取中间变量并命名。

## 与项目约定的集成

- 本项目 `go.mod` 已依赖 `github.com/samber/lo v1.39.0`，无需额外安装。
- 业务错误处理仍使用 `ierr` 体系（`ierr.Wrap`/`ierr.New`），`lo.Must` 仅限启动阶段。
- DTO 层使用 `lo.FromPtr`/`lo.ToPtr` 处理可空字段时，注意不违反"DTO 禁止导入 dbmodel"的约束。
- `lo.Must0`/`lo.Must1`/`lo.Must2` 在本项目中广泛用于中间件中写错误响应后 panic 中断请求链。

## 进阶参考

完整的函数列表和签名参考 `references/cheatsheet.md`。
