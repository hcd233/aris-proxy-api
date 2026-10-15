# session/list keyword 搜索性能修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `GET /api/web/v1/session/list?keyword=...` 高频词 10s~60s+ 的慢查询（correlated SubPlan 根因），保持「中文/英文任意子串匹配、只搜用户消息」语义不变。

**Architecture:** 两段式查询——第一段用 user 消息部分 trigram 索引把 keyword 解析成消息 id 集合（带 60s 读缓存），第二段以 `= ANY(?::bigint[])` 常量数组过滤 sessions。ids 变成常量参数后，`EXISTS(SRF + IN 子查询)` 的 correlated SubPlan 从结构上不可能再产生。

**Tech Stack:** Go + gorm（`gorm.io/driver/postgres`）+ PostgreSQL 18（`gin_trgm_ops`）+ Redis 读缓存（`internal/infrastructure/cache.ReadCache`）

**Spec:** `docs/superpowers/specs/2026-10-15-session-keyword-search-perf-design.md`

## Global Constraints

- **SQL 占位符约束（fix #58/#59，违反即 SQLSTATE 42601/42809）**：`SessionKeywordFilterSQL` 整段只能有 1 个 `?`；禁止 `ANY(sessions.message_ids)` / `ANY(message_ids)`（message_ids 是 jsonb 文本非原生数组）。
- **SQL 形态约束**：禁止 `JOIN messages ON messages.id =`、禁止 `jsonb_exists(`、禁止 `IN (SELECT id FROM messages`（本次改为 `= ANY(?::bigint[])` 常量数组形态）。
- **分层约束**：`internal/application/**` 禁止 import `internal/infrastructure/cache`（lint 强制）；缓存逻辑只放仓储层。
- **测试基建约束**：单测/E2E 全部基于 sqlite（`test/unit/sqlitetest`），PG 特有语法（`jsonb_array_elements_text`、`::jsonb`、`::bigint[]`）无法执行；SQL 行为正确性由本地 PG 验证清单 + 生产验证覆盖，不引入 PG 测试容器。
- **编码前置**：编辑任何 Go 文件前，先执行 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <file>`（输出不得截断）；编码过程遵守 `golang-naming`、`golang-code-style`、`golang-samber-lo`、`golang-samber-mo` 与 `ponytail`（full）。
- **提交规范**：在 worktree `.worktrees/session-keyword-search-perf`（分支 `bugfix/session-keyword-search-perf-2026-10-15`）内提交；提交前跑 `make lint && make test`（或项目等价命令，见 `docs/agents/commands.md`）。
- **文档语言**：注释与文档中文；错误信息走 `internal/common/ierr`（不裸 `errors.New`）。

## File Structure

| 文件 | 职责 | 动作 |
|---|---|---|
| `internal/common/constant/sql.go` | SQL 片段常量（`SessionKeywordFilterSQL` 改写 + 新增 `SessionKeywordMatchIDsSQL`） | 修改 |
| `internal/common/constant/readcache.go` | 读缓存 TTL 常量（新增 keyword ids 专属 TTL） | 修改 |
| `internal/common/constant/rediskey.go` | Redis key 模板（新增 keyword ids key） | 修改 |
| `internal/infrastructure/cache/read_cache.go` | `GetOrLoadWithTTL` 变体（per-call TTL） | 修改 |
| `internal/infrastructure/repository/session_repository.go` | 两段式 keyword 查询 + 缓存注入 | 修改 |
| `internal/bootstrap/modules/repository.go` | `NewSessionReadRepository` 构造签名加 `*cache.ReadCache` | 修改 |
| `test/unit/session_keyword_filter/session_keyword_filter_test.go` | SQL 形态断言更新 | 修改 |
| `test/unit/cache/read_cache_test.go` | `GetOrLoadWithTTL` 单测 | 修改 |
| `test/unit/session_keyword_filter/keyword_format_test.go` | `formatBigIntArray` / `normalizeKeyword` 纯函数单测 | 新建 |

---

### Task 1: SQL 常量改造 + 形态断言更新

**Files:**
- Modify: `internal/common/constant/sql.go:249-273`（`SessionKeywordFilterSQL` 常量块）
- Test: `test/unit/session_keyword_filter/session_keyword_filter_test.go`

**Interfaces:**
- Consumes: 无（纯常量）
- Produces:
  - `constant.SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint = ANY(?::bigint[]))"`（Task 3 使用）
  - `constant.SessionKeywordMatchIDsSQL = "SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE ?"`（Task 3 使用）

- [ ] **Step 1: 写失败的形态断言（替换两个既有测试函数体）**

将 `TestSessionKeywordFilterSQL_UsesPKJoinShape` 重命名为 `TestSessionKeywordFilterSQL_UsesAnyArrayShape` 并整体替换为：

```go
// TestSessionKeywordFilterSQL_UsesAnyArrayShape 断言 2026-10-15 形态：
// 第二段过滤必须以 = ANY(?::bigint[]) 接收应用层传入的常量 id 数组。
//
// 历史教训：EXISTS(SRF + IN 子查询) 会被 planner 建成 correlated SubPlan，
// messages 的 ILIKE 扫描对外层每行重复执行（生产实测 10s~60s+）；
// IN 子查询形态（2026-08-02）的去相关成败依赖 trigram 统计信息，统计漂移后退化。
// 常量数组形态从结构上不可能产生 correlated SubPlan。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestSessionKeywordFilterSQL_UsesAnyArrayShape(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if fragment == "" {
		t.Fatal("SessionKeywordFilterSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "jsonb_array_elements_text(") {
		t.Errorf("SessionKeywordFilterSQL must expand sessions.questions via jsonb_array_elements_text(), got %q", fragment)
	}
	if !strings.Contains(fragment, "= ANY(?::bigint[])") {
		t.Errorf("SessionKeywordFilterSQL must match ids via = ANY(?::bigint[]) constant array to rule out correlated SubPlan, got %q", fragment)
	}
	if strings.Contains(fragment, "IN (SELECT id FROM messages") {
		t.Errorf("SessionKeywordFilterSQL must not use IN (SELECT id FROM messages ...) — statistics drift turns it into a correlated SubPlan with per-row ILIKE scans; pass ids as a constant array instead, got %q", fragment)
	}
	if strings.Contains(fragment, "JOIN messages ON messages.id =") {
		t.Errorf("SessionKeywordFilterSQL must not JOIN messages (trigram bitmap trap on high-frequency keywords), got %q", fragment)
	}
}

// TestSessionKeywordFilterSQL_AvoidsLandmines 维护历史 bugfix 的护栏：
//
//   - 不能用 ANY(sessions.message_ids)：message_ids 在 PG 里是 jsonb 文本不是原生数组（fix #58）
//   - 整段 SQL 必须只有 1 个 '?'（fix #59：裸 '?' 操作符与 gorm 占位符撞车，SQLSTATE 42601）
//   - 不能出现 jsonb_exists 强相关历史形态（refactor 2026-06-07）
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestSessionKeywordFilterSQL_AvoidsLandmines(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordFilterSQL

	if strings.Contains(fragment, "ANY(sessions.message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use native array ANY(sessions.message_ids), got %q", fragment)
	}
	if strings.Contains(fragment, "ANY(message_ids)") {
		t.Errorf("SessionKeywordFilterSQL must not use ANY against raw message_ids column, got %q", fragment)
	}

	placeholderCount := strings.Count(fragment, "?")
	if placeholderCount != 1 {
		t.Errorf("SessionKeywordFilterSQL must contain exactly one ? placeholder (for the id array), got %d in %q", placeholderCount, fragment)
	}

	if strings.Contains(fragment, "jsonb_exists(") {
		t.Errorf("SessionKeywordFilterSQL must not use historical jsonb_exists() correlated form, got %q", fragment)
	}
}

// TestSessionKeywordMatchIDsSQL_ScopesToUserContent 断言第一段只搜用户消息正文：
// 部分索引 idx_messages_user_content_trgm 以 role='user' 为谓词，查询必须携带相同
// 条件才能命中索引；搜 ->>'content' 而非 message 全文避免 role/tool 元数据误报。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestSessionKeywordMatchIDsSQL_ScopesToUserContent(t *testing.T) {
	t.Parallel()
	fragment := constant.SessionKeywordMatchIDsSQL

	if fragment == "" {
		t.Fatal("SessionKeywordMatchIDsSQL must be defined and non-empty")
	}
	if !strings.Contains(fragment, "message::jsonb->>'role' = 'user'") {
		t.Errorf("SessionKeywordMatchIDsSQL must scope to user messages to hit the partial index, got %q", fragment)
	}
	if !strings.Contains(fragment, "message::jsonb->>'content' ILIKE ?") {
		t.Errorf("SessionKeywordMatchIDsSQL must match message content via ILIKE ? (single placeholder), got %q", fragment)
	}
	if strings.Contains(fragment, "message::text ILIKE") {
		t.Errorf("SessionKeywordMatchIDsSQL must not match raw message JSON text (role/tool metadata false positives), got %q", fragment)
	}
	if strings.Count(fragment, "?") != 1 {
		t.Errorf("SessionKeywordMatchIDsSQL must contain exactly one ? placeholder, got %q", fragment)
	}
}
```

- [ ] **Step 2: 运行断言确认失败**

Run: `rtk go test ./test/unit/session_keyword_filter/... -run 'TestSessionKeyword' -v`
Expected: FAIL（`SessionKeywordFilterSQL` 仍是 IN 子查询形态、`SessionKeywordMatchIDsSQL` 未定义）

- [ ] **Step 3: 改写常量（`internal/common/constant/sql.go:249-273` 整块替换）**

```go
	// SessionKeywordMatchIDsSQL keyword 搜索第一段：关键词 → 用户消息 id 集合。
	//
	// 只搜 role='user' 的消息正文（->>'content'）：
	//   - 与「只搜用户消息」的产品语义一致，搜索面为 11.5K 行/127MB（全表 164K 行/1GB）；
	//   - 与部分索引 idx_messages_user_content_trgm（WHERE role='user'）谓词对齐，才能走索引；
	//   - 不搜 message 全文 JSON，避免 role/tool_call 等元数据造成误报（搜 "user" 命中全部用户消息）。
	SessionKeywordMatchIDsSQL = "SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE ?"

	// SessionKeywordFilterSQL session 列表 keyword 过滤 SQL 片段（第二段）。
	//
	// 设计沿革：
	//   - 2026-06-07 之前：EXISTS + jsonb_exists 强相关，O(sessions × messages) 全表 ILIKE。
	//   - refactor/session-list-keyword-perf-2026-06-07 → JOIN messages 形态：planner 被
	//     trigram 索引误导选 bitmap 回表，高频词（Task:）27.4s 超时。
	//   - bugfix/session-list-keyword-perf-2026-08-02 → IN 子查询形态：当时统计下 planner
	//     以 arr 驱动主键点查（22ms）。但统计漂移后退化为 correlated SubPlan，
	//     messages 的 ILIKE 扫描对每条 session 重复执行（2026-10-15 生产实测 10s~60s+）。
	//   - bugfix/session-keyword-search-perf-2026-10-15（本次）→ = ANY(?::bigint[]) 常量数组：
	//     匹配集由应用层两段式查询（SessionKeywordMatchIDsSQL + 读缓存）传入，
	//     片段内无子查询，correlated SubPlan 从结构上不可能产生。
	//
	// 占位符约束（fix #58/#59）：
	//   - 整段 SQL 只能有 1 个 '?'（gorm 占位符），命中 id 数组以 "{1,2,3}" 字符串传入；
	//   - 不要写 messages.id = ANY(sessions.message_ids)：message_ids 是 jsonb 文本非
	//     原生数组，会触发 SQLSTATE 42809。
	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint = ANY(?::bigint[]))"
```

- [ ] **Step 4: 运行断言确认通过**

Run: `rtk go test ./test/unit/session_keyword_filter/... -v`
Expected: PASS（4 个测试函数全绿）

- [ ] **Step 5: Commit**

```bash
rtk git add internal/common/constant/sql.go test/unit/session_keyword_filter/session_keyword_filter_test.go
rtk git commit -m "refactor(constant): session keyword SQL 改两段式 = ANY 常量数组形态"
```

---

### Task 2: ReadCache per-call TTL 变体

**Files:**
- Modify: `internal/infrastructure/cache/read_cache.go:57-95`（`GetOrLoad`）
- Modify: `internal/common/constant/readcache.go`
- Test: `test/unit/cache/read_cache_test.go`

**Interfaces:**
- Consumes: `cache.ReadCache`、`cache.GetOrLoad`（既有）
- Produces:
  - `cache.GetOrLoadWithTTL[T any](ctx context.Context, c *ReadCache, key string, ttl, jitter time.Duration, loader func(context.Context) (mo.Option[T], error)) (mo.Option[T], error)`（Task 3 使用）
  - `constant.ReadCacheKeywordIDsTTL = 60 * time.Second`、`constant.ReadCacheKeywordIDsTTLJitter = 10 * time.Second`（Task 3 使用）

背景：既有 `GetOrLoad` 的 TTL 固定为全局常量（`ReadCacheTTL` 5min±1min）；keyword 命中集需要更短的 60s±10s（搜索结果新鲜度），故加 per-call TTL 变体，`GetOrLoad` 语义不变（委托默认值）。

- [ ] **Step 1: 写失败的单测（追加到 `test/unit/cache/read_cache_test.go`）**

```go
// TestGetOrLoadWithTTL_UsesCallerTTL 验证 per-call TTL 生效：
// 自定义 TTL 显著小于默认 5min，miniredis 快进后条目必须已过期回源。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestGetOrLoadWithTTL_UsesCallerTTL(t *testing.T) {
	t.Parallel()
	mr, rc := newReadCache(t) // 本文件既有 fixture：newReadCache(t) (*miniredis.Miniredis, *cache.ReadCache)
	const key = "cache:rd:kwids:caller-ttl-test"

	var loads atomic.Int64
	loader := func(_ context.Context) (mo.Option[[]uint], error) {
		loads.Add(1)
		return mo.Some([]uint{1, 2, 3}), nil
	}

	first, err := cache.GetOrLoadWithTTL(t.Context(), rc, key, 30*time.Second, 0, loader)
	if err != nil {
		t.Fatalf("first GetOrLoadWithTTL: %v", err)
	}
	if got := first.OrEmpty(); len(got) != 3 {
		t.Fatalf("first GetOrLoadWithTTL = %v, want [1 2 3]", got)
	}
	if _, err := cache.GetOrLoadWithTTL(t.Context(), rc, key, 30*time.Second, 0, loader); err != nil {
		t.Fatalf("second GetOrLoadWithTTL: %v", err)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader called %d times, want 1 (cache hit must skip loader)", loads.Load())
	}

	mr.FastForward(31 * time.Second) // 快进超过自定义 TTL（默认 TTL 是 5min，不会提前过期）

	if _, err := cache.GetOrLoadWithTTL(t.Context(), rc, key, 30*time.Second, 0, loader); err != nil {
		t.Fatalf("third GetOrLoadWithTTL: %v", err)
	}
	if loads.Load() != 2 {
		t.Fatalf("entry must expire at caller TTL (30s), loader called %d times, want 2", loads.Load())
	}
}
```

（fixture / 包名 / ctx 写法已按 `test/unit/cache/read_cache_test.go` 既有风格定死：外部包 `cache_test`、`newReadCache(t)`、`t.Context()`、`atomic.Int64` 计数。）

- [ ] **Step 2: 运行确认失败**

Run: `rtk go test ./test/unit/cache/... -run TestGetOrLoadWithTTL -v`
Expected: FAIL（`GetOrLoadWithTTL` 未定义，编译错误）

- [ ] **Step 3: 实现变体（`read_cache.go`）**

把既有 `GetOrLoad` 函数体改为委托（公开签名与语义不变）：

```go
// GetOrLoad 读缓存主入口（默认 TTL：constant.ReadCacheTTL ± ReadCacheTTLJitter）
//
// 命中缓存直接返回；未命中经 singleflight 回源后写回。
// loader 返回 mo.None 表示「查无此物」，写入空值标记（短 TTL）防缓存穿透。
//
//	@param ctx context.Context
//	@param c *ReadCache 为 nil 时退化为直接回源（无缓存装配）
//	@param key string 缓存键（constant.ReadCache*KeyTemplate）
//	@param loader func(context.Context) (mo.Option[T], error) 回源加载器，空结果返回 mo.None
//	@return mo.Option[T]
//	@return error
//	@author centonhuang
//	@update 2026-10-10 10:00:00
func GetOrLoad[T any](ctx context.Context, c *ReadCache, key string, loader func(context.Context) (mo.Option[T], error)) (mo.Option[T], error) {
	return GetOrLoadWithTTL(ctx, c, key, constant.ReadCacheTTL, constant.ReadCacheTTLJitter, loader)
}

// GetOrLoadWithTTL 读缓存主入口（调用方自定义 TTL）
//
// 与 GetOrLoad 相同的穿透/雪崩/击穿防护，但条目 TTL 由调用方指定
// （如 keyword 命中集需要 60s±10s 的新鲜度）。jitter 语义同 ReadCacheTTLJitter。
//
//	@param ctx context.Context
//	@param c *ReadCache 为 nil 时退化为直接回源
//	@param key string 缓存键
//	@param ttl time.Duration 条目基准 TTL
//	@param jitter time.Duration TTL 随机抖动上限（<=0 表示不抖动）
//	@param loader func(context.Context) (mo.Option[T], error) 回源加载器，空结果返回 mo.None（短 TTL 空值标记）
//	@return mo.Option[T]
//	@return error
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func GetOrLoadWithTTL[T any](ctx context.Context, c *ReadCache, key string, ttl, jitter time.Duration, loader func(context.Context) (mo.Option[T], error)) (mo.Option[T], error) {
```

函数体 = 既有 `GetOrLoad` 的函数体原文，仅两处替换：
1. `encodeCached(value)` → `encodeCachedWithTTL(value, ttl, jitter)`；
2. 其余逻辑（nil 缓存退化、singleflight、类型断言兜底）一字不动。

同时把 `encodeCached` 改为委托（调用点兼容）：

```go
// encodeCached 编码缓存载荷（默认 TTL）
func encodeCached[T any](value mo.Option[T]) (string, time.Duration, error) {
	return encodeCachedWithTTL(value, constant.ReadCacheTTL, constant.ReadCacheTTLJitter)
}

// encodeCachedWithTTL 编码缓存载荷（调用方自定义 TTL）
//
// mo.None 编码为空值标记（短 TTL，防穿透——空值标记 TTL 不受调用方 ttl 影响）；
// mo.Some 编码为 JSON 载荷，TTL 叠加随机抖动（防雪崩）。
func encodeCachedWithTTL[T any](value mo.Option[T], ttl, jitter time.Duration) (string, time.Duration, error) {
	v, ok := value.Get()
	if !ok {
		return constant.ReadCacheEmptyValue, jitteredTTL(constant.ReadCacheEmptyTTL, constant.ReadCacheEmptyTTLJitter), nil
	}
	payload, err := sonic.MarshalString(v)
	if err != nil {
		return "", 0, err
	}
	return payload, jitteredTTL(ttl, jitter), nil
}
```

`internal/common/constant/readcache.go` 追加（放在 `ReadCacheEmptyTTLJitter` 之后）：

```go
	// ReadCacheKeywordIDsTTL 读缓存-关键词命中用户消息 ID 集合基准 TTL
	//
	// 搜索结果新鲜度敏感（新消息应在分钟级可被搜到），显著短于目录类缓存的 5min；
	// 实际写入值 = 基准 TTL + [0, ReadCacheKeywordIDsTTLJitter) 随机抖动。
	ReadCacheKeywordIDsTTL = 60 * time.Second
	// ReadCacheKeywordIDsTTLJitter 读缓存-关键词命中 ID 集合 TTL 随机抖动上限
	ReadCacheKeywordIDsTTLJitter = 10 * time.Second
```

- [ ] **Step 4: 运行确认通过（含既有用例不回退）**

Run: `rtk go test ./test/unit/cache/... -v`
Expected: PASS（新增 1 个 + 既有全部通过）

- [ ] **Step 5: Commit**

```bash
rtk git add internal/infrastructure/cache/read_cache.go internal/common/constant/readcache.go test/unit/cache/read_cache_test.go
rtk git commit -m "feat(cache): ReadCache 支持 per-call TTL（GetOrLoadWithTTL）"
```

---

### Task 3: 仓储两段式 keyword 查询

**Files:**
- Modify: `internal/common/constant/rediskey.go`（key 模板）
- Modify: `internal/infrastructure/repository/session_repository.go`（结构体、构造函数、`ListAllSessions` / `ListSessionsByOwnerIDs` 的 keyword 分支、新增私有方法）
- Modify: `internal/bootstrap/modules/repository.go:88`（构造签名）
- Test: `internal/infrastructure/repository/keyword_filter_test.go`（新建，同包纯函数单测）

**Interfaces:**
- Consumes: `constant.SessionKeywordFilterSQL` / `constant.SessionKeywordMatchIDsSQL`（Task 1）、`cache.GetOrLoadWithTTL` + `constant.ReadCacheKeywordIDsTTL`（Task 2）
- Produces:
  - `repository.NewSessionReadRepository(db *gorm.DB, rc *cache.ReadCache) session.SessionReadRepository`（签名变更，`rc` 可为 nil；fx 自动注入）
  - 私有：`(*sessionReadRepository).applyKeywordFilter(ctx, sql *gorm.DB, keyword string) (out *gorm.DB, ok bool, err error)`；`formatBigIntArray(ids []uint) string`；`normalizeKeyword(s string) string`

- [ ] **Step 1: 写失败的纯函数单测（新建 `internal/infrastructure/repository/keyword_filter_test.go`，同包 `package repository`）**

```go
package repository

import "testing"

// TestFormatBigIntArray 断言 id 数组序列化为 PG bigint[] 字面量。
// 纯数字拼接，无注入面（禁止用 %s 直接拼用户输入）。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestFormatBigIntArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ids  []uint
		want string
	}{
		{"empty", nil, "{}"},
		{"single", []uint{42}, "{42}"},
		{"multi", []uint{210213, 1, 168194}, "{210213,1,168194}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatBigIntArray(tc.ids); got != tc.want {
				t.Errorf("formatBigIntArray(%v) = %q, want %q", tc.ids, got, tc.want)
			}
		})
	}
}

// TestNormalizeKeyword 断言 keyword 归一化（trim + 小写）用于缓存 key。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestNormalizeKeyword(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"  Shadcn ": "shadcn",
		"Task:":    "task:",
		"":         "",
	}
	for in, want := range cases {
		if got := normalizeKeyword(in); got != want {
			t.Errorf("normalizeKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `rtk go test ./internal/infrastructure/repository/... -run 'TestFormatBigIntArray|TestNormalizeKeyword' -v`
Expected: FAIL（`formatBigIntArray` / `normalizeKeyword` 未定义，编译错误）

- [ ] **Step 3: 实现常量与纯函数**

`internal/common/constant/rediskey.go` 在 `ReadCacheModelListKeyTemplate` 之后追加：

```go
	// ReadCacheKeywordIDsKeyTemplate 读缓存-关键词命中用户消息 ID 集合（%s = 归一化 keyword）
	ReadCacheKeywordIDsKeyTemplate = "cache:rd:kwids:%s"
```

`internal/infrastructure/repository/session_repository.go`：

(a) import 增加（按 goimports 分组规则放置）：

```go
	"strings"
	"strconv"

	"github.com/samber/mo"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
```

(b) 结构体与构造函数（`sessionReadRepository` 字段加 `cache`）：

```go
// sessionReadRepository SessionReadRepository 的 GORM 实现
type sessionReadRepository struct {
	db         *gorm.DB
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
	toolDAO    *dao.ToolDAO
	cache      *cache.ReadCache
}

// NewSessionReadRepository 构造只读仓储
//
//	@param db *gorm.DB
//	@param rc *cache.ReadCache keyword 命中集读缓存；nil 时退化为无缓存直查
//	@return session.SessionReadRepository
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func NewSessionReadRepository(db *gorm.DB, rc *cache.ReadCache) session.SessionReadRepository {
	return &sessionReadRepository{
		db:         db,
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
		toolDAO:    dao.GetToolDAO(),
		cache:      rc,
	}
}
```

(c) 新增私有方法（放在 `sessionSummaryRow` 定义之后）：

```go
// normalizeKeyword 归一化 keyword 作为缓存 key 片段（trim + 小写）
func normalizeKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

// formatBigIntArray 把 id 集合序列化为 PG bigint[] 字面量（"{1,2,3}"）
//
// 纯数字拼接，无 SQL 注入面；供 SessionKeywordFilterSQL 的 = ANY(?::bigint[]) 使用。
//
// ponytail: 无命中上限保护，user 消息超数十万行时再考虑分页/降级/FTS，
// upgrade path: 分区表 + tsvector 或命中集分页传递。
func formatBigIntArray(ids []uint) string {
	return "{" + strings.Join(lo.Map(ids, func(id uint, _ int) string {
		return strconv.FormatUint(uint64(id), 10)
	}), ",") + "}"
}

// keywordMessageIDs 两段式第一段：keyword → 用户消息 id 集合（带读缓存）
//
// 命中为空时返回 nil（GetOrLoad 的 mo.None 空值标记防穿透，短 TTL 自愈）。
func (r *sessionReadRepository) keywordMessageIDs(ctx context.Context, keyword string) ([]uint, error) {
	key := fmt.Sprintf(constant.ReadCacheKeywordIDsKeyTemplate, normalizeKeyword(keyword))
	opt, err := cache.GetOrLoadWithTTL(ctx, r.cache, key,
		constant.ReadCacheKeywordIDsTTL, constant.ReadCacheKeywordIDsTTLJitter,
		func(loadCtx context.Context) (mo.Option[[]uint], error) {
			var ids []uint
			if err := r.db.WithContext(loadCtx).Raw(constant.SessionKeywordMatchIDsSQL, "%"+keyword+"%").Scan(&ids).Error; err != nil {
				return mo.None[[]uint](), ierr.Wrap(ierr.ErrDBQuery, err, "match keyword message ids")
			}
			if len(ids) == 0 {
				return mo.None[[]uint](), nil
			}
			return mo.Some(ids), nil
		})
	if err != nil {
		return nil, err
	}
	ids, _ := opt.Get()
	return ids, nil
}

// applyKeywordFilter 注入 keyword 过滤条件
//
// ok=false 表示关键词命中为空，调用方直接返回空页（短路，不执行第二段查询）。
func (r *sessionReadRepository) applyKeywordFilter(ctx context.Context, sql *gorm.DB, keyword string) (*gorm.DB, bool, error) {
	ids, err := r.keywordMessageIDs(ctx, keyword)
	if err != nil {
		return nil, false, err
	}
	if len(ids) == 0 {
		return nil, false, nil
	}
	return sql.Where(constant.SessionKeywordFilterSQL, formatBigIntArray(ids)), true, nil
}
```

(d) 两个注入点替换。`ListAllSessions` 中（原 `session_repository.go:234-236`）：

```go
	if keyword != "" {
		filtered, ok, err := r.applyKeywordFilter(ctx, sql, keyword)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return []*session.SessionSummaryProjection{}, &model.PageInfo{Page: param.Page, PageSize: param.PageSize}, nil
		}
		sql = filtered
	}
```

`ListSessionsByOwnerIDs` 中（原 `session_repository.go:303-305`）做完全相同的替换（短路返回体与 ListAllSessions 一致：空 slice + `&model.PageInfo{Page: param.Page, PageSize: param.PageSize}`；该方法 rows 为空时 out 经 `lo.Map` 也是空 slice，形态吻合）。

- [ ] **Step 4: 修 bootstrap 包装函数签名（`internal/bootstrap/modules/repository.go:88`）**

```go
func NewSessionReadRepository(db *gorm.DB, rc *cache.ReadCache) session.SessionReadRepository {
	return repository.NewSessionReadRepository(db, rc)
}
```

同文件 import 增加 `internal/infrastructure/cache`（若未引入）。fx 按类型自动解析 `*cache.ReadCache`（`infra.go:60` 已 Provide）。

- [ ] **Step 5: 运行单测确认通过**

Run: `rtk go test ./internal/infrastructure/repository/... ./test/unit/session_keyword_filter/... -v`
Expected: PASS

- [ ] **Step 6: 全量构建 + 既有测试不回退**

Run: `rtk go build ./... && rtk go test ./test/unit/... ./internal/...`
Expected: PASS（重点确认 `test/unit/` 下 session/dao 相关用例——构造函数签名变更若有测试直接调用 `repository.NewSessionReadRepository`，补第二参数 `nil`）

- [ ] **Step 7: Commit**

```bash
rtk git add internal/common/constant/rediskey.go internal/infrastructure/repository/session_repository.go internal/bootstrap/modules/repository.go internal/infrastructure/repository/keyword_filter_test.go
rtk git commit -m "feat(session): keyword 搜索改两段式查询 + ids 读缓存"
```

---

### Task 4: 索引 DDL（生产授权执行）+ 本地 PG 行为验证

**Files:**
- 无代码文件（运维操作 + 手工验证清单）
- 参考：`docs/superpowers/specs/2026-10-15-session-keyword-search-perf-design.md` §详细设计 1

**Interfaces:**
- Consumes: Task 1 的 SQL 常量（验证脚本引用同一形态）
- Produces: 生产索引 `idx_messages_user_content_trgm`；行为验证结论（记录进 memory）

- [ ] **Step 1: 生产索引 DDL（必须经用户逐条授权）**

向用户完整展示以下 SQL 并等待明确授权（`query-prod-database` 索引 DDL 流程；`CONCURRENTLY` 不可包事务，失败会遗留 INVALID 索引）：

```sql
CREATE INDEX CONCURRENTLY idx_messages_user_content_trgm
ON messages USING gin ((message::jsonb->>'content') gin_trgm_ops)
WHERE message::jsonb->>'role' = 'user';
```

- [ ] **Step 2: 执行并验证索引有效性（只读）**

```sql
SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'messages' AND indexname = 'idx_messages_user_content_trgm';
SELECT indexrelid::regclass AS index_name, indisvalid FROM pg_index WHERE indexrelid = 'idx_messages_user_content_trgm'::regclass;
```

Expected: 1 行索引定义 + `indisvalid = t`。若 `indisvalid = f`：`DROP INDEX CONCURRENTLY idx_messages_user_content_trgm;`（需再次授权）后重建。

- [ ] **Step 3: 本地 PG 行为验证（docker 一次性 PG，不进 CI）**

```bash
docker run --rm -d --name kw-pg -e POSTGRES_PASSWORD=test -p 55432:5432 postgres:18.1-alpine
sleep 3
PGPASSWORD=test psql -h 127.0.0.1 -p 55432 -U postgres -d postgres -f /tmp/kw-fixture.sql
PGPASSWORD=test psql -h 127.0.0.1 -p 55432 -U postgres -d postgres -f /tmp/kw-verify.sql
docker stop kw-pg
```

`/tmp/kw-fixture.sql`（列名与 gorm serializer:json 的 TEXT 存储形态对齐：message_ids / questions / model_ids 均为 TEXT，需 `::jsonb` cast）：

```sql
CREATE TABLE messages (
  id bigserial PRIMARY KEY,
  created_at timestamptz DEFAULT now(), updated_at timestamptz DEFAULT now(), deleted_at bigint DEFAULT 0,
  model_id text DEFAULT '', message text NOT NULL, check_sum text DEFAULT '', token_count bigint DEFAULT 0
);
CREATE TABLE sessions (
  id bigserial PRIMARY KEY,
  created_at timestamptz DEFAULT now(), updated_at timestamptz DEFAULT now(), deleted_at bigint DEFAULT 0,
  api_key_name text DEFAULT '', api_key_id bigint DEFAULT 0,
  message_ids text NOT NULL DEFAULT '[]', tool_ids text NOT NULL DEFAULT '[]',
  questions text NOT NULL DEFAULT '[]', model_ids text NOT NULL DEFAULT '[]',
  score int, scored_at timestamptz
);
CREATE INDEX idx_messages_user_content_trgm ON messages USING gin ((message::jsonb->>'content') gin_trgm_ops) WHERE message::jsonb->>'role' = 'user';

INSERT INTO messages (id, message) VALUES
  (1, '{"role":"user","content":"评估下web端的搜索框能换成shadcn原生组件吗"}'),
  (2, '{"role":"user","content":"帮我优化这个中文子串搜索"}'),
  (3, '{"role":"assistant","content":"shadcn appears in assistant reply, must NOT match"}'),
  (4, '{"role":"user","content":"Task: fix the bug"}');
INSERT INTO sessions (id, questions, model_ids, message_ids, tool_ids) VALUES
  (100, '[1]', '["m1"]', '[1]', '[]'),
  (101, '[2]', '["m1"]', '[2]', '[]'),
  (102, '[3]', '["m1"]', '[3]', '[]'),
  (103, '[4]', '["m1"]', '[4]', '[]');
```

`/tmp/kw-verify.sql`（语句与 Task 1 常量形态逐字一致，`= ANY(?::bigint[])` 位置换成字面量）：

```sql
-- V1 第一段：英文子串，必须只命中 id=1（assistant 的 id=3 不得命中）
SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE '%shadcn%';
-- V2 第一段：中文子串，必须命中 id=2
SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE '%中文子串%';
-- V3 第二段：常量数组过滤，必须命中 session 100
SELECT id, COUNT(*) OVER () AS total_count FROM sessions
WHERE deleted_at = 0
  AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
              WHERE arr.mid::bigint = ANY('{1}'))
ORDER BY updated_at DESC LIMIT 20;
-- V4 计划形态：不得出现 SubPlan / 对 messages 的扫描
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, COUNT(*) OVER () AS total_count FROM sessions
WHERE deleted_at = 0
  AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
              WHERE arr.mid::bigint = ANY('{1}'))
ORDER BY updated_at DESC LIMIT 20;
-- V5 索引命中：V1 计划应含 idx_messages_user_content_trgm（Bitmap/索引扫描）
EXPLAIN (ANALYZE, BUFFERS)
SELECT id FROM messages WHERE message::jsonb->>'role' = 'user' AND message::jsonb->>'content' ILIKE '%shadcn%';
```

验证清单（逐项记录结果）：
1. V1 只命中 `{1}`（英文子串 + role 过滤正确）；
2. V2 命中 `{2}`（中文子串）；
3. 无命中 keyword（如 `%不存在词%`）返回 0 行 → 应用层短路；
4. V3 命中 session 100，V4 计划无 `SubPlan`、无 messages 扫描；
5. V5 计划走 `idx_messages_user_content_trgm`；
6. **gorm 绑定路径**：在同库上用 Go 跑 `r.db.Raw(constant.SessionKeywordMatchIDsSQL, "%shadcn%").Scan(&ids)` 与
   `r.db.Raw(constant.SessionKeywordFilterSQL, formatBigIntArray([]uint{1}))`（经两段式方法或临时 main），确认 `= ANY(?::bigint[])` + `"{1}"` 字符串绑定在真实 gorm + PG 下执行成功（可写成一次性 `go test -run` 脚本放 `/tmp`，不入库）。

- [ ] **Step 4: Commit（验证记录不入库则写 memory）**

验证结论追加到 memory `perf/session-keyword-correlated-subplan-rca-2026-10-15`（Serena `edit_memory`），无代码变更不产生 commit。

---

### Task 5: lint / 测试 / ponytail-review / 沉淀 / 部署验证

**Files:**
- 无新增（质量关卡 + 发布流程）

**Interfaces:**
- Consumes: Task 1-4 全部产物
- Produces: 合并到 `master` 的 PR/提交 + 生产验证结论

- [ ] **Step 1: 全量质量关卡**

```bash
make lint && make test
```
（命令清单以 `docs/agents/commands.md` 为准；预期全绿）

- [ ] **Step 2: ponytail-review 审查 diff**

对 `git diff master...HEAD` 执行过度工程审查（投机抽象、重复造轮子、死代码），逐行列出可删项；有则修，无则记录「无发现」。

- [ ] **Step 3: 沉淀工程经验（提交/合并前强制）**

Serena `write_memory` 追加实施经验：gorm `= ANY(?::bigint[])` 绑定实测结论、`GetOrLoadWithTTL` 扩展决策、构造函数签名变更的 fx 影响面。

- [ ] **Step 4: 向用户确认提交方式（禁止擅自推送/合并）**

询问用户：提 MR 还是直接合并 `master`（触发 `docker-publish.yml` 构建部署）。

- [ ] **Step 5: 部署后生产验证**

1. 重放 `GET /api/web/v1/session/list?keyword=shadcn&page=1&pageSize=20&sort=desc&sortField=updated_at`，对比 latency（基线 10.06s，预期 <1s 首次 / 毫秒级缓存命中）；
2. 低频词 + 无命中各一次；
3. 生产 `EXPLAIN` 复核计划无 SubPlan、第一段走 `idx_messages_user_content_trgm`；
4. 若 E2E 失败取 `X-Trace-Id` 回 CLS 排障流程。

- [ ] **Step 6: 清理 worktree**

分支合并后 `git worktree remove .worktrees/session-keyword-search-perf`（失败则删目录 + `git worktree prune`）。
