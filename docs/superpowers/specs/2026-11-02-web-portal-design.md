# Web 门户页与全局偏好控件设计

- 日期：2026-11-02
- 分支：`feature/web-portal-2026-11-02`（worktree `.worktrees/web-portal`）
- 范围：仅 `web/` 前端，后端零改动

## 1. 意图

目前 `https://<host>/web/` 一打开就是仪表盘，未登录的人会直接被踢到登录页，看不到 Aris 是什么。这次要加一个门户页，让第一次来的访客：

1. 看懂 Aris 是什么产品、能做什么；
2. 能去登录，也能跳到 GitHub 看源码。

同时，主题和语言要做成全局控件，在门户、登录、仪表盘等页面都能随时切换。

## 2. 目标 / 非目标 / 反目标

**目标**
- `/web/` 改为门户页（公开访问，不需要登录）。
- 原来的仪表盘首页（「概览 · 仪表盘」tab）移到 `/web/dashboard/`。
- 每个页面都能切换主题（anthropic / moonshot）和语言（en / zh / ja），控件只有一套实现。

**非目标**
- 其他管理页的路由不变：`/web/sessions/`、`/web/apikeys/`、`/web/upstream/` 等保持原路径。`/login`、`/callback`、`/share` 也不变。
- 不改后端（`internal/router/webfs.go` 的 `/web/*` 静态托管与 index 回落已能覆盖新增的 `dashboard/index.html`）。
- 不做文档站、定价页、注册流程或博客。

**反目标（即使功能能用也算失败）**
- 门户在 moonshot 主题或 en/ja 下出现硬编码颜色、文案缺失或回落成 key。
- 切换语言时门户布局发生宽高跳变（沿用 `docs/agents/web-frontend.md` 的 i18n 契约）。
- 已登录用户打开 `/web/` 被强制跳走，或找不到进入控制台的入口。
- 旧链接或书签（如 `/web/sessions/`）失效。

## 3. 已定方案（用户确认）

### 3.1 门户布局：方案 C「链路示意 + 三大支柱」

样稿见 `.superpowers/brainstorm/*/content/portal-layout.html` 的选项 C。自上而下：

1. **顶栏**：左侧品牌（与登录页同款的 A 字标 + "Aris"）；右侧依次是全局偏好控件（语言、主题）、GitHub 图标链接、主按钮。
2. **首屏**：一句标语、一句副标题（「模型聚合网关 + 使用数据沉淀平台」），下面两个按钮：
   - 主按钮「登录控制台」，指向 `/login/`；
   - 次按钮「GitHub 源码 ↗」，指向 `https://github.com/hcd233/aris-proxy-api`，新标签打开，带 `rel="noopener noreferrer"`。
3. **链路示意**：左列是客户端（Claude Code、Codex、OpenCode 等），中间是高亮的 Aris 节点（鉴权 · 路由 · 熔断 · 审计），右列是上游（OpenAI / Anthropic、各家 MaaS、订阅账号）。用 HTML/CSS 或内联 SVG 绘制，只用主题 token，两个主题下都要协调。
4. **三大支柱卡片**，每张卡顶部有一条色带：
   - **接入**：模型聚合与别名、OpenAI/Anthropic 协议互转（含 SSE）、订阅账号 OAuth（Claude Code / Codex）、`aris model export` 客户端配置导出；
   - **沉淀**：会话聚合、checksum 去重、评分与分享、ShareGPT/SFT 导出；
   - **治理**：调用审计与运行时监控、四级权限 + Demo 账户、限流、触发词拦截。
5. **页脚**：产品名 + GitHub 链接。

文案以 `README.md` 的 Core Capabilities 为准，提炼成短句。中英日三语都要写全。

### 3.2 已登录状态

门户页不自动跳转。已登录时（`useAuth().user` 存在），主按钮文案变为「进入控制台」，指向 `/dashboard/`。认证加载期间先显示「登录控制台」，避免按钮闪烁。

### 3.3 路由调整

- `app/(dashboard)/page.tsx` 移到 `app/(dashboard)/dashboard/page.tsx`，导出产物为 `out/dashboard/index.html`。
- 新增 `app/page.tsx`（门户），放在 `(dashboard)` 路由组外，因此不带侧边栏和 `PermissionGuard`。
- 侧边栏导航「仪表盘」的 href 从 `/` 改为 `/dashboard/`；`isItemActive` 去掉 `href === "/"` 的特判，改为统一的前缀匹配。
- 登录后的落点从 `/web/` 改为 `/web/dashboard/`：`app/login/page.tsx` 3 处、`app/callback/page.tsx` 1 处。如有其他「回首页」类跳转，也一并核对。
- `app/share/page.tsx` 头部的 A 字标 `href="/"` 保持不变，现在会指向门户，符合「了解产品」的语义。

### 3.4 全局偏好控件

现状：登录页右上角已有「语言 + 主题」一组（#218）；分享页和回调页只有左下角的主题悬浮球；仪表盘侧边栏底部有一组，但侧边栏折叠后会隐藏。

方案：
- 新建 `PreferenceControls` 组件（语言下拉 + 主题切换），作为唯一实现。
- 门户：放在顶栏右侧。
- 登录：替换现有右上角的内联组合。
- 回调、分享：用它替换左下角悬浮球，放在页面右上角。分享页如果已有头部，就放进头部右侧，不能和工具按钮重叠。
- 仪表盘：侧边栏展开时位置不变；**折叠时也要能切换**，可以竖排图标，或收进用户头像菜单。
- 去掉 `ThemeSwitcher` 的 FAB 变体，以及根布局里的全局挂载。
- 现在的 inline 变体写死了 `text-sidebar-foreground` 等侧边栏配色，需要让它在普通页面背景上也能用（例如增加 tone 参数，或改用中性 token）。

## 4. 约束

- Next.js 静态导出（`output: "export"`、`basePath: "/web"`、`trailingSlash: true`）。门户是纯客户端组件，只读 auth 状态。
- 只用主题 token 和既有工具类，不写死颜色。两个主题都要人工核对。
- 动效沿用现有体系（`animate-rise`、`stagger-rise`、motion tier），并遵守 `prefers-reduced-motion`。
- 响应式：390 宽的手机竖屏下，链路图改为纵向堆叠，三张卡片改为单列。
- 截断文案遵守 `truncate-requires-tooltip` lint 规则。
- 外链只有 GitHub 一个，且是写死的常量。门户不发任何额外 API 请求（auth 状态复用 `AuthProvider`）。

## 5. 交给实现者决定

- 标语的具体措辞（中文参考：「一个地址，接入所有大模型」）。
- 链路图用 CSS 还是 SVG、节点数量，以及是否加一条低调的流动动效。
- 折叠侧边栏里偏好控件的具体形态。
- 新增的 i18n key 命名（建议用 `portal.*` 前缀）。
- 是否给 `isItemActive` 补单测（推荐补，因为根路径特判被移除了）。
- 同步更新 `docs/agents/web-frontend.md` 中关于路由组的描述。

## 6. 验收

- `npm run lint`、`npm run format:check`、`npm run test`、`npx next build --webpack` 全部通过。
- 导出产物里有 `out/index.html`（门户）和 `out/dashboard/index.html`。
- 用 chrome-headless-shell 无头截图或 DOM 断言检查：门户在两个主题 × 三种语言下文案完整、无横向溢出；视口覆盖 1440 和 390 两档。
- 手工路径验证：
  - 未登录：`/web/` 显示门户 → 登录 → 落在 `/web/dashboard/`。
  - 已登录：`/web/` 的按钮显示「进入控制台」。
  - 侧边栏「仪表盘」能正确高亮。
  - `/web/sessions/` 等旧路径正常。
  - 每个页面（含折叠侧边栏）都能切换主题和语言。
