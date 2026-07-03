# Web 前端契约

> **使用场景**：修改 `web/` 目录下的前端代码时加载。

## 项目模型

- 位置：仓库根目录 `web/`，独立 npm 工程，不参与 Go module。
- 技术栈：Next.js `16.2.6`（App Router）+ React `19` + TypeScript + Tailwind v4 + shadcn/ui（`base-nova` 风格）+ `@base-ui/react` + `lucide-react` + `sonner`。
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
- 后端开发地址：`API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL`。本地联调先 `go run main.go server start --host localhost --port 8080`，再设置 `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` 后 `npm run dev`。
- 鉴权：登录态由 `AuthProvider` 维护；需要管理员能力的页面用 `<PermissionGuard role="admin">` 包裹，不要自行重复判断 token。
- UI 组件：优先复用 `src/components/ui/` 里的 shadcn 组件；新增基础组件用 `npx shadcn add <name>` 生成，不要手写散件。
- 样式：仅使用 Tailwind v4 + `cn()` 组合 class，禁止内联 `style` 写定值；颜色走 `globals.css` 中 CSS 变量 + `neutral` baseColor，避免硬编码 hex。
- 图标：统一 `lucide-react`，不要混用其他图标库。
- Toast：用 `sonner` 的 `toast.*`，禁止 `alert/confirm`。
- 路径前缀：所有内部跳转链接必须考虑 `basePath=/web`；用 `next/link`、`next/navigation` 即可，框架自动加前缀，**不要手动拼 `/web` 前缀**。
- 修改前端 DTO 时如发现后端字段缺失，按 `huma-dto-conventions` 流程到 `internal/dto/` 同步更新。

## i18n 布局稳定性契约

> 详见 [docs/adr/0005-i18n-layout-stability.md](../adr/0005-i18n-layout-stability.md) 与 [web/CONTEXT.md](../../web/CONTEXT.md)。切换语言（en/zh/ja）时组件不得发生宽高跳变。

### Category Reserve（刚性元素宽度预留）

新增/修改以下组件类别时，必须按类别预留 `min-w`，使跨语言不位移：

- **按钮**（`Button` 文本尺寸）：`default` → `min-w-20`、`sm` → `min-w-16`、`lg` → `min-w-24`、`xs` → `min-w-14`。已在 `button.tsx` size variants 内置，新增 size 沿用。
- **分页触发器**：显示动态文本（如 `{n} per_page`）的 `DropdownMenuTrigger` 按钮加 `min-w-[7.5rem]`。
- **侧边栏导航项**：不需要 `min-w`（侧边栏容器 `w-64` 已定宽，吸收位移）。

个别超长翻译可在调用点额外加 `min-w-[Nrem]`，不要为单条翻译改全局类别值。

### Layout-Pattern Height Fix（布局高度稳定）

- **表格**：`<th>` 保持 `whitespace-nowrap`（已内置）；单行长内容单元格用 `max-w-[Nch] truncate` + `title=` 提示，不要强制行高。
- **卡片网格**：`grid` 默认 `items-stretch` 已等高；卡片描述用 `line-clamp-2` 限两行。
- **对话框正文**：显示动态长度描述的 `DialogDescription` 加 `min-h-[2.5rem]`（约两行）预留；自由描述文本不加 `min-h`。

### Font Scale（CJK 字号对齐）

`globals.css` 的 `:lang(zh)/:lang(ja)` 块覆盖 Tailwind v4 `--text-*` 主题变量，等比下调 CJK 字号（zh 0.92、ja 0.88），仅动字号不动 rem 间距。新增 text utility 档位时同步在两个 `:lang()` 块补对应 `--text-*` 覆盖。预留 `min-w` 应以缩放后的 CJK 宽度为准核定。

### 切换闪烁

`<LocaleFade>` 包裹 dashboard `<main>` 内 `max-w-6xl` 容器与 share 页根，监听 `locale` 变化做 ~120ms opacity 淡入。新增会因切换语言而 reflow 的页面根容器时，用 `<LocaleFade>` 包裹。不要在 `width`/`height` 上加 CSS transition，不要引入 View Transitions API。

## 联调与发布

- 本地完整链路：先后端 `go run main.go server start ...`，再 `cd web && npm run dev`，浏览器访问 `http://localhost:3000/web`。
- 生产路径：`make build` → 镜像里 Go 二进制内置 `internal/web/dist/`，浏览器访问 `https://<host>/web/`。
- CI：`.github/workflows/docker-publish.yml` 的 path filter **不包含** `web/**`，所以纯前端改动不会触发镜像重建；纯前端发布需要触发后端文件改动或在 PR 描述中说明，必要时手工触发 workflow。
- 测试：当前没有强制的前端单测/e2e 框架；改动后至少运行 `cd web && npm run lint && npm run build` 验证类型与导出能成功。
- 提交：前端改动同样遵循 `.worktrees/` + `feature|bugfix|refactor|chore|docs|test|hotfix/...-YYYY-MM-DD` 分支规范；与后端联动的功能尽量在同一个 PR 中提交，避免接口前后不一致。
