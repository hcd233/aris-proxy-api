# API Key 管理模块设计

## 1. 概述

为系统增加用户维度的 API Key 管理功能，支持创建、列出、删除 API Key。用户通过 JWT 鉴权，权限为 `user` 及以上可操作 API Key。

## 2. 背景与目标

- **现状**：`ProxyAPIKey` 模型已存在，但无用户维度隔离，所有 key 全局共享
- **目标**：实现用户维度的 API Key 管理，支持创建、列出、删除操作

## 3. 数据模型变更

### 3.1 ProxyAPIKey 模型新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `UserID` | `uint` | 所属用户 ID，新增索引 |

```go
type ProxyAPIKey struct {
    BaseModel
    ID     uint   `json:"id" gorm:"column:id;primary_key;auto_increment"`
    UserID uint   `json:"userId" gorm:"column:user_id;not null;index;comment:所属用户ID"`
    Name   string `json:"name" gorm:"column:name;uniqueIndex;not null;comment:密钥名称"`
    Key    string `json:"key" gorm:"column:key;uniqueIndex;not null;comment:API密钥值"`
}
```

### 3.2 GORM 迁移

通过 GORM AutoMigrate 自动迁移，新增 `user_id` 字段和索引。

## 4. 接口设计

### 4.1 创建 API Key

- **路径**：`POST /api/v1/apikey`
- **认证**：JWT
- **权限**：`user` 及以上
- **请求体**：
  ```json
  {
    "name": "My API Key"
  }
  ```
- **响应**（201 Created）：
  ```json
  {
    "data": {
      "id": 1,
      "name": "My API Key",
      "key": "sk-aris-xxxxxxxxxxxx",
      "createdAt": "2026-04-08 10:00:00"
    }
  }
  ```
- **注意**：`key` 字段仅在创建响应中返回完整值，后续列表接口只返回 masked 值

### 4.2 列出 API Key

- **路径**：`GET /api/v1/apikey`
- **认证**：JWT
- **权限**：`user` 及以上
- **响应**（200 OK）：
  ```json
  {
    "data": [
      {
        "id": 1,
        "name": "My API Key",
        "key": "sk-ar***xxxx",
        "createdAt": "2026-04-08 10:00:00"
      }
    ]
  }
  ```
- **业务规则**：
  - 普通用户：只能看到自己创建的 key
  - 管理员：可以看到所有用户的 key

### 4.3 删除 API Key

- **路径**：`DELETE /api/v1/apikey/:id`
- **认证**：JWT
- **权限**：`user` 及以上
- **响应**（200 OK）：
  ```json
  {
    "data": null
  }
  ```
- **业务规则**：
  - 普通用户：只能删除自己创建的 key
  - 管理员：可以删除任何用户的 key
- **删除方式**：软删除（设置 `deleted_at` 时间戳）

## 5. 业务规则

| 规则 | 说明 |
|------|------|
| Key 格式 | `sk-aris-` 前缀 + 24 字符随机字符串（base62），总长 32 字符 |
| 数量限制 | 每位用户最多创建 **5 个** API Key |
| 数量限制检查 | 创建前检查用户已有未删除 key 数量 |
| 名称唯一性 | 同一用户内 name 必须唯一（`uniqueIndex:idx_name_user`） |
| 软删除 | 删除时设置 `deleted_at`，列表时自动过滤已删除记录 |
| Key 唯一性 | Key 全局唯一（已有 `uniqueIndex`） |

## 6. Key 生成算法

1. 生成 24 字符的 cryptographically secure 随机字符串（base62 字符集：`a-zA-Z0-9`）
2. 拼接前缀 `sk-aris-`
3. 最终格式：`sk-aris-{24随机字符}`，共 32 字符

## 7. 项目结构

```
internal/
├── handler/
│   └── apikey.go              # API Key Handler（新增）
├── service/
│   └── apikey.go              # API Key Service（新增）
├── router/
│   └── apikey.go              # API Key 路由注册（新增）
├── dto/
│   └── apikey.go              # API Key 请求/响应 DTO（新增）
```

## 8. 实现要点

### 8.1 路由注册

在 `router/router.go` 中新增路由组：

```go
apikeyGroup := huma.NewGroup(v1Group, "/apikey")
initAPIKeyRouter(apikeyGroup)
```

### 8.2 中间件

- JWT 认证：复用 `middleware.JwtMiddleware()`
- 权限校验：复用 `middleware.LimitUserPermissionMiddleware`

### 8.3 错误处理

| 场景 | 错误码 |
|------|--------|
| Key 数量超限 | `ErrQuotaExceeded`（新建） |
| Key 不存在或无权限 | `ErrDataNotExists` |
| 重复的 Key 名称 | `ErrAlreadyExists` |
| 数据库错误 | `ErrDBQuery` / `ErrDBCreate` / `ErrDBDelete` |

### 8.4 上下文注入

- 从 `CtxKeyUserID` 获取当前用户 ID
- 从 `CtxKeyPermission` 获取当前用户权限

## 9. 测试要点

| 测试场景 | 说明 |
|---------|------|
| 创建 Key | 正常创建、name 重复、数量超限（≥5） |
| 列出 Key | 普通用户只返回自己的、admin 返回所有 |
| 删除 Key | 正常删除、无权限删除不存在的 Key、admin 删除他人 Key |
| 软删除 | 已删除的 Key 不在列表中显示 |
| Key 格式 | 验证格式 `sk-aris-[24字符]`、唯一性 |
