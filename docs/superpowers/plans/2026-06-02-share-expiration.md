# 分享链接自定义过期时间 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to choose share link expiration: 1d / 7d / 30d / never / custom date picker

**Architecture:** Backend adds `expiresIn` string enum (1d/7d/30d/never/custom) + optional `expiresAt` timestamp to `CreateShareReqBody`. Handler maps to `time.Duration`, passes to `ShareCache.CreateShare`. Redis TTL and `shareRecord` use dynamic TTL instead of hardcoded `constant.ShareTTL`. Frontend adds radio group + date picker to ShareDialog.

**Tech Stack:** Go 1.25, Huma, Redis, Next.js 16, shadcn/ui

---

### Task 1: 常量 & DTO 变更

**Files:**
- Modify: `internal/common/constant/share.go`
- Modify: `internal/dto/session_share.go`
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: 更新常量文件，新增具名 TTL 常量**

编辑 `internal/common/constant/share.go`：

```go
package constant

import "time"

const (
    ShareTTLDefault       = 24 * time.Hour
    ShareTTL1Day          = 24 * time.Hour
    ShareTTL1Week         = 7 * 24 * time.Hour
    ShareTTL1Month        = 30 * 24 * time.Hour
    ShareTTLNeverExpire   = 100 * 365 * 24 * time.Hour

    ShareExpiredRetention = 72 * time.Hour

    ShareIDAlphabet          = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    ShareIDMinLen            = 6
    ShareIDMaxLen            = 8
    ShareIDMaxAttemptsPerLen = 3
)
```

删除旧的 `ShareTTL = 24 * time.Hour`（已被 `ShareTTLDefault` 替代）。

- [ ] **Step 2: 更新 DTO，新增 expiresIn / expiresAt 字段**

编辑 `internal/dto/session_share.go` 的 `CreateShareReqBody`：

```go
type CreateShareReqBody struct {
    SessionID uint   `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
    ExpiresIn string `json:"expiresIn" doc:"过期选项: 1d | 7d | 30d | never | custom，默认 1d"`
    ExpiresAt *int64 `json:"expiresAt,omitempty" doc:"自定义过期 Unix 秒级时间戳，expiresIn=custom 时必填"`
}
```

- [ ] **Step 3: 更新前端 TS 类型**

编辑 `web/src/lib/types.ts` 的 `CreateShareReqBody`：

```typescript
export interface CreateShareReqBody {
  sessionId: number;
  expiresIn?: string;
  expiresAt?: number;
}
```

---

### Task 2: shareRecord 增加 TTL 字段 + ShareCache 接口变更

**Files:**
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: shareRecord 新增 TTL 字段**

```go
type shareRecord struct {
    ShareID   string `json:"shareId"`
    SessionID uint   `json:"sessionId"`
    CreatedAt int64  `json:"createdAt"`
    TTL       int64  `json:"ttl"`
}
```

- [ ] **Step 2: ShareCache 接口 CreateShare 增加 ttl 参数**

```go
type ShareCache interface {
    CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error)
    // 其余方法不变
}
```

- [ ] **Step 3: 运行 tests 验证编译失败（接口变更契约）**

Run: `go build ./...`
Expected: 编译失败，`mockShareCache` 未实现新接口

---

### Task 3: reserveShareID 支持动态 TTL

**Files:**
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: reserveShareID 新增 ttl 参数并使用它**

```go
func (s *shareCache) reserveShareID(ctx context.Context, sessionID uint, ttl time.Duration) (string, string, error) {
    for length := constant.ShareIDMinLen; length <= constant.ShareIDMaxLen; length++ {
        for attempt := 0; attempt < constant.ShareIDMaxAttemptsPerLen; attempt++ {
            shareID, genErr := util.GenerateShareID(sessionID, length)
            if genErr != nil {
                return "", "", genErr
            }
            key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
            ok, setErr := s.cache.SetNX(ctx, key, sessionID, ttl).Result()
            if setErr != nil {
                return "", "", ierr.Wrap(ierr.ErrInternal, setErr, "failed to reserve share key")
            }
            if ok {
                return shareID, key, nil
            }
        }
    }
    return "", "", ierr.New(ierr.ErrInternal, "failed to reserve unique shareID after retries")
}
```

---

### Task 4: CreateShare 实现动态 TTL

**Files:**
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: 更新 CreateShare 方法**

```go
func (s *shareCache) CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error) {
    if sessionID == 0 {
        return "", time.Time{}, ierr.New(ierr.ErrValidation, "sessionID must be greater than 0")
    }
    if ttl <= 0 {
        return "", time.Time{}, ierr.New(ierr.ErrValidation, "ttl must be greater than 0")
    }

    existingShareID, checkErr := s.GetSessionShareID(ctx, sessionID)
    if checkErr != nil {
        return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, checkErr, "failed to check existing share")
    }
    if existingShareID != "" {
        return "", time.Time{}, ierr.New(ierr.ErrDataExists, "session already has an active share")
    }

    now := time.Now()
    expiresAt := now.Add(ttl)

    shareID, key, reserveErr := s.reserveShareID(ctx, sessionID, ttl)
    if reserveErr != nil {
        return "", time.Time{}, reserveErr
    }

    userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
    sessionSharesKey := fmt.Sprintf(constant.SessionSharesKeyTemplate, sessionID)

    record := &shareRecord{
        ShareID:   shareID,
        SessionID: sessionID,
        CreatedAt: now.Unix(),
        TTL:       int64(ttl.Seconds()),
    }
    recordJSON, err := sonic.Marshal(record)
    if err != nil {
        s.cache.Del(ctx, key)
        return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, "failed to marshal share record")
    }

    pipe := s.cache.Pipeline()
    pipe.ZAdd(ctx, userSharesKey, redis.Z{
        Score:  float64(record.CreatedAt),
        Member: string(recordJSON),
    })
    pipe.SAdd(ctx, sessionSharesKey, shareID)
    pipe.Expire(ctx, sessionSharesKey, ttl)

    if _, execErr := pipe.Exec(ctx); execErr != nil {
        s.cache.Del(ctx, key)
        return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, execErr, "failed to create share")
    }

    return shareID, expiresAt, nil
}
```

- [ ] **Step 2: 运行聚焦测试验证编译通过**

Run: `go build ./...`
Expected: 编译成功

---

### Task 5: ListUserShares 使用存储的 TTL

**Files:**
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: 更新 ListUserShares 计算逻辑**

找到 `ListUserShares` 方法，修改两处：

```go
// 修改范围过滤：使用 ShareTTLNeverExpire 确保"永不过期"的记录不被主动清除
minCreatedAt := now.Add(-constant.ShareTTLNeverExpire - constant.ShareExpiredRetention).Unix()

// 修改 ExpiresAt 计算：使用记录中存储的 TTL
expiresAt := time.Unix(r.CreatedAt, 0).Add(time.Duration(r.TTL) * time.Second)
if expiresAt.Before(retentionStart) {
    continue
}
```

`ListUserShares` 完整方法变更后如下：

```go
func (s *shareCache) ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error) {
    userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
    if page < 1 {
        page = 1
    }
    if pageSize < 1 {
        pageSize = 20
    }

    now := time.Now()
    minCreatedAt := now.Add(-constant.ShareTTLNeverExpire - constant.ShareExpiredRetention).Unix()
    results, zErr := s.cache.ZRevRangeByScore(ctx, userSharesKey, &redis.ZRangeBy{
        Max: constant.RedisZRangePositiveInfinity,
        Min: strconv.FormatInt(minCreatedAt, constant.DecimalBase),
    }).Result()
    if zErr != nil {
        return nil, nil, ierr.Wrap(ierr.ErrInternal, zErr, "failed to list user shares")
    }

    var records []shareRecord
    for _, result := range results {
        var record shareRecord
        if unmarshalErr := sonic.Unmarshal([]byte(result), &record); unmarshalErr != nil {
            continue
        }
        records = append(records, record)
    }

    pipe := s.cache.Pipeline()
    existsCmds := make([]*redis.IntCmd, len(records))
    for i, r := range records {
        shareKey := fmt.Sprintf(constant.ShareKeyTemplate, r.ShareID)
        existsCmds[i] = pipe.Exists(ctx, shareKey)
    }
    if _, pipeErr := pipe.Exec(ctx); pipeErr != nil && !errors.Is(pipeErr, redis.Nil) {
        return nil, nil, ierr.Wrap(ierr.ErrInternal, pipeErr, "failed to batch check share keys")
    }

    retentionStart := now.Add(-constant.ShareExpiredRetention)
    items := make([]*dto.ShareItem, 0, len(records))
    for i, r := range records {
        createdAt := time.Unix(r.CreatedAt, 0)
        expiresAt := createdAt.Add(time.Duration(r.TTL) * time.Second)
        if expiresAt.Before(retentionStart) {
            continue
        }
        if existsCmds[i].Val() == 0 && !expiresAt.Before(now) {
            continue
        }

        items = append(items, &dto.ShareItem{
            ShareID:   r.ShareID,
            SessionID: r.SessionID,
            CreatedAt: createdAt,
            ExpiresAt: expiresAt,
        })
    }

    total := int64(len(items))
    start := (page - 1) * pageSize
    if start >= len(items) {
        items = []*dto.ShareItem{}
    } else {
        end := start + pageSize
        if end > len(items) {
            end = len(items)
        }
        items = items[start:end]
    }

    pageInfo := &model.PageInfo{
        Page:     page,
        PageSize: pageSize,
        Total:    total,
    }

    return items, pageInfo, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译成功

---

### Task 6: Handler 新增 parseExpiresIn 映射函数

**Files:**
- Modify: `internal/handler/session.go`

- [ ] **Step 1: 新增 import**

在 `internal/handler/session.go` 的 import 中确认已有 `"time"`（可能需要新增）。

- [ ] **Step 2: 新增 parseExpiresIn 函数**

在 handler 文件中新增工具函数（放在文件末尾或 `HandleCreateShare` 之前）：

```go
func parseExpiresIn(expiresIn string, customAt *int64) (time.Duration, error) {
    switch expiresIn {
    case "1d", "":
        return constant.ShareTTL1Day, nil
    case "7d", "1w":
        return constant.ShareTTL1Week, nil
    case "30d", "1M":
        return constant.ShareTTL1Month, nil
    case "never":
        return constant.ShareTTLNeverExpire, nil
    case "custom":
        if customAt == nil {
            return 0, ierr.New(ierr.ErrValidation, "expiresAt is required when expiresIn is custom")
        }
        t := time.Unix(*customAt, 0)
        remaining := time.Until(t)
        if remaining <= 0 {
            return 0, ierr.New(ierr.ErrValidation, "expiresAt must be in the future")
        }
        return remaining, nil
    default:
        return constant.ShareTTLDefault, nil
    }
}
```

- [ ] **Step 3: 更新 HandleCreateShare 使用 parseExpiresIn**

找到 `HandleCreateShare` 中的：

```go
shareID, expiresAt, shareErr := h.shareCache.CreateShare(ctx, userID, sessionID)
```

替换为：

```go
ttl, parseErr := parseExpiresIn(req.Body.ExpiresIn, req.Body.ExpiresAt)
if parseErr != nil {
    logger.WithCtx(ctx).Warn("[SessionHandler] Create share: invalid expiration",
        zap.String("expiresIn", req.Body.ExpiresIn), zap.Error(parseErr))
    rsp.Error = ierr.ToBizError(parseErr, ierr.ErrValidation.BizError())
    return apiutil.WrapHTTPResponse(rsp, nil)
}

shareID, expiresAt, shareErr := h.shareCache.CreateShare(ctx, userID, sessionID, ttl)
```

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译成功

---

### Task 7: 更新 mockShareCache 和单元测试

**Files:**
- Modify: `test/unit/session_share/session_share_test.go`

- [ ] **Step 1: 更新 mockShareCache.CreateShare 签名**

```go
func (m *mockShareCache) CreateShare(_ context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error) {
    if m.createErr != nil {
        return "", time.Time{}, m.createErr
    }
    if m.sharedSessions[sessionID] {
        return "", time.Time{}, ierr.New(ierr.ErrDataExists, "session already has an active share")
    }
    shareID := "test-share-id-" + time.Now().Format("150405")
    now := time.Now()
    m.shares[shareID] = &mockShareEntry{
        userID:    userID,
        sessionID: sessionID,
        createdAt: now,
        expiresAt: now.Add(ttl),
    }
    m.userShares[userID] = append(m.userShares[userID], shareID)
    m.sharedSessions[sessionID] = true
    return shareID, now.Add(ttl), nil
}
```

- [ ] **Step 2: 更新所有调用 mockCreateShare 的地方传入 ttl**

将测试中所有 `sc.CreateShare(context.Background(), userID, sessionID)` 改为 `sc.CreateShare(context.Background(), userID, sessionID, constant.ShareTTLDefault)`。

更新位置：
- `TestListShares_Success` L272-273：两个 `sc.CreateShare`
- `TestDeleteShare_Success` L304
- `TestCreateShare_AlreadyShared`（不使用 `sc.CreateShare`，直接用 `sharedSessions`，无需改）

- [ ] **Step 3: 更新 seedRedisShare 写入 TTL 字段**

```go
func seedRedisShare(t *testing.T, rdb *redis.Client, userID uint, shareID string, sessionID uint, createdAt time.Time, active bool, ttl time.Duration) {
    t.Helper()
    ctx := context.Background()
    record := struct {
        ShareID   string `json:"shareId"`
        SessionID uint   `json:"sessionId"`
        CreatedAt int64  `json:"createdAt"`
        TTL       int64  `json:"ttl"`
    }{
        ShareID:   shareID,
        SessionID: sessionID,
        CreatedAt: createdAt.Unix(),
        TTL:       int64(ttl.Seconds()),
    }
    recordJSON, err := sonic.Marshal(record)
    if err != nil {
        t.Fatalf("marshal share record failed: %v", err)
    }
    if err := rdb.ZAdd(ctx, fmt.Sprintf(constant.UserSharesKeyTemplate, userID), redis.Z{
        Score:  float64(record.CreatedAt),
        Member: string(recordJSON),
    }).Err(); err != nil {
        t.Fatalf("seed user share failed: %v", err)
    }
    if !active {
        return
    }
    remaining := time.Until(createdAt.Add(ttl))
    if remaining <= 0 {
        t.Fatalf("active share %s has non-positive ttl %s", shareID, remaining)
    }
    if err := rdb.Set(ctx, fmt.Sprintf(constant.ShareKeyTemplate, shareID), sessionID, remaining).Err(); err != nil {
        t.Fatalf("seed active share key failed: %v", err)
    }
}
```

- [ ] **Step 4: 更新 seedRedisShare 调用处**

在 `TestRedisShareCacheListUserShares_IncludesRecentlyExpiredOnly` 中更新三个调用：

```go
seedRedisShare(t, rdb, userID, "active-share", 1, now.Add(-time.Hour), true, constant.ShareTTLDefault)
seedRedisShare(t, rdb, userID, "recent-expired-share", 2, now.Add(-constant.ShareTTLDefault-time.Hour), false, constant.ShareTTLDefault)
seedRedisShare(t, rdb, userID, "old-expired-share", 3, now.Add(-constant.ShareTTLDefault-72*time.Hour-time.Hour), false, constant.ShareTTLDefault)
```

- [ ] **Step 5: 更新 TestCreateShare_DTOFollowsHumaBodyConvention 回归测试**

新增对 `ExpiresIn` 和 `ExpiresAt` 字段的检查。在测试函数中添加：

```go
expiresInField, ok := bodyType.FieldByName("ExpiresIn")
if !ok {
    t.Fatal("CreateShareReqBody must have ExpiresIn field")
}
if expiresInField.Tag.Get("json") != "expiresIn" {
    t.Errorf(`CreateShareReqBody.ExpiresIn json tag = %q, want "expiresIn"`, expiresInField.Tag.Get("json"))
}

expiresAtField, ok := bodyType.FieldByName("ExpiresAt")
if !ok {
    t.Fatal("CreateShareReqBody must have ExpiresAt field")
}
```

- [ ] **Step 6: 新增 parseExpiresIn 单元测试**

在 `TestCreateShare_AlreadyShared` 后新增测试：

```go
func TestParseExpiresIn(t *testing.T) {
    future := time.Now().Add(48 * time.Hour).Unix()

    tests := []struct {
        name      string
        expiresIn string
        customAt  *int64
        want      time.Duration
        wantErr   bool
    }{
        {"default (empty)", "", nil, constant.ShareTTLDefault, false},
        {"1 day", "1d", nil, constant.ShareTTL1Day, false},
        {"1 week", "7d", nil, constant.ShareTTL1Week, false},
        {"1 week alt", "1w", nil, constant.ShareTTL1Week, false},
        {"1 month", "30d", nil, constant.ShareTTL1Month, false},
        {"1 month alt", "1M", nil, constant.ShareTTL1Month, false},
        {"never", "never", nil, constant.ShareTTLNeverExpire, false},
        {"custom valid", "custom", &future, 48 * time.Hour, false}, // approximate
        {"custom missing at", "custom", nil, 0, true},
        {"custom past time", "custom", lo.ToPtr(time.Now().Add(-time.Hour).Unix()), 0, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := handler.ParseExpiresIn(tt.expiresIn, tt.customAt)
            if tt.wantErr {
                if err == nil {
                    t.Error("expected error but got nil")
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            // custom case: approximate match
            if tt.expiresIn == "custom" && tt.customAt != nil {
                expectedTTL := time.Until(time.Unix(*tt.customAt, 0))
                diff := got - expectedTTL
                if diff < 0 {
                    diff = -diff
                }
                if diff > time.Second {
                    t.Errorf("ttl = %v, want ~%v (diff %v)", got, expectedTTL, diff)
                }
            } else if got != tt.want {
                t.Errorf("parseExpiresIn(%q) = %v, want %v", tt.expiresIn, got, tt.want)
            }
        })
    }
}
```

注意：`parseExpiresIn` 是 `sessionHandler` 的辅助函数，需要导出或使用类型断言。实际实现中可以把它作为包级函数在 handler 包中，测试中直接调用 `handler.ParseExpiresIn`（首字母大写导出）或通过未导出函数测试。

建议将 `parseExpiresIn` 改为导出的 `ParseExpiresIn` 以便测试。

- [ ] **Step 7: 新增 CreateShare 不同 TTL 的集成测试**

```go
func TestRedisShareCache_CreateShare_WithCustomTTL(t *testing.T) {
    ctx := context.Background()
    server := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
    defer func() { _ = rdb.Close() }()

    shareCache := cache.NewShareCache(rdb)
    userID := uint(42)
    sessionID := uint(1)

    // 先创建一个 1-week TTL 的分享
    shareID, expiresAt, err := shareCache.CreateShare(ctx, userID, sessionID, constant.ShareTTL1Week)
    if err != nil {
        t.Fatalf("CreateShare failed: %v", err)
    }
    if shareID == "" {
        t.Fatal("expected non-empty shareID")
    }

    expectedExpiry := time.Now().Add(constant.ShareTTL1Week)
    diff := expiresAt.Sub(expectedExpiry)
    if diff < 0 {
        diff = -diff
    }
    if diff > time.Second {
        t.Errorf("expiresAt = %v, want ~%v", expiresAt, expectedExpiry)
    }

    // 通过 GetShareSessionID 验证 key 存在 (TTL 由 Redis 托管)
    gotSessionID, err := shareCache.GetShareSessionID(ctx, shareID)
    if err != nil {
        t.Fatalf("GetShareSessionID failed: %v", err)
    }
    if gotSessionID != sessionID {
        t.Errorf("GetShareSessionID = %d, want %d", gotSessionID, sessionID)
    }

    // 验证 ListUserShares 中 TTL 计算正确
    items, _, listErr := shareCache.ListUserShares(ctx, userID, 1, 10)
    if listErr != nil {
        t.Fatalf("ListUserShares failed: %v", listErr)
    }
    if len(items) != 1 {
        t.Fatalf("shares count = %d, want 1", len(items))
    }
    if items[0].ShareID != shareID {
        t.Errorf("shareID = %s, want %s", items[0].ShareID, shareID)
    }
    if items[0].ExpiresAt.Before(time.Now()) {
        t.Error("expiresAt should be in the future for 1-week share")
    }
}
```

- [ ] **Step 8: 运行全部单元测试**

Run: `go test -count=1 ./test/unit/session_share/ -v`
Expected: 所有测试通过

---

### Task 8: Web 前端 — 过期选项选择器

**Files:**
- Modify: `web/src/components/share/share-dialog.tsx`

- [ ] **Step 1: 新增 import**

```typescript
import { CalendarIcon } from "lucide-react";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
```

- [ ] **Step 2: 新增过期选项类型和 state**

在 `ShareDialog` 组件内新增：

```typescript
type ExpireOption = "1d" | "7d" | "30d" | "never" | "custom";

const [expireOption, setExpireOption] = useState<ExpireOption>("1d");
const [customDate, setCustomDate] = useState<Date | undefined>(undefined);
```

- [ ] **Step 3: 更新 createShare API 调用**

```typescript
const createShare = useCallback(async () => {
    setCreating(true);
    try {
      const body: CreateShareReqBody = { sessionId };
      if (expireOption !== "1d") {
        body.expiresIn = expireOption;
      }
      if (expireOption === "custom" && customDate) {
        body.expiresAt = Math.floor(customDate.getTime() / 1000);
      }
      const rsp = await api.createShare(body);
      // ... 其余不变
    }
    // ...
}, [sessionId, expireOption, customDate]);
```

- [ ] **Step 4: 在 Dialog UI 中插入过期选项选择器**

在 description 之后、"生成链接"的边框区域之前插入过期选项 UI：

```tsx
{!shareURL && (
  <div className="space-y-4">
    <div className="space-y-2">
      <Label>Link expiration</Label>
      <RadioGroup
        value={expireOption}
        onValueChange={(v) => setExpireOption(v as ExpireOption)}
        className="flex flex-wrap gap-2"
      >
        {[
          { value: "1d", label: "1 day" },
          { value: "7d", label: "1 week" },
          { value: "30d", label: "1 month" },
          { value: "never", label: "Never" },
          { value: "custom", label: "Custom" },
        ].map((opt) => (
          <div key={opt.value} className="flex items-center gap-1.5">
            <RadioGroupItem value={opt.value} id={`expire-${opt.value}`} />
            <Label htmlFor={`expire-${opt.value}`} className="text-sm font-normal cursor-pointer">
              {opt.label}
            </Label>
          </div>
        ))}
      </RadioGroup>
    </div>

    {expireOption === "custom" && (
      <Popover>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            className={cn(
              "w-full justify-start text-left font-normal",
              !customDate && "text-muted-foreground"
            )}
          >
            <CalendarIcon className="mr-2 size-4" />
            {customDate ? format(customDate, "PPP") : "Pick a date"}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="start">
          <Calendar
            mode="single"
            selected={customDate}
            onSelect={setCustomDate}
            disabled={(date) => date < new Date()}
            initialFocus
          />
        </PopoverContent>
      </Popover>
    )}

    <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 px-4 py-5 text-center text-sm text-muted-foreground">
      Anyone with the generated link will be able to read this session
      until it expires.
    </div>
  </div>
)}
```

注意：原有的 description 文字需要更新 — 去掉 "The link expires after 24 hours" 的硬编码描述。

更新 description（第 121-125 行）：

```tsx
<DialogDescription>
  {shareURL
    ? "Copy the public link below to share this conversation. The link can be revoked anytime from the Shares page."
    : "Generate a public link to this conversation. Choose how long the link stays valid."}
</DialogDescription>
```

- [ ] **Step 5: 构建验证**

Run: `cd web && npm run build`
Expected: 类型检查和构建通过

---

### Task 9: 全量回归验证

- [ ] **Step 1: 运行后端 lint**

Run: `make lint`
Expected: 无 lint 错误

- [ ] **Step 2: 运行后端全量测试**

Run: `go test -count=1 ./...`
Expected: 全部通过

- [ ] **Step 3: 再次确认前端构建**

Run: `cd web && npm run build`
Expected: 构建成功
