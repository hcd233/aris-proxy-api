# 分享链接自定义过期时间

- **日期**：2026-06-02
- **作者**：opencode
- **状态**：设计稿

## 概述

分享链接目前固定 24 小时过期，用户希望支持自定义过期时间：1 天 / 1 周 / 1 月 / 永不过期 / 自定义日期。

## 约束

- 后端 Redis 存储不变，"永不过期"使用 100 年 TTL 实现
- 旧客户端兼容（不传 `expiresIn` 时默认 1 天）
- `ListUserShares` 过期计算不再依赖全局常量，而用 `shareRecord` 中存储的 TTL

## DTO 变更

```go
// internal/dto/session_share.go

type CreateShareReqBody struct {
    SessionID uint   `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
    ExpiresIn string `json:"expiresIn" doc:"过期选项: 1d | 7d | 30d | never | custom，默认 1d"`
    ExpiresAt *int64 `json:"expiresAt,omitempty" doc:"自定义过期 Unix 秒级时间戳，expiresIn=custom 时必填"`
}
```

## 常量变更

```go
// internal/common/constant/share.go

const (
    ShareTTLDefault       = 24 * time.Hour
    ShareTTL1Day          = 24 * time.Hour
    ShareTTL1Week         = 7 * 24 * time.Hour
    ShareTTL1Month        = 30 * 24 * time.Hour
    ShareTTLNeverExpire   = 100 * 365 * 24 * time.Hour
    ShareExpiredRetention = 72 * time.Hour
)
```

删除旧 `ShareTTL`（之前只有 24h），替换为上述具名常量。

## shareRecord 新增 TTL 字段

```go
type shareRecord struct {
    ShareID   string `json:"shareId"`
    SessionID uint   `json:"sessionId"`
    CreatedAt int64  `json:"createdAt"`
    TTL       int64  `json:"ttl"`    // 新增：过期 TTL（秒）
}
```

## ShareCache 接口变更

```go
type ShareCache interface {
    CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error)
    // 其余方法不变
}
```

## Handler 变更

新增 `parseExpiresIn` 将 `expiresIn` 字符串映射为 `time.Duration`：
- `"1d"` → `ShareTTL1Day`
- `"7d"` / `"1w"` → `ShareTTL1Week`
- `"30d"` / `"1M"` → `ShareTTL1Month`
- `"never"` → `ShareTTLNeverExpire`
- `"custom"` → 校验 `ExpiresAt` 非空，计算 `time.Until(t)`
- 其他/unset → `ShareTTLDefault`

`HandleCreateShare` 从 `req.Body.ExpiresIn` 解析 TTL，传入 `shareCache.CreateShare`。

## Cache 实现变更

- `reserveShareID` 的 `SetNX` 使用入参 `ttl`
- `shareRecord.TTL` 写入 `int64(ttl.Seconds())`
- `sessionSharesKey` 的 `Expire` 使用入参 `ttl`
- `ListUserShares` 中 `ExpiresAt` 改为 `createdAt.Add(time.Duration(r.TTL) * time.Second)`
- `ListUserShares` 中 `listUserShares` 的范围过滤 `minCreatedAt` 改为 `now.Add(-max(ShareTTLNeverExpire, ShareExpiredRetention))`，确保"永不过期"的分享不被主动清除

## 前端 ShareDialog 变更

- 在创建按钮上方插入 Radio 组（1天 / 1周 / 1月 / 永不过期 / 自定义）
- 默认选中"1天"
- 选"自定义"时展开 shadcn DatePicker（`Calendar` + `Popover`）
- 提交时带上 `expiresIn` 和可选的 `expiresAt`
- 共享管理页面（shares page）无变化，`expiresAt` 字段已由后端正确计算

## API 客户端 & Types 变更

```typescript
// web/src/lib/types.ts
export interface CreateShareReqBody {
  sessionId: number;
  expiresIn?: string;
  expiresAt?: number;
}
```

`api-client.ts` 的 `createShare` 方法无需改签名（已接收 `CreateShareReqBody`）。

## 测试

- 单元测试覆盖所有 `ParseExpiresIn` 映射 case
- 单元测试覆盖 `CreateShare` 传入不同 TTL 时 Redis SETNX TTL 正确
- 单元测试覆盖 `ListUserShares` 中"永不过期"的分享不被过滤
- 更新 DTO 序列化测试确保 `ExpiresIn` / `ExpiresAt` 传输正确

## 影响范围

| 层 | 文件 | 改动量 |
|---|---|---|
| 常量 | `internal/common/constant/share.go` | 替换常量名 |
| DTO | `internal/dto/session_share.go` | `CreateShareReqBody` 加字段 |
| Cache 接口 | `internal/infrastructure/cache/share.go` | `CreateShare` 加参数 |
| Cache 实现 | `internal/infrastructure/cache/share.go` | TTL 动态化，ListUserShares 适配 |
| Handler | `internal/handler/session.go` | 新增 `parseExpiresIn`，调用传参 |
| 前端组件 | `web/src/components/share/share-dialog.tsx` | 新增过期选择器 |
| 前端类型 | `web/src/lib/types.ts` | 更新 `CreateShareReqBody` |
| 单元测试 | `test/unit/session_share/` | 新增/更新测试 |
