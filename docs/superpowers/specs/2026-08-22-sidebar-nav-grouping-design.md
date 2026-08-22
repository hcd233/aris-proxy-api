# 侧边栏导航分组设计（Sidebar Nav Grouping）

日期：2026-08-22
状态：已确认方向（可折叠分组），分组与展开细节为默认判断，用户可后调

## 背景与问题

`web/src/app/(dashboard)/layout.tsx` 的左侧导航目前是 **16 个扁平排列的导航项**（admin 视角全部可见），无任何分组或层级，视觉杂乱、难以扫视。

## 目标

- 建立分组层级，消除杂乱感；所有入口保持一次点击直达。
- 保留既有能力：demo 模块锁定置灰、collapsed 窄栏模式、移动端 Sheet、i18n 三语言。

## 方案（用户选定：可折叠分组）

### 分组结构

| 组 | 项 |
|----|----|
| 概览 | 仪表盘 |
| 数据观测 | 会话记录、分享记录、模型调用审计、训练数据、智能体轨迹 |
| 网关配置 | API 密钥、上游端点、模型管理、触发词 |
| 运维与自动化 | 定时任务、任务审计、运行时监控 |
| 系统管理 | 用户管理、Demo 演示、个人中心 |

分组依据「看数据 / 配网关 / 跑运维 / 管系统」的心智模型；两个审计入口按语义归入数据观测与运维。

### 展开行为

- 初始（无存储）：只展开当前 pathname 所在组，其余收起。
- 手动展开/收起：持久化到 `localStorage("sidebar-nav-open-groups")`（JSON 数组）。
- 导航到收起组内的页面时，自动展开该组（effect 监听 pathname）。

### collapsed 窄栏（w-16）模式

- 组标题隐藏，组间渲染细分隔线，图标项平铺**不折叠**（保持现状交互 + tooltip 机制）。

### 权限与 demo

- 组内项过滤逻辑不变（adminOnly 过滤 / demo 全显示）；过滤后组内为空则整组隐藏。
- demo 锁定项（置灰 + Lock 图标 + tooltip）在分组内照常渲染。

## 改动范围

- `web/src/app/(dashboard)/layout.tsx`：`NavItem[]` → `NavGroup[]`（`groupKey` + `items`）；`SidebarNav` 渲染组头（button + Chevron 旋转）与子项；展开状态管理。
- `web/src/locales/{zh,en,ja}.json`：新增 `nav.group.*` 5 个键 ×3 语言。
- 不引入新依赖：复用 session-detail 已有的 `useState` + Chevron 条件渲染模式。

## 验证

- `cd web && npm run lint && npm run build`。
- 浏览器目视：展开/收起、记忆、当前组自动展开、窄栏分隔线、demo 锁定项、移动端 Sheet。
