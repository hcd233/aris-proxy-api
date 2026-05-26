# Warm Paper Console 全局后台外壳设计规格

## 背景

当前 `web` 端是 Next.js App Router + Tailwind CSS + shadcn 风格组件的后台管理界面，整体为默认灰白后台风格。用户选择将本次美化范围聚焦在“全局后台外壳”，视觉方向为 `Warm Paper Console`，并补充希望字体参考 Claude，带一点手写/人文温度。

## 目标

- 将全局后台从默认灰白风格升级为温暖、低压、适合长时间管理操作的纸张控制台质感。
- 统一登录页、权限态、加载态、侧边栏、主内容背景、卡片、按钮、表格和 Badge 的视觉语言。
- 保持现有业务流程、路由、API 调用和权限逻辑不变。
- 字体方向采用“人文手写感 + 后台可读性”的组合：品牌、一级标题和少量强调文案使用带手写气质的 display 字体，正文和表格仍保持清晰易读。

## 非目标

- 不新增业务功能。
- 不重做页面信息架构。
- 不引入图表库或复杂动效库。
- 不改变接口、认证、权限、分页、创建/编辑/删除等行为。

## 视觉方向

### 色彩

- 背景：暖纸色渐变，如米白、浅羊皮纸、淡棕。
- 侧边栏：深咖啡/墨棕，形成稳定的后台导航锚点。
- 强调色：蜂蜜金/琥珀色，用于品牌符号、主按钮、激活导航和关键状态。
- 内容容器：接近纸张的浅色卡片，配暖色描边和柔和阴影。
- 危险操作：保留清晰红色语义，但降低刺眼程度，适配暖色主题。

### 字体

- 品牌和一级标题：采用接近 Claude 气质的温暖、人文、略带手写感的 display 字体。
- 正文、表格和控件：保持 sans-serif，提高后台信息密度下的可读性。
- 代码/API Key/模型名：继续使用 mono 字体，强调机器可读内容。

实现时使用本地字体栈优先匹配 `Kalam` / `Bradley Hand` / `Comic Sans MS` 这类手写感 display 字体，用于品牌 Logo、一级标题和少量强调文案；正文使用清晰 sans-serif 字体栈，代码/API Key/模型名继续使用 mono 字体栈。手写感字体不用于表格正文、表单输入和长段说明，避免整页过于随意，并避免构建时依赖外部字体下载。

## 涉及页面与组件

### 全局布局

- `web/src/app/layout.tsx`
  - 注册新的标题/display 字体变量。
  - 保留现有 `AuthProvider` 和 `Toaster` 结构。

- `web/src/app/globals.css`
  - 更新 CSS variables：背景、前景、卡片、主色、边框、ring、sidebar 等。
  - 增加全局纸张背景、选区色、滚动条等基础质感。
  - 保持 Tailwind/shadcn 变量体系，不绕开现有组件体系。

- `web/src/app/(dashboard)/layout.tsx`
  - 深咖侧边栏、暖纸主背景、品牌块、激活导航状态、用户信息卡片。
  - 移动端 Sheet 保持一致视觉。
  - 主内容区从纯 `bg-background` 改为暖纸背景容器。

### 通用组件质感

优先通过主题变量和少量 class 修改，不大改组件 API：

- `Card`：纸张背景、暖色边框、柔和阴影、更大的圆角。
- `Button`：主按钮使用琥珀强调，outline/ghost 与暖色背景协调。
- `Table`：表头更像 ledger，行 hover 使用暖色浅底。
- `Badge`：圆润标签，admin/user/pending 在暖色系统中保持辨识度。
- `Skeleton`：使用暖色 shimmer/底色，避免冷灰割裂。

### 登录和权限状态

- `web/src/app/login/page.tsx`
  - 改为 Warm Paper 风格品牌登录页。
  - 保留 GitHub/Google OAuth 行为。
  - 登录处理中和错误提示使用同一视觉体系。

- `web/src/components/permission-guard.tsx`
  - Loading、Access Pending、Access Denied 使用同一背景和卡片表达。

## 交互与动效

- 只使用 CSS/Tailwind 级别的微交互。
- 导航、按钮、卡片 hover 使用轻微上浮/阴影/暖色底变化。
- 不添加复杂动画，避免后台管理界面干扰操作效率。
- 尊重可访问性：focus ring 必须清晰，颜色对比不因暖色主题降低。

## 数据流与行为保持

本次改动不改变数据流：

- 登录状态仍由 `AuthProvider` 管理。
- 页面数据仍由 `api-client.ts` 请求。
- Dashboard、Sessions、API Keys、Endpoints、Models、Profile 的请求、分页、弹窗、删除、复制等行为保持不变。
- 管理员可见菜单仍由 `isAdmin()` 控制。

## 验证标准

- `web` 目录 TypeScript/ESLint 通过。
- `web` 构建通过。
- 主要页面可以正常渲染：登录页、Dashboard、Sessions、Session Detail、API Keys、Endpoints、Models、Profile。
- 未登录仍跳转登录页；pending/adminOnly 状态仍按原逻辑展示。
- 桌面和移动端布局无明显溢出，侧边栏折叠仍可用。

## 实现约束

- 尽量最小改动，优先改主题变量、布局 class 和现有 UI 组件样式。
- 不引入新的 UI 框架。
- 不新增业务状态或接口调用。
- 如果新增字体，必须通过 `next/font/google` 管理，避免运行期外链字体加载不可控。
- 保留现有 `basePath: "/web"` 和静态导出配置。
