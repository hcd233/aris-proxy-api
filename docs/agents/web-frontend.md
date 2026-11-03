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
- `src/app/page.tsx`：公开门户页（`/web/`），是一张长页，依次为悬浮导航、Hero、功能卡跑马灯、兼容生态、请求链路、四大能力、接入步骤、收尾 CTA、页脚。
  - 组件在 `src/components/portal/`，按区块拆分；动效守卫用 `use-portal-loop.ts`（档位为 `still` 或页面隐藏时暂停），滚动揭示用 `reveal.tsx`。
  - 演示数据与文案 key 集中在 `src/lib/portal.ts`，模型名一律小写；纯动效逻辑放在 `src/lib/portal-motion.ts`；样式写在 `src/styles/portal.css`。
  - 不挂 `PermissionGuard`，已登录也不自动跳转。
  - 厂商与客户端图标都走 `ProviderIcon`；旁边已有文字时传 `decorative`，对读屏器隐藏图标自带的 `<title>`。
- `src/app/login/`、`src/app/callback/`：OAuth2 登录入口与回调，登录成功落到 `DASHBOARD_HOME_URL`（`/web/dashboard/`，见 `src/lib/nav.ts`）。
- `src/app/(dashboard)/`：登录后的管理后台路由组，仪表盘首页在 `dashboard/page.tsx`（`/web/dashboard/`），其余子页面（`sessions/`、`apikeys/`、`upstream/` 等）路径不带 `dashboard` 前缀；导航高亮统一用 `isNavItemActive`。
- 导航与 ⌘K：侧栏和页面跳转面板（`src/components/nav-command-palette.tsx`，Dialog + Base UI `Autocomplete inline`）共用 `src/lib/nav.ts` 的 `NAV_GROUPS`；新增页面只改这里，`resolveNavGroups` 统一做按身份裁剪与 demo 锁定（面板不列出被锁模块）。`pinned` 分组（仪表盘）不渲染组头、直接铺在顶层，不参与展开逻辑（`findActiveGroupKey` 会跳过）。入口有三处：侧栏搜索按钮、移动端顶栏图标、`⌘K` / `Ctrl+K`，共用 `DashboardLayout` 里同一个 `paletteOpen`。
- 主题/语言切换：唯一实现是 `src/components/preference-controls.tsx`（`tone="page" | "sidebar"`），门户、登录、回调、分享、仪表盘侧栏（含折叠态）都挂它，不要再单独拼 `LanguageSwitcher` + `ThemeSwitcher`。
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
- **空态要有下一步**：列表空态用 `ListEmptyState` 的 `action` 槽位，别做死胡同。真没数据 → 给创建 / 前往入口；被筛选条件筛空 → 给「清除筛选」（`filterBar.clearTokens`），两者用 `filterBar.tokens.length > 0` 区分，文案也要分开（`xxx.no_match` 与 `xxx.empty`）。已接入：`sessions`、`apikeys`、`upstream`。
- 数值展示：统一走 `src/lib/utils.ts` 的格式化入口——token 用 `formatTokenCount`（K/M/B 量级缩写）、计数用 `formatInteger`（千分位）、小数用 `formatDecimal`、时长用 `formatDurationMs`、占比用 `formatPct`、金额用 `formatCost`（`src/lib/money.ts`）。禁止在页面/组件里自写 `toLocaleString`、`toFixed` 拼数值文案（日期格式化除外）。
- 修改前端 DTO 时如发现后端字段缺失，按 `huma-dto-conventions` 流程到 `internal/dto/` 同步更新。
- **不适用的上游 skill**：`next-cache-components-adoption` / `next-cache-components-optimizer` / `next-partial-prefetching-adoption` 都要求服务端渲染 + 流式 Suspense，与本项目 `output: "export"` 静态导出（88/101 个组件是 `"use client"`，数据全部经 `api-client.ts` 在浏览器 fetch）架构冲突，**不要引入**。

## 动效契约

> 设计见 `docs/superpowers/specs/2026-10-12-theme-motion-worlds-design.md`，术语见 [web/CONTEXT.md](../../web/CONTEXT.md)「Motion」。

- **档位唯一来源**：`<html data-motion-tier>`（`immersive | lite | still`）。JS 读 `useMotionTier()`（`@/lib/motion/tier-store`），CSS 用 `html[data-motion-tier="…"]`；新代码不要再自行 `matchMedia("(prefers-reduced-motion)")`。全局 reduced-motion 守卫保留作底线；系统运行时开启「减弱动态」时切换到 `still`。
- **页面背景保持纯色**：body 的 `bg-background` 使用 `globals.css` 中主题 token（Anthropic `#faf8f5`、Moonshot `#000`）；整页 `.page-surface` 透明以透出该底色。不要为全页背景加纹理、渐变、粒子或 canvas，除非用户明确重新要求。
  - **唯一的用户授权例外（2026-11-02）**：门户页（`/web/`）moonshot 主题的星空。实现是 `.portal-root .portal-stars`，一层纯 CSS 的静态 `radial-gradient` 点阵，位于 `src/styles/portal.css`；只有 `immersive` 档会极慢地平移。它只挂在门户根容器上，控制台、登录、分享页一律不加。门户的其余样式也都限定在 `.portal-root` 作用域内。设计见 `docs/superpowers/specs/2026-11-02-portal-longpage-design.md` §10。
- **没有环境背景运行时**：不要添加 WebGL / Canvas 背景、帧率 governor 或背景脉冲总线。页面需要局部指针反馈时使用 `InteractionLayer` 和 CSS 变量。

- **组件动效写在语法层**：通用组件的进出场、按压、悬停动效只写在 `src/styles/motion-{anthropic,moonshot}.css`，按 `data-slot`（shadcn 自带）、`data-variant`（`Button` 输出）、`data-interactive`（可倾斜/可抬起的卡片，按需在业务处标记）命中。`components/ui/` 的弹层组件不再使用 tw-animate 的 `data-open:animate-in` 一类类名，新增弹层时同样不要加，而是在两份语法层里各补一组规则。例外：`Drawer` 的 popup 位移与过渡由下滑手势驱动（`--drawer-swipe-movement-*` 等变量），语法层只给 `drawer-overlay` 补底色，不要覆盖 `drawer-popup` 的 transform / transition。
- **新增 shadcn 组件不要直接 `shadcn add`**：CLI 会顺带 `npm install` 依赖清单（含 npm 包 `cn`、无版本号的 `@base-ui/react`），并可能覆盖已定制的 `button.tsx` 等文件。做法是 `shadcn add <name> --dry-run --view src/components/ui/<name>.tsx` 取源码落盘，把 `from "cn"` 改为 `@/lib/utils`，删掉 tw-animate 进出场类，再按需 `npm install --package-lock-only` 单独加真正需要的依赖。CI 按 lock 安装的 `shadcn` 是 4.8.0，其 `tailwind.css` 只提供 `no-scrollbar`；生成代码里的 `scroll-fade-*`、`shimmer`、`scrollbar-*` 在 CI 构建中是空操作，落盘时一并删掉。`src/components/shadcn-native.test.ts` 守护这些规范。
- **能用 CSS 就不用 JS**：只有需要逐帧物理的效果（磁吸弹簧、倾斜跟随）才进 `InteractionLayer`；弹簧参数在 `lib/motion/tokens.ts`，CSS 侧的 `--ease-orbit` 由 `lib/motion/spring.ts` 采样生成，`spring.test.ts` 保证两者一致——改弹簧参数要同步重新生成 CSS。
- **入场动画用 `backwards` 填充**：动画结束后不残留 `transform/filter/clip-path`，否则会覆盖元素自身的 transform（如卡片倾斜）或形成 containing block。moonshot 下不要在包含玻璃卡片的祖先元素上做 `filter` 动画（祖先会成为 backdrop root，入场期间玻璃失效）；页面包裹层只用 `--animate-page`（透明度 + 缩放），blur 放在内部块上。
- **主题切换走 `setTheme(next, origin)`**：`lib/motion/theme-transition.ts` 用 View Transitions 做日蚀 / 墨染转场（边缘层 `.vt-edge` 只存在于新状态、实时渲染）；`still` 档或浏览器不支持时回退到颜色渐变。语言切换仍只用 `LocaleFade`，不得复用这条路径。
- **侧栏激活态由 `NavIndicator` 承担**：导航项只输出 `data-nav-item` / `data-active`，激活背景是那个随弹簧移动的指示器；新增导航项不要再给激活项加背景色。
- **recharts 系列要展开 `useChartMotion()`**：`<Line>` / `<Area>` 的 `isAnimationActive` / 时长 / 缓动 / `animationMatchBy`（按 `time` 配对）统一来自 `@/lib/motion/chart-motion`，数据行必须带 `time` 字段。recharts 在 JS 里做动画，CSS 的 reduced-motion 守卫管不到，`still` 档只能靠这个 hook 关掉。
- **刷新不重绘没变的图**：recharts 只要 points 换引用就重放动画，而 points 依赖 data 与坐标轴配置。① 图表 data 用 `useMemo` 按各自的源曲线派生，源数据经 `replaceEqualDeep`（`@/lib/structural-share`）结构共享，内容不变则引用不变；② `XAxis`/`YAxis` 的 `tickFormatter`、`domain` 不要写内联函数/数组（每次渲染新引用会让坐标轴选择器失效、整图重放），用 `useCallback` 或模块级常量；③ 换时间范围属于换视图，用 scope 作 `ChartContainer` 的 key 整图重新入场。
- **定时刷新**：统一用 `@/hooks/use-auto-refresh` + `AutoRefreshButton`，档位在 `@/lib/refresh-interval`（`0` = 关闭）；不要在页面里自建 `setInterval`。
- **指标数字用 `MetricNumber`**：直接写 DOM 做里程表 / 解码动画，只动变化的位；不要再新增 `CountUp` 一类逐帧 setState 的计数器。
- **逐字入场标题（`SplitText`）按词断行**：字符被包进 `.split-word`（`inline-block` + `nowrap`），一行只能在词与词之间断，否则每个 inline-block 字符两侧都是换行机会，中文会拆出「接 / 入」、英文会断出 `mod / el`。英文按 `Intl.Segmenter` 词边界即可；中日文没有空格，ICU 词典认不出「接入」这类词，所以**文案里用 U+200B（JSON 写 `\u200b`）显式标出允许断行的位置**，有标记时 CJK 连续文字只在标记处断（`lib/split-words.ts`）。新增 / 修改 `portal.tagline` 这类用 `SplitText` 的文案时，zh / ja 都要补标记，并实测三语 × 桌面 / 手机无断词、无溢出。
- **内联脚本不要拼接**：根布局是 server component，从 `"use client"` 模块（如 `lib/theme.tsx`）导入的常量在这里是客户端引用，只能原样作为 prop 传入；与其他字符串拼接会把引用代理 stringify 成 `function(){throw …}`。每段首帧脚本单独一个 `<script>`。

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
- **对话框横向 flex 行**：`DialogContent` 是 grid，其子项 `min-width: auto` 会被内容 min-content 撑宽——横向 flex 行（如「币种选择 + 导入按钮」）放不下时不会换行/滚动，而是把整个 grid 轨道撑宽，内容溢出弹窗圆角边界。对话框正文里的横向 flex 工具行必须显式加 `flex-wrap`，不要依赖固定宽度组合刚好放得下。

### Table-Fit（列多表格的列宽预算）

列多到需要左右滑动的数据表，改用 `<Table fit>`：`table-layout: fixed` + 单元格 `overflow: hidden`，列宽由表头 `w-*` 声明，**只留主列不声明**（fixed 布局把剩余空间全给未声明列，实测 Chrome 行为：声明宽度之和小于表格宽度时余量归未声明列）。

- **列宽预算**：声明宽度之和 ≥ 容器宽度时会溢出成横向滚动，增删列前后都要核对。1280 视口下 `max-w-6xl` 容器的实测可用宽度约 900px（侧栏 256 + 页面内边距）。
- **不可截断内容按实测给足**：时间戳这类等宽文本（`2026/10/19 18:09:13` 在 `text-xs` 下约 163px）宽度不足会被单元格裁掉且无提示，必须按 `td.scrollWidth` 实测值给（该值含 `p-2` 内边距）。**操作列同属不可截断内容**：按钮是 `shrink-0`，行内 flex 又是 `justify-end`，宽度不足时溢出部分从**左侧**被裁掉，最左的按钮整颗消失（既无省略号也不出现滚动条）。预算按最宽语言实测（upstream 操作列：编辑/定价两个 `icon-sm` + 带文字 `DeleteButton`，zh 10rem、en 10.5rem）。
- **可截断内容一律 `truncate` + Tooltip**（lint 强制）；实测「静默裁掉」的方法：遍历 `tbody td`，`td.scrollWidth > td.clientWidth + 1` 且不在 `TooltipTrigger` 子树内即为不合格。**注意 `scrollWidth` 只看得见右侧溢出**，`justify-end` 的 flex 从左侧裁掉时该判据失效，需另比单元格内容盒与子元素包围盒的左右边界（任一侧溢出 > 1px 即不合格）。
- **窄视口兜底**：容器保留 `overflow-x-auto`，只有预算放不下时才出现横向滚动，属于预期行为。
- 开启 `fit` 的页面：`upstream`（分组/平铺）、`audit/model`、`audit/demo`、`sessions`、`cron`、`apikeys`。
- **声明宽度必须跟着容器量，不要跟着视口猜**：容器可用宽 = 视口 − 侧栏（256）− 页面内边距，且被 `max-w-6xl` 封顶。实测 `sessions` 表格：1280 视口 902px、1512 视口 1118px；**侧栏在 `lg` 以下收起，1024 视口反而有 ~976px**，所以窄视口不一定更窄，改预算前先量。
- **列表页也要按容器宽度渐进显示列**：`sessions` 的声明宽度合计曾涨到 924px（> 902），主列「摘要」被压到 41px 并出现横向滚动。现在常驻列合计 692px，`评分` / `工具` / `Agent` 三列分别用 `@min-[64rem]` / `@min-[58rem]` / `@min-[52rem]` 渐进展开（页面里的 `SHOW_*_FROM` 常量），主列稳定在 ~210px。新增列先定优先级与阈值，别直接常驻；预算涨上去之前先跑一遍实测。
- **`sr-only` 只能包文本，不能写在 `th` 上**：`sr-only` 是绝对定位，写在 `th` 上会让该列宽度不计入 fixed 布局的声明总和，剩余空间既不归主列也不归任何列——实测 902px 容器丢掉 53px，主列从 210px 掉到 157px。要藏表头文字就把 `sr-only` 放到 `th` 内部的 `<span>` 上。
- **按面板宽度渐进显示列（仪表盘 Endpoint / Top 两张表）**：1280 视口下面板表格只有约 310px，六列常驻的声明宽度就占 280px，主名列被挤到 17px（Top 排行只剩 5px）——名字完全看不见。这两张表的面板主体声明 `@container`，次要列用 `dense-table.tsx` 的 `SHOW_FROM_*`（`hidden @min-[Npx]:table-cell`）按优先级渐进展开；阈值按「主名列可见宽度 ≥ ~110px」反推，数值是表格可用宽度。新增列先定优先级，不要直接常驻。验证按 zh / en / ja × 1280 / 1440 / 1920 三语三宽实测（主名列 `clientWidth ≥ scrollWidth`、无被裁表头）。

### Font Scale（CJK 字号对齐）

`globals.css` 的 `:lang(zh)/:lang(ja)` 块覆盖 Tailwind v4 `--text-*` 主题变量，等比下调 CJK 字号（zh 0.92、ja 0.88），仅动字号不动 rem 间距。新增 text utility 档位时同步在两个 `:lang()` 块补对应 `--text-*` 覆盖。预留 `min-w` 应以缩放后的 CJK 宽度为准核定。

- **字号下限**：`--text-xs` 在 zh / ja 钉在 `0.75rem`（12px），不再等比下调——CJK 笔画密，11px 以下读不清。业务里的字面量字号不得小于 `text-[11px]`（`text-[10px]`、`text-[9px]`、`text-[0.625rem]` 一律不允许）：表头、环比、热力图坐标这类密集数据标签用 11px，正文与说明 ≥ 12px。
- **小字品牌色用 `text-primary-ink`**：`--primary` / `--chart-2` 在 `#faf8f5`、`#f5f0eb` 上只有 3.96–4.3:1，eyebrow、类别标签这类 ≤13px 的品牌色文字改用 `--primary-ink`（≥ 4.5:1）。`--positive` / `--warning` 会直接当文字色，浅色主题下已调到白卡上 ≥ 4.5:1，别再调浅。
- 验证：无头浏览器遍历文本节点，按「字号 < 11px」与「前景色 alpha 合成到真实背景后的对比度 < 4.5:1」两个判据统计，目标均为 0。

### 切换闪烁

`<LocaleFade>` 包裹 dashboard `<main>` 内 `max-w-6xl` 容器与 share 页根，监听 `locale` 变化做 ~120ms opacity 淡入。新增会因切换语言而 reflow 的页面根容器时，用 `<LocaleFade>` 包裹。不要在 `width`/`height` 上加 CSS transition，不要引入 View Transitions API。

## 联调与发布

- 本地完整链路：先后端 `go run ./cmd/server server start ...`，再 `cd web && npm run dev`，浏览器访问 `http://localhost:3000/web`。
- 生产路径：`make build` → 镜像里 Go 二进制内置 `internal/web/dist/`，浏览器访问 `https://<host>/web/`。
- CI：`.github/workflows/build-and-publish.yml` 的 push path filter **包含** `web/**`（与 `internal/**`、`go.mod` 等同级），纯前端改动也会触发构建发布：构建前端 → embed 进 Go 镜像 → 推送到 ghcr → 部署到 K8s。推送到 `master` 或合并 PR 到 `master` 即自动发布，无需手工触发。另有 `.github/workflows/lint.yml` 的 `web-lint` job 跑 `npm run lint` + `npm run format:check`（无 `--max-warnings`，warning 不阻塞）。
- 测试：`cd web && npm run test`（vitest，覆盖 `filter-dsl` 等纯函数与自定义 eslint 规则）。改动后至少跑 `npm run lint && npm run test && npm run build`。
- **本机 `node_modules` 可能比 lock 新**：CI 按 `package-lock.json` 安装，本机依赖会漂移（曾出现 lock 锁 recharts 3.8.0、本机装着 3.10.1，用了 3.9 才有的 `matchByDataKey`，本地 tsc/build 全过、CI 构建失败）。使用依赖的新 API 前先核对 lock 里的版本（`node -p "require('./package-lock.json').packages['node_modules/<pkg>'].version"`），不够就升级 lock；涉及依赖的改动，在临时目录 `rsync` 源码后 `npx npm@10 ci && npm run build` 复现 CI。
- 提交：前端改动同样遵循 `.worktrees/` + `feature|bugfix|refactor|chore|docs|test|hotfix/...-YYYY-MM-DD` 分支规范；与后端联动的功能尽量在同一个 PR 中提交，避免接口前后不一致。
- worktree 内**禁止 `npm ci`**：软链主工作区依赖 `ln -s ../../../web/node_modules web/node_modules` 即可跑 lint/test（详见 [workflow.md](workflow.md) 的「Worktree 精简创建」）；需要构建产物时 Next 16 默认 Turbopack 会拒绝该软链，改用 `npx next build --webpack`。验证后删除 `web/.next` / `web/out` 构建产物。
