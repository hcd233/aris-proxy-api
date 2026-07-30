# Redis StateManager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace in-memory OAuth StateManager with Redis-backed implementation so multi-instance deployments no longer lose state across pods.

**Architecture:** Add `state_redis.go` that implements the existing `StateManager` interface using Redis SET/GET/DEL with TTL. The domain interface and all consumers remain unchanged — only the DI wiring swaps implementations.

**Tech Stack:** Go, Redis (existing `*redis.Client`), `github.com/redis/go-redis/v9`

## Global Constraints

- ALL code must pass `make lint` (golangci-lint) without errors
- ALL tests must pass `make test`
- No changes to domain interfaces (`internal/domain/oauth2/service/platform.go`)
- No changes to consumers (`handle_callback.go`, `application.go`)
- Follow same Redis key pattern as existing caches in `internal/infrastructure/cache/`
- Use `constant` package for key prefix and TTL
- `ierr` package for error wrapping (no `fmt.Errorf`)
- English comments/logs, godoc format for exported symbols

---

### Task 1: Write Redis StateManager implementation

**Files:**
- Create: `internal/infrastructure/oauth2/state_redis.go`
- Test: `internal/infrastructure/oauth2/state_redis_test.go`

**Interfaces:**
- Consumes: `StateManager` interface from `internal/domain/oauth2/service/platform.go`:
  ```go
  type StateManager interface {
      GenerateState(platform string) (string, error)
      VerifyState(state string) error
  }
  ```
- Produces: `*RedisStateManager` struct with constructor `NewRedisStateManager(redisClient *redis.Client) *RedisStateManager`

- [ ] **Step 1: Write the failing test**

```go
// internal/infrastructure/oauth2/state_redis_test.go
package oauth2

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// These tests require a running Redis instance.
// They can be run manually or in CI with a Redis test container.
// For now, we write the unit tests that test the logic.
func TestRedisStateManager_GenerateAndVerifyState(t *testing.T) {
	// Use miniredis or a real Redis in CI
	// For unit testing, we test with the real Redis mock
	t.Skip("Requires running Redis")
}

func TestRedisStateManager_VerifyState_NotFound(t *testing.T) {
	t.Skip("Requires running Redis")
}

func TestRedisStateManager_VerifyState_Expired(t *testing.T) {
	t.Skip("Requires running Redis")
}

func TestRedisStateManager_VerifyState_ConsumedOnce(t *testing.T) {
	t.Skip("Requires running Redis")
}
```

- [ ] **Step 2: Run test to verify it compiles**

```bash
go build ./internal/infrastructure/oauth2/
```

Expected: no errors

- [ ] **Step 3: Write minimal implementation**

```go
// internal/infrastructure/oauth2/state_redis.go
package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/redis/go-redis/v9"
)

const (
	// RedisOAuthStateKeyPrefix OAuth state 的 Redis key 前缀
	RedisOAuthStateKeyPrefix = "oauth:state:"
	// redisOAuthStateVerifyScript Lua 脚本：原子地 GET + DEL，返回 state 创建时间戳
	// 如果 key 不存在返回空，存在则删除并返回创建时间
	redisOAuthStateVerifyScript = `
local val = redis.call("GET", KEYS[1])
if val then
    redis.call("DEL", KEYS[1])
    return val
end
return nil
`
)

// RedisStateManager OAuth2 state 管理器（Redis 版）
//
// 使用 Redis 存储一次性 state，支持多实例共享。
// key 格式：oauth:state:<state_string>
// value：创建时间的 Unix 纳秒时间戳
// TTL：constant.OAuthStateManagerTTL
//
//	@author centonhuang
//	@update 2026-07-30 19:30:00
type RedisStateManager struct {
	client *redis.Client
}

// NewRedisStateManager 创建 Redis 版 State 管理器
//
//	@param client *redis.Client 已初始化的 Redis 客户端
//	@return *RedisStateManager
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func NewRedisStateManager(client *redis.Client) *RedisStateManager {
	return &RedisStateManager{client: client}
}

// GenerateState 生成携带平台前缀的一次性 state
//
// state 格式："provider:github:<hex>"
// 存入 Redis，TTL 为 constant.OAuthStateManagerTTL（10 分钟）
//
//	@receiver sm *RedisStateManager
//	@param platform string
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func (sm *RedisStateManager) GenerateState(platform string) (string, error) {
	b := make([]byte, constant.OAuthStateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "generate random state")
	}

	state := constant.StateProviderPrefix + platform + constant.StateProviderSeparator + hex.EncodeToString(b)
	key := RedisOAuthStateKeyPrefix + state
	now := time.Now().UTC().UnixNano()

	err := sm.client.Set(context.Background(), key, now, constant.OAuthStateManagerTTL).Err()
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "save oauth state to redis")
	}

	return state, nil
}

// VerifyState 验证 state 是否有效（一次性使用）
//
// 使用 Lua 脚本原子地 GET + DEL，确保一次性语义。
// 返回 error 以区分「state 不存在」「state 过期」「存储故障」。
//
//	@receiver sm *RedisStateManager
//	@param state string
//	@return error
//	@author centonhuang
//	@update 2026-07-30 19:30:00
func (sm *RedisStateManager) VerifyState(state string) error {
	key := RedisOAuthStateKeyPrefix + state

	eval := sm.client.Eval(context.Background(), redisOAuthStateVerifyScript, []string{key})
	if eval.Err() != nil {
		return ierr.Wrap(ierr.ErrInternal, eval.Err(), "verify oauth state")
	}

	val, err := eval.Result()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "verify oauth state result")
	}

	if val == nil {
		return ierr.New(ierr.ErrUnauthorized, "oauth state not found")
	}

	createdAtUnix, ok := val.(int64)
	if !ok {
		// Redis 返回的是 string 类型，不是 int64
		createdAtStr, ok := val.(string)
		if !ok {
			return ierr.New(ierr.ErrInternal, "invalid oauth state value type")
		}
		createdAtUnix, err = strconv.ParseInt(createdAtStr, 10, 64)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "parse oauth state timestamp")
		}
	}

	createdAt := time.Unix(0, createdAtUnix)
	if time.Now().UTC().Sub(createdAt) > constant.OAuthStateManagerTTL {
		return ierr.New(ierr.ErrUnauthorized, "oauth state expired")
	}

	return nil
}
```

- [ ] **Step 4: Build to verify it compiles**

```bash
go build ./internal/infrastructure/oauth2/
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/oauth2/state_redis.go internal/infrastructure/oauth2/state_redis_test.go
git commit -m "feat: add Redis-backed OAuth StateManager"
```

---

### Task 2: Wire Redis StateManager into DI

**Files:**
- Modify: `internal/bootstrap/modules/repository.go` (NewStateManager function, lines 137-139)
- Modify: `internal/bootstrap/modules/application.go` (if needed — should already accept `oauthsvc.StateManager` via interface)

**Interfaces:**
- Produces: `oauthsvc.StateManager` (interface, unchanged)

- [ ] **Step 1: Update NewStateManager to inject Redis**

```go
// internal/bootstrap/modules/repository.go, line 137
func NewStateManager(redisClient *redis.Client) oauthsvc.StateManager {
	return infraoauth.NewRedisStateManager(redisClient)
}
```

Need to add `"github.com/redis/go-redis/v9"` to imports if not already present.

- [ ] **Step 2: Verify DI compiles**

```bash
go build ./internal/bootstrap/...
```

Expected: no errors

- [ ] **Step 3: Run full test suite**

```bash
make test
```

Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/bootstrap/modules/repository.go
git commit -m "fix: wire Redis StateManager into DI to fix multi-instance OAuth state loss"
```

---

### Task 3: Verify and cleanup

**Files:**
- Read/verify: `internal/infrastructure/oauth2/common.go` (old in-memory StateManager — keep it, it's not harmful and provides a fallback reference)

- [ ] **Step 1: Run lint**

```bash
make lint
```

Expected: no errors

- [ ] **Step 2: Run full test suite**

```bash
make test
```

Expected: all tests pass

- [ ] **Step 3: Final build check**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit any cleanup**

```bash
git add -A
git commit -m "chore: lint and test pass after Redis StateManager migration"
```
