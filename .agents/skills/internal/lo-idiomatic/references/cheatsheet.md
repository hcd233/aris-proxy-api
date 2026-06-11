# samber/lo v1.39 速查手册

按类别列出常用函数签名和简短示例。完整 API 见 [官方文档](https://pkg.go.dev/github.com/samber/lo)。

## 切片 (Slice)

| 函数 | 签名 | 说明 |
|------|------|------|
| `Map` | `Map[T, R any]([]T, func(T, int) R) []R` | 映射，回调接收元素和索引 |
| `Filter` | `Filter[T any]([]T, func(T, int) bool) []T` | 过滤 |
| `FilterMap` | `FilterMap[T, R any]([]T, func(T, int) (R, bool)) []R` | 映射+过滤合一 |
| `FlatMap` | `FlatMap[T, R any]([]T, func(T, int) []R) []R` | 映射后展平 |
| `Flatten` | `Flatten[T any]([][]T) []T` | 展平一层 |
| `Uniq` | `Uniq[T comparable]([]T) []T` | 去重，保持首次出现顺序 |
| `UniqBy` | `UniqBy[T any, U comparable]([]T, func(T) U) []T` | 按字段去重 |
| `Contains` | `Contains[T comparable]([]T, T) bool` | 是否包含 |
| `ContainsBy` | `ContainsBy[T any]([]T, func(T) bool) bool` | 按条件判断 |
| `Find` | `Find[T any]([]T, func(T) bool) (T, bool)` | 查找第一个满足条件的元素 |
| `FindOrElse` | `FindOrElse[T any]([]T, T, func(T) bool) T` | 查找，未找到返回默认值 |
| `IndexOf` | `IndexOf[T comparable]([]T, T) int` | 查找索引 |
| `LastIndexOf` | `LastIndexOf[T comparable]([]T, T) int` | 最后出现的索引 |
| `Some` | `Some[T any]([]T, func(T, int) bool) bool` | 至少一个满足 |
| `Every` | `Every[T any]([]T, func(T, int) bool) bool` | 全部满足 |
| `EveryBy` | `EveryBy[T any]([]T, func(T) bool) bool` | 全部满足（无索引版） |
| `None` | `None[T any]([]T, func(T, int) bool) bool` | 全部不满足 |
| `GroupBy` | `GroupBy[T any, U comparable]([]T, func(T) U) map[U][]T` | 按键分组 |
| `Chunk` | `Chunk[T any]([]T, int) [][]T` | 分块 |
| `Partition` | `Partition[T any]([]T, func(T, int) bool) ([]T, []T)` | 二分区分 |
| `Reverse` | `Reverse[T any]([]T) []T` | 反转（返回新切片） |
| `Shuffle` | `Shuffle[T any]([]T) []T` | 随机打乱 |
| `Sample` | `Sample[T any]([]T) T` | 随机取一个 |
| `Samples` | `Samples[T any]([]T, int) []T` | 随机取 N 个 |
| `Compact` | `Compact[T comparable]([]T) []T` | 去除零值元素 |
| `Repeat` | `Repeat[T any](int, T) []T` | 重复 N 次 |
| `Difference` | `Difference[T comparable]([]T, []T) ([]T, []T)` | 差集 (A-B, B-A) |
| `Intersect` | `Intersect[T comparable]([]T, ...[]T) []T` | 交集 |
| `Union` | `Union[T comparable]([]T, ...[]T) []T` | 并集（去重） |
| `KeyBy` | `KeyBy[T any, U comparable]([]T, func(T) U) map[U]T` | 切片转 map（去重保留最后一个） |
| `OrderBy` | `OrderBy[T any]([]T, []string, []string)` | 排序（字段名 + "asc"/"desc"） |
| `SortBy` | `SortBy[T any]([]T, func(T) float64) []T` | 按数值排序 |
| `Reject` | `Reject[T any]([]T, func(T, int) bool) []T` | Filter 的反面 |
| `Count` | `Count[T comparable]([]T, T) int` | 计数 |
| `CountBy` | `CountBy[T any]([]T, func(T) bool) int` | 按条件计数 |
| `CountValues` | `CountValues[T comparable]([]T) map[T]int` | 每个值的出现次数 |
| `DistinctBy` | `DistinctBy[T any, U comparable]([]T, func(T) U) []T` | UniqBy 的别名 |

## Map

| 函数 | 签名 | 说明 |
|------|------|------|
| `Keys` | `Keys[K comparable, V any](map[K]V) []K` | 所有键 |
| `Values` | `Values[K comparable, V any](map[K]V) []V` | 所有值 |
| `Entries` | `Entries[K comparable, V any](map[K]V) []Tuple[K, V]` | 键值对切片 |
| `FromEntries` | `FromEntries[K comparable, V any]([]Tuple[K, V]) map[K]V` | 键值对切片转 map |
| `SliceToMap` | `SliceToMap[T any, K comparable, V any]([]T, func(T) (K, V)) map[K]V` | 切片转 map |
| `MapToSlice` | `MapToSlice[K comparable, V any, R any](map[K]V, func(K, V) R) []R` | map 转切片 |
| `Assign` | `Assign[K comparable, V any](map[K]V, ...map[K]V) map[K]V` | 合并 map（后者覆盖） |
| `MapValues` | `MapValues[K comparable, V any, R any](map[K]V, func(V, K) R) map[K]R` | 值映射 |
| `OmitBy` | `OmitBy[K comparable, V any](map[K]V, func(K, V) bool) map[K]V` | 按条件排除 |
| `PickBy` | `PickBy[K comparable, V any](map[K]V, func(K, V) bool) map[K]V` | 按条件选取 |
| `Invert` | `Invert[K comparable, V comparable](map[K]V) map[V]K` | 键值互换 |

## 指针

| 函数 | 签名 | 说明 |
|------|------|------|
| `ToPtr` | `ToPtr[T any](T) *T` | 取指针 |
| `FromPtr` | `FromPtr[T any](*T) T` | 解引用，nil 返回零值 |
| `FromPtrOr` | `FromPtrOr[T any](*T, T) T` | 解引用，nil 返回默认值 |
| `Coalesce` | `Coalesce[T any](...T) T` | 第一个非零值 |
| `CoalesceOrEmpty` | `CoalesceOrEmpty[T any](...*T) *T` | 第一个非 nil 指针 |

## 错误处理 / Must

| 函数 | 签名 | 说明 |
|------|------|------|
| `Must` | `Must[T any](T, error) T` | 错误则 panic |
| `Must0` | `Must0(...any)` | 仅检查 error |
| `Must1` | `Must1[T any](T, error) T` | 同 Must |
| `Must2` | `Must2[T1, T2 any](T1, T2, error) (T1, T2)` | 保留两个返回值 |
| `Must3` | `Must3[T1, T2, T3 any](T1, T2, T3, error) (T1, T2, T3)` | 保留三个返回值 |
| `Must4` | `Must4[T1, T2, T3, T4 any](T1, T2, T3, T4, error) (T1, T2, T3, T4)` | 保留四个返回值 |
| `Try` | `Try(func() bool) bool` | 捕获 panic |
| `Try0` | `Try0(func()) bool` | 捕获 panic |
| `Try1` | `Try1[T any](func() T) (T, bool)` | 捕获 panic |
| `TryWithRecover` | `TryWithRecover(func()) bool` | 捕获 panic 并调用 recover 回调 |
| `Attempt` | `Attempt(int, func(int) error) (int, error)` | 重试 |
| `AttemptWithDelay` | `AttemptWithDelay(int, time.Duration, func(int, time.Duration) error) (int, time.Duration, error)` | 带延迟重试 |

## 条件 / 控制流

| 函数 | 签名 | 说明 |
|------|------|------|
| `Ternary` | `Ternary[T any](bool, T, T) T` | 三元表达式 |
| `TernaryF` | `TernaryF[T any](bool, func() T, func() T) T` | 延迟求值三元 |
| `If` | `If[T any](bool, T) *Condition[T]` | 链式条件 |
| `IfF` | `IfF[T any](bool, func() T) *Condition[T]` | 延迟求值链式 |
| `Switch` | `Switch[T comparable, R any](T) *SwitchStmt[T, R]` | 链式 switch |

## 数学

| 函数 | 签名 | 说明 |
|------|------|------|
| `Clamp` | `Clamp[T constraints.Ordered](T, T, T) T` | 限制在 [min, max] |
| `Min` | `Min[T constraints.Ordered](...T) T` | 最小值 |
| `Max` | `Max[T constraints.Ordered](...T) T` | 最大值 |
| `Sum` | `Sum[T constraints.Float \| constraints.Integer \| constraints.Complex]([]T) T` | 求和 |
| `Mean` | `Mean[T constraints.Float \| constraints.Integer \| constraints.Complex]([]T) float64` | 均值 |

## 并发

| 函数 | 签名 | 说明 |
|------|------|------|
| `Async` | `Async[T any](func() T) <-chan T` | 异步执行 |
| `Transaction` | `Transaction[T any](...func() (T, error)) ([]T, error)` | 顺序执行，遇错停 |
| `WaitFor` | `WaitFor(func() bool, time.Duration) bool` | 轮询等待条件 |

## Channel

| 函数 | 说明 |
|------|------|
| `ChannelDispatcher` | 将输入分发到 N 个 worker channel |
| `FanIn` | 多 channel 合并为一个 |
| `FanOut` | 一个 channel 扇出到 N 个 |
| `Generator` | 生成器模式 |

## 时间

| 函数 | 说明 |
|------|------|
| `Duration` | 返回函数执行耗时 |
| `Debounce` | 防抖 |
| `DebounceBy` | 按键防抖 |

## 杂项

| 函数 | 说明 |
|------|------|
| `IsEmpty` | 判断是否为零值 |
| `IsNotNil` | 判断指针是否非 nil |
| `IsNil` | 判断是否 nil |
| `Range` | 生成数值范围切片 |
| `RangeFrom` | 从起始值生成范围 |
| `RangeWithSteps` | 按步长生成范围 |
| `Substring` | 安全子串（不 panic） |
| `ChunkString` | 字符串分块 |
| `RandomString` | 随机字符串 |
| `ShuffleString` | 随机打乱字符串 |
