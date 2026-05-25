# aris-proxy-api 管理前端设计文档

## 概述

为 aris-proxy-api 开发管理前端页面，嵌入 Go 二进制同源部署，支持 OAuth2 登录、角色权限控制、会话历史查看和 admin 级别的 Endpoint/Model 配置。

## 技术选型

| 项目 | 选择 | 理由 |
|------|------|------|
| 框架 | Next.js App Router (output: 'export') | 静态导出可嵌入 Go 二进制 |
| UI 组件 | shadcn/ui + Tailwind CSS | 可定制性强，视觉效果好 |
| 渲染模式 | 纯客户端渲染 (SPA) | 嵌入方案无法使用 SSR |
| 仓库位置 | 同仓库 `web/` 子目录 | 前后端统一管理 |
| 部署方式 | Go embed.FS 嵌入，Fiber 提供静态文件服务 | 同源无 CORS，单一 Docker 镜像 |

## 架构

### 请求流向

```
浏览器 → api.lvlvko.top/web/*   →  Fiber Static (/web/)  →  嵌入的 Next.js 静态文件
浏览器 → api.lvlvko.top/api/*   →  Fiber API Routes      →  Go 后端处理
浏览器 → api.lvlvko.top/oauth2  →  后端 OAuth2 流程      →  重定向回 /web/
```

### 目录结构

```
aris-proxy-api/
├── web/                          # Next.js 前端项目
│   ├── src/
│   │   ├── app/                  # App Router 页面
│   │   │   ├── login/            # 登录页
│   │   │   ├── auth/             
│   │   │   │   └── callback/     # OAuth2 回调中转页
│   │   │   ├── dashboard/        # 仪表盘
│   │   │   ├── sessions/         # 会话列表
│   │   │   │   └── [id]/         # 会话详情（气泡式对话）
│   │   │   ├── apikeys/          # API Key 管理
│   │   │   ├── admin/
│   │   │   │   ├── endpoints/    # Endpoint 配置（admin only）
│   │   │   │   └── models/       # Model 配置（admin only）
│   │   │   └── profile/          # 个人资料
│   │   ├── components/           # 通用组件 + shadcn/ui
│   │   ├── lib/                  # API 客户端、auth、工具函数
│   │   └── hooks/                # 自定义 React hooks
│   ├── package.json
│   ├── next.config.ts            # output:'export', basePath:'/web'
│   └── tailwind.config.ts
├── internal/
│   ├── web/                      # embed.FS + Fiber static 路由
│   ├── handler/                  # 新增 endpoint/model handler
│   └── ...
└── Makefile                      # 新增 web-build 目标
```

## 认证流程

```
1. 用户访问 /web/login → 点击 GitHub/Google 按钮
2. 前端跳转到 /api/v1/oauth2/login?platform=github
3. 后端生成 state → 重定向到 GitHub/Google 授权页
4. 用户授权 → GitHub/Google 回调到 /api/v1/oauth2/callback
5. 后端验证 → 创建/更新用户 → 生成 JWT pair
6. 后端重定向到 /web/auth/callback?access_token=xxx&refresh_token=xxx
7. 前端 auth callback 页存储 JWT 到 localStorage → 跳转 dashboard
8. 后续请求携带 Authorization: Bearer <access_token>
9. Token 过期 → 用 refresh_token 调 /api/v1/token/refresh
```

## 权限路由守卫

| 角色 | 可访问页面 | 不可访问 |
|------|-----------|---------|
| `pending` | /web/login | 其他所有页面 → 显示「等待审核」提示 |
| `user` | dashboard, sessions, sessions/[id], apikeys, profile | admin/* |
| `admin` | 所有页面 | 无 |

前端通过 `/api/v1/user/current` 获取用户信息后，根据 `permission` 字段做路由守卫。

## 核心页面

### 登录页 /web/login
- 两个按钮：GitHub 登录、Google 登录
- 点击后跳转 `/api/v1/oauth2/login?platform=xxx`
- 未登录用户访问其他页面时自动重定向到此处

### 仪表盘 /web/dashboard
- 用户欢迎信息 + 角色标识
- 快捷入口卡片：会话历史、API Key 管理
- Admin 额外看到：Endpoint 管理、Model 管理入口

### 会话列表 /web/sessions
- 分页表格：会话摘要、API Key 名称、评分、时间
- 点击行进入会话详情

### 会话详情 /web/sessions/[id]
- 对话气泡布局：用户消息右对齐、AI 回复左对齐
- Markdown 渲染 AI 回复内容
- 顶部显示会话元信息（模型、评分等）

### Endpoint 管理 /web/admin/endpoints（admin only）
- CRUD 表格：名称、OpenAI Base URL、Anthropic Base URL、能力开关
- 新建/编辑弹窗表单

### Model 管理 /web/admin/models（admin only）
- CRUD 表格：别名、上游模型名、关联 Endpoint
- 新建/编辑弹窗表单，Endpoint 下拉选择

### API Key 管理 /web/apikeys
- 列表展示 Key 名称、前缀、创建时间
- 创建弹窗（创建后仅显示一次完整 Key）
- 删除确认弹窗

## 前端数据流

```
App 入口
  ├─ AuthProvider（Context）
  │    ├─ 登录状态、用户信息、JWT token
  │    ├─ 自动刷新 token（过期前 5 分钟）
  │    └─ 未登录 → 重定向 /web/login
  ├─ PermissionGuard（组件）
  │    ├─ user 角色 → 拦截 admin/* 路由
  │    └─ pending 角色 → 显示等待审核页
  └─ API Client（fetch 封装）
       ├─ 自动附加 Authorization header
       ├─ 401 响应 → 尝试 refresh → 失败则跳转登录
       └─ 基础 URL: 同源 /api/v1
```

## 后端嵌入方案

```go
// internal/web/static.go
//go:embed all:dist
var webDist embed.FS

func RegisterWebRoutes(app *fiber.App) {
    fs, _ := fs.Sub(webDist, "dist")
    app.Use("/web", filesystem.New(filesystem.Config{
        Root:       http.FS(fs),
        Index:      "index.html",
        Browse:     false,
    }))
    // SPA fallback: /web/* 非 API 路径回退到 index.html
    app.Use("/web/*", func(c *fiber.Ctx) error {
        return c.SendFile("index.html")
    })
}
```

注意：Go `//go:embed` 不支持 `../` 路径，因此 `internal/web/dist/` 目录由构建步骤从 `web/out/` 复制而来，并在 `.gitignore` 中忽略。

Makefile 新增：
```makefile
web-build: ## 构建前端静态文件
	cd web && npm ci && npm run build
	rm -rf internal/web/dist && cp -r web/out internal/web/dist

build: web-build ## 先构建前端再构建 Go 二进制
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/aris-proxy-api .
```

## 需要的后端改动

| 改动 | 文件 | 说明 |
|------|------|------|
| 新增 Endpoint CRUD handler | `internal/handler/endpoint.go` | 遵循现有 handler 模式 |
| 新增 Model CRUD handler | `internal/handler/model.go` | 遵循现有 handler 模式 |
| 新增 Endpoint usecase | `internal/application/endpoint/` | CQRS command/query |
| 新增 Model usecase | `internal/application/model/` | CQRS command/query |
| 新增 Endpoint/Model DTO | `internal/dto/endpoint.go`, `model.go` | 请求/响应结构 |
| 新增 Endpoint/Model repository | `internal/infrastructure/database/` | 数据访问层 |
| 注册新路由 | `internal/router/router.go` | `/api/v1/endpoint/`, `/api/v1/model/` |
| 注册新依赖 | `internal/bootstrap/container.go` | dig 注入 |
| 嵌入前端静态文件 | `internal/web/static.go` | embed.FS + Fiber static，embed 路径为 `dist/`（构建时从 `web/out/` 复制到 `internal/web/dist/`） |
| OAuth2 回调重定向改造 | `internal/handler/oauth2.go`, `internal/application/oauth2/command/handle_callback.go` | 回调成功后重定向到 `/web/auth/callback?access_token=xxx&refresh_token=xxx`，而非直接返回 JSON |
| OAuth2 重定向 URL 更新 | `env/api.env.template` | OAuth2 redirect URL 指向 `/api/v1/oauth2/callback`，前端回调 URL 为 `/web/auth/callback` |
| Session API 权限确认 | `internal/router/router.go` | 确保 session 用 jwtAuth |

## 新增后端 API 端点

| 端点 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/api/v1/endpoint/` | GET | admin | 列出所有 endpoint |
| `/api/v1/endpoint/` | POST | admin | 创建 endpoint |
| `/api/v1/endpoint/{id}` | PATCH | admin | 更新 endpoint |
| `/api/v1/endpoint/{id}` | DELETE | admin | 删除 endpoint |
| `/api/v1/model/` | GET | admin | 列出所有 model |
| `/api/v1/model/` | POST | admin | 创建 model |
| `/api/v1/model/{id}` | PATCH | admin | 更新 model |
| `/api/v1/model/{id}` | DELETE | admin | 删除 model |

## 不在范围内

- 前端不做 LLM 代理调用界面
- 不做用户管理界面（admin 修改其他用户角色）
- 不做审计日志前端页面
- 不做国际化（仅中文）
- 不做 Vercel 部署
