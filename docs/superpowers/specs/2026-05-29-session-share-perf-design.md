# Session Share 页面性能优化设计

## 背景

Session Detail 页面已通过拆分接口做了性能优化（metadata + 分页 messages + 分页 tools），但 Session Share 页面仍使用单一的 `getShareContent` 接口一次性返回全部 messages 和 tools。对于消息量大的 session，首屏加载慢且无法分页。

## 目标

参照 Session Detail 的优化模式：
1. 将 `getShareContent` 拆分为 metadata / message list / tool list 三个公开分页接口
2. 前端使用 `useInfiniteList` 实现滚动加载
3. 抽取共享组件减少 session-detail 和 share 页面之间的重复代码

## 后端 API 变更

### 新增公开接口

| 接口 | Method | Path | 鉴权 |
|------|--------|------|------|
| Share metadata | GET | `/api/v1/session/share/metadata` | IP 限流 |
| Share messages | GET | `/api/v1/session/share/message/list` | IP 限流 |
| Share tools | GET | `/api/v1/session/share/tool/list` | IP 限流 |

全部请求通过 `?id=<shareId>` 查询参数传入 shareId，与现有 `getShareContent` 一致。

### 处理流程

1. `shareCache.GetSessionID(shareId)` 解析 shareId → sessionID（失败返回 404）
2. 调用已有 `getMetaByUser` / `listMessages` / `listTools` 查询（`SkipOwnershipCheck: true`）
3. 返回不含敏感字段（apiKeyName）的响应

### DTO

```go
// GetShareMetadataReq
type GetShareMetadataReq struct {
    ShareID string `query:"id" required:"true" doc:"分享ID"`
}

// GetShareMetadataRsp
type GetShareMetadataRsp struct {
    CommonRsp
    ID          uint              `json:"id"`
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    MessageCount int              `json:"messageCount"`
    ToolCount    int              `json:"toolCount"`
}

// ListShareMessagesReq / ListShareToolsReq
// 复用 model.PageParam，加 ShareID query 参数
// 响应结构与 ListSessionMessagesRsp / ListSessionToolsRsp 一致
```

### 路由注册

在 `initSessionPublicRouter` 中注册 3 条路由，复用已有 IP 限流中间件。

### 旧接口兼容

`getShareContent`（`GET /api/v1/session/share/?id=xxx`）保留不删，标记 deprecated。

## 前端变更

### api-client.ts 新增方法

- `getShareMetadata(shareId)` → 公开请求，不带 Authorization
- `listShareMessages(shareId, page, pageSize)` → 公开请求
- `listShareTools(shareId, page, pageSize)` → 公开请求

### types.ts 新增 TS interface

- `ShareMetadata`、`GetShareMetadataRsp`
- `ListShareMessagesRsp`、`ListShareToolsRsp`

### Share 页面重构

1. 首屏只拉 `getShareMetadata` → 渲染 header + 骨架屏
2. messages 用 `useInfiniteList` 分页加载
3. tools 用 `useInfiniteList`，桌面端面板打开 / 移动端 sheet 打开时才启用

### 共享组件抽取

从 `session-detail-client.tsx` 导出 `CollapsibleText` 和 `ToolSidebarItem`，share 页面引用而非内联副本。

## 不做的事

- 不删除旧 `getShareContent` 接口
- 不修改 JWT 分页接口