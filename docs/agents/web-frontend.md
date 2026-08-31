# Web 前端契约

> **使用场景**：修改 `web/` 目录下的前端代码时加载。

## 项目模型

- 位置：仓库根目录 `web/`，独立 npm 工程，不参与 Go module。
- 技术栈：Next.js `16.3.3`（App Router，Turbopack）+ React `19` + TypeScript + Tailwind v4 + shadcn/ui（`base-nova` 风格）+ `@base-ui/react` + `lucide-react` + `sonner`。
- 关键配置（`web/next.config.ts`）：`output: "export"` 静态导出、`basePath: "/web"`、`trailingSlash: true`、`images.unoptimized: true`。前端最终被 Go 后端 embed 服务，**不要改 basePath，也不要引入需要 SSR/Edge 的依赖**。
- 后端 embed 路径：`internal/web/static.go` 里 `//go:embed all:dist`；构建时 `make web-build` 把 `web/out/` 拷贝到 `internal/web/dist/`（已 `.gitignore`）。线上请求由 `internal/router/web.go` 的 `/web/*` 处理，找不到的非 `_next/`/`static/` 路径回落 `index.html`。
- 注意：`web/AGENTS.md` 已声明这是新版 Next.js，与训练数据有差异；动手前优先读 `node_modules/next/dist/docs/` 中的对应章节，并尊重弃用提示。

## 目录结构

- `src/app/layout.tsx`：全局布局，挂载 `AuthProvider` 和 `Toaster`。
- `src/app/login/`、`src/app/callback/`：OAuth2 登录入口与回调。
- `src/app/(dashboard)/`：登录后的管理后台路由组，包含 `apikeys/`、`endpoints/`、`models/`、`profile/`、`sessions/`、`shares/` 子页面，以及 `layout.tsx`、`page.tsx`。
- `src/app/share/`：会话分享只读页面（公开访问）。
- `src/components/ui/`：shadcn 生成的基础组件，遵循其约定，禁止手改命名/导出方式。
- `src/components/chat/`、`src/components/session-detail/`、`src/components/share/`：业务组件按页面切分。
- `src/components/permission-guard.tsx`：基于 `auth-context` 的角色守卫。
- `src/lib/api-client.ts`：统一 fetch 封装，自动带 `Authorization: Bearer <access_token>` 并处理 401 → `/api/v1/token` 刷新。
- `src/lib/auth-context.tsx`：登录态、`isAdmin/isUser` 判断、`access_token` / `refresh_token` 存储。
- `src/lib/types.ts`：与后端 huma DTO 对应的 TS 类型；后端 DTO 改动时必须同步。
- `src/hooks/use-mobile.ts`：响应式断点 hook。
- `src/lib/utils.ts`：`cn`、通用辅助函数；新增公共 helper 一律放这里，禁止散落业务文件。

## 开发契约

- 路由别名：使用 `@/components`、`@/lib`、`@/hooks`、`@/components/ui`（见 `components.json`），不要写相对路径回溯。
- 调用后端：所有 HTTP 调用统一走 `src/lib/api-client.ts` 暴露的 `api.*` 方法，**禁止**业务组件里直接 `fetch`；新增接口时同步在 `types.ts` 增补 `XxxReq` / `XxxRsp`，命名与后端 huma DTO 一致。
- 后端开发地址：`API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL`。本地联调先 `go run ./cmd/server server start --host localhost --port 8080`，再设置 `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` 后 `npm run dev`。
- 鉴权：登录态由 `AuthProvider` 维护；需要管理员能力的页面用 `<PermissionGuard role="admin">` 包裹，不要自行重复判断 token。
- UI 组件：优先复用 `src/components/ui/` 里的 shadcn 组件；新增基础组件用 `npx shadcn add <name>` 生成，不要手写散件。
- 样式：仅使用 Tailwind v4 + `cn()` 组合 class，禁止内联 `style` 写定值；颜色走 `globals.css` 中 CSS 变量 + `neutral` baseColor，避免硬编码 hex。
- 图标：统一 `lucide-react`，不要混用其他图标库。
- Toast：用 `sonner` 的 `toast.*`，禁止 `alert/confirm`。
- 路径前缀：所有内部跳转链接必须考虑 `basePath=/web`；用 `next/link`、`next/navigation` 即可，框架自动加前缀，**不要手动拼 `/web` 前缀**。
- 修改前端 DTO 时如发现后端字段缺失，按 `huma-dto-conventions` 流程到 `internal/dto/` 同步更新。

## i18n 布局稳定性契约

> 详见 [web/CONTEXT.md](../../web/CONTEXT.md)。切换语言（en/zh/ja）时组件不得发生宽高跳变。

### Category Reserve（刚性元素宽度预留）

新增/修改以下组件类别时，必须按类别预留 `min-w`，使跨语言不位移：

- **按钮**（`Button` 文本尺寸）：`default` → `min-w-20`、`sm` → `min-w-16`、`lg` → `min-w-24`、`xs` → `min-w-14`。已在 `button.tsx` size variants 内置，新增 size 沿用。
- **分页触发器**：显示动态文本（如 `{n} per_page`）的 `DropdownMenuTrigger` 按钮加 `min-w-[7.5rem]`。
- **侧边栏导航项**：不需要 `min-w`（侧边栏容器 `w-64` 已定宽，吸收位移）。

个别超长翻译可在调用点额外加 `min-w-[Nrem]`，不要为单条翻译改全局类别值。

### Layout-Pattern Height Fix（布局高度稳定）

- **表格**：`<th>` 保持 `whitespace-nowrap`（已内置）；单行长内容单元格用 `max-w-[Nch] truncate` + Tooltip 组件展示完整内容，不要强制行高。
- **卡片网格**：`grid` 默认 `items-stretch` 已等高；卡片描述用 `line-clamp-2` 限两行。
- **截断与 Tooltip（lint 强制）**：`truncate` / `line-clamp-1` 元素必须处于 `TooltipTrigger` 渲染子树内（`web/eslint-rules/truncate-requires-tooltip.mjs`，error 级），保证用户可悬停查看完整内容；标准写法 `<TooltipRoot><TooltipTrigger render={<span className="… truncate">…</span>} /><TooltipContent className="max-w-xs break-all">…</TooltipContent></TooltipRoot>`。`line-clamp-2+`（卡片描述）豁免；恒定短占位（`—` 等）直接删截断类，不加 tooltip。
- **对话框正文**：显示动态长度描述的 `DialogDescription` 加 `min-h-[2.5rem]`（约两行）预留；自由描述文本不加 `min-h`。

### Font Scale（CJK 字号对齐）

`globals.css` 的 `:lang(zh)/:lang(ja)` 块覆盖 Tailwind v4 `--text-*` 主题变量，等比下调 CJK 字号（zh 0.92、ja 0.88），仅动字号不动 rem 间距。新增 text utility 档位时同步在两个 `:lang()` 块补对应 `--text-*` 覆盖。预留 `min-w` 应以缩放后的 CJK 宽度为准核定。

### 切换闪烁

`<LocaleFade>` 包裹 dashboard `<main>` 内 `max-w-6xl` 容器与 share 页根，监听 `locale` 变化做 ~120ms opacity 淡入。新增会因切换语言而 reflow 的页面根容器时，用 `<LocaleFade>` 包裹。不要在 `width`/`height` 上加 CSS transition，不要引入 View Transitions API。

## 运行时验证

> 使用 `next-dev-loop` skill（`.agents/skills/external/next-dev-loop/`，来自 vercel/next.js）。`npm run lint && npm run build` 只证明类型与静态导出能成功，不证明页面行为正确——前端改动必须补一轮运行时验证。

### 双视角

- **`/_next/mcp`**：`next dev` 自带的 HTTP 端点，框架视角。本项目验证过的工具：`get_compilation_issues`（Turbopack 编译问题）、`get_errors`（服务端 + 冒泡上来的浏览器错误）、`get_routes`（路由图）、`get_page_metadata`（当前路由由哪些文件渲染，可用来收窄搜索范围）、`get_logs`。回包是 SSE，用 `sed -n 's/^data: //p'` 取 JSON。
- **`agent-browser`**：驱动真实 Chrome，浏览器视角。DOM、console、network、React fiber。运行命令前先跑一次 `agent-browser skills get core` 读版本匹配的用法，不要凭记忆猜子命令（headless 是默认行为，**没有** `--headless` flag）。

两个视角互为交叉校验：两者结论冲突时先怀疑工具链（多为浏览器会话失效），而不是先怀疑业务代码。

### 本项目适配

- **端口**：本项目 dev server 常用非 3000 端口，需相应设置 `NEXT_MCP_URL=http://localhost:<port>/_next/mcp`。
- **URL 必须带 `basePath`**：入口是 `http://localhost:<port>/web/`，不是 `/`。
- **session 隔离**：本项目在 `.worktrees/` 下并行开发，必须用 `--scope worktree` 生成 session id 并 `export AGENT_BROWSER_SESSION` / `AGENT_BROWSER_RESTORE`，否则多 worktree 会互相抢浏览器。
- **登录态**：dashboard 路由受 `AuthProvider` 保护，需 OAuth2 登录。首次由用户在 headed 浏览器里完成登录，`agent-browser close` 会存 cookie，后续 `--restore` 复用。只验证 `/web/login/`、`/web/share/` 等公开页时不需要。
- **不适用的上游 skill**：`next-cache-components-adoption` / `next-cache-components-optimizer` / `next-partial-prefetching-adoption` 都要求服务端渲染 + 流式 Suspense，与本项目 `output: "export"` 静态导出（88/101 个组件是 `"use client"`，数据全部经 `api-client.ts` 在浏览器 fetch）架构冲突，**不要引入**。

## 联调与发布

- 本地完整链路：先后端 `go run ./cmd/server server start ...`，再 `cd web && npm run dev`，浏览器访问 `http://localhost:3000/web`。
- 生产路径：`make build` → 镜像里 Go 二进制内置 `internal/web/dist/`，浏览器访问 `https://<host>/web/`。
- CI：`.github/workflows/build-and-publish.yml` 的 push path filter **包含** `web/**`（与 `internal/**`、`go.mod` 等同级），纯前端改动也会触发构建发布：构建前端 → embed 进 Go 镜像 → 推送到 ghcr → 部署到 K8s。推送到 `master` 或合并 PR 到 `master` 即自动发布，无需手工触发。另有 `.github/workflows/lint.yml` 的 `web-lint` job 跑 `npm run lint` + `npm run format:check`（无 `--max-warnings`，warning 不阻塞）。
- 测试：`cd web && npm run test`（vitest，覆盖 `filter-dsl` 等纯函数与自定义 eslint 规则）。改动后至少跑 `npm run lint && npm run test && npm run build`，再按上文「运行时验证」补一轮 `next-dev-loop`。
- 提交：前端改动同样遵循 `.worktrees/` + `feature|bugfix|refactor|chore|docs|test|hotfix/...-YYYY-MM-DD` 分支规范；与后端联动的功能尽量在同一个 PR 中提交，避免接口前后不一致。
