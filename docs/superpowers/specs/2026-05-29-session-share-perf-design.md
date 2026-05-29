# Session Share 页面性能优化设计

## 背景

Session Detail 页面已通过拆分接口做了性能优化（metadata + 分页 messages + 分页 tools），但 Session Share 页面仍使用单一的 `getShareContent` 接口一次性返回全部 messages 和 tools。对于消息量大的 session，首屏加载慢且无法分页。

## 目标

参照 Session Detail 的优化模式：
1. 将 `getShareContent` 拆分为 metadata / message list / tool list 三个公开分页接口
2. 前端使用 `useInfiniteList` 实现滚动加载
3. 抽取共享组件减少 session-detail 和 share 页面之间的重复代码
4. 删除旧 `getShareContent` 接口（已被拆分接口完全替代）

## 后端 API 变更

### 新增公开接口

| 接口 | Method | Path | 鉴权 |
|------|--------|------|------|
| Share metadata | GET | `/api/v1/session/share/metadata` | IP 限流（独立桶） |
| Share messages | GET | `/api/v1/session/share/message/list` | IP 限流（独立桶） |
| Share tools | GET | `/api/v1/session/share/tool/list` | IP 限流（独立桶） |

全部请求通过 `?id=<shareId>` 查询参数传入 shareId。

### 处理流程

1. `shareCache.GetShareSessionID(shareId)` 解析 shareId → sessionID（失败返回 404）
2. 调用已有 `getMetaByUser` / `listMessages` / `listTools` 查询（`UserID: 0, IsAdmin: true`）
3. 返回不含敏感字段（apiKeyName）的响应

### DTO

```go
// ShareSessionMetadata 分享 Session 元数据（不含敏感字段）
type ShareSessionMetadata struct {
    ID           uint              `json:"id"`
    CreatedAt    time.Time         `json:"createdAt"`
    UpdatedAt    time.Time         `json:"updatedAt"`
    Metadata     map[string]string `json:"metadata,omitempty"`
    MessageCount int               `json:"messageCount"`
    ToolCount    int               `json:"toolCount"`
}

// GetShareMetadataReq / GetShareMetadataRsp
// ListShareMessagesReq / ListShareMessagesRsp
// ListShareToolsReq / ListShareToolsRsp
// 响应结构与 ListSessionMessagesRsp / ListSessionToolsRsp 一致
```

### 路由注册

在 `initSessionPublicRouter` 中注册 3 条路由，各自使用独立的 IP 限流桶。

### 删除的接口

`getShareContent`（`GET /api/v1/session/share?id=xxx`）已删除，被三个分页接口完全替代。

## 前端变更

### api-client.ts 变更

- 删除 `getShareContent(shareId)` 方法
- 新增 `getShareMetadata(shareId)` → 公开请求，不带 Authorization
- 新增 `listShareMessages(shareId, page, pageSize)` → 公开请求
- 新增 `listShareTools(shareId, page, pageSize)` → 公开请求

### types.ts 变更

- 删除 `ShareContentSessionDetail`、`GetShareContentRsp`
- 新增 `ShareSessionMetadata`、`GetShareMetadataRsp`
- 新增 `ListShareMessagesRsp`、`ListShareToolsRsp`

### Share 页面重构

1. 首屏只拉 `getShareMetadata` → 渲染 header + 骨架屏
2. messages 用 `useInfiniteList` 分页加载
3. tools 用 `useInfiniteList`，桌面端面板打开 / 移动端 sheet 打开时才启用

### 共享组件抽取

从 `session-detail-client.tsx` 导出 `CollapsibleText` 和 `ToolSidebarItem`，share 页面引用而非内联副本。

## 不做的事

- 不修改 JWT 分页接口
