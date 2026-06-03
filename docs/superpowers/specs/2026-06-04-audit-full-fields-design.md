# Audit 列表全字段展示优化设计

日期：2026-06-04
状态：approved
分支：feature/audit-full-fields-2026-06-04

## 1. 问题

当前 audit 列表仅展示 17 个字段中的部分字段，以下字段未渲染：
- `upstreamProvider`（上游提供商）
- `apiProvider`（接口协议）
- `cacheCreationInputTokens`（缓存写入 token）
- `cacheReadInputTokens`（缓存命中 token）
- `userAgent`（请求 UA）
- `errorMessage`（错误信息）

## 2. 目标

在桌面端和移动端展示全部 17 个字段，信息不丢失，同时保持界面可用性。

## 3. 设计决策

### 3.1 桌面端 — 紧凑混合布局（10 列）

| 列 | 内容 | 说明 |
|---|---|---|
| 时间 | `01:20:29` + 日期副行 | 两行显示 |
| 模型 | `deepseek-v4-flash` | 过长截断 |
| 用户 | `hcd233` + 邮箱副行 | 两行显示 |
| Provider | `openai` + `upstream: openai` 副行 | 两行显示 |
| 状态 | Badge（2xx 绿 / 非 2xx 红）| errorMessage 不为空时 hover Badge 展示 Tooltip |
| Token | `1,079 / 421` + `c: 0 / 1,024` 副行 | 缓存行仅在任一缓存值 > 0 时显示 |
| 延迟 | `681ms` + `3.8s` 副行 | 首 Token / 流时长 |
| UA | 截断显示（max 160px），hover 完整 Tooltip | — |
| TraceID | 截断 6 字符，点击复制完整 ID | 等宽字体 |

- 错误行（非 2xx）整行淡红色背景
- 缓存行键名缩写 `c:` = cache

### 3.2 移动端 — 可展开卡片

**折叠态**：模型 + Badge · 用户/Key/Provider · Token 缩写/延迟/缓存(非零) · TraceID · 时间

**展开态**（`grid-template-rows: 0fr → 1fr`，250ms ease-out）：
- 两列网格：输入 Token / 输出 Token / 缓存写入 / 缓存命中 / 首 Token / 流时长 / Upstream / 用户
- UA 完整显示
- 完整时间戳
- 复制 TraceID 按钮
- 箭头 ▾ 旋转 180deg（200ms ease）

**有错误时**：展开态顶部红色区域展示 errorMessage

**交互规则**：一次只展开一个卡片（手风琴模式）

## 4. 动画方案

- 桌面端：无动画（表格内联展示，信息已全部可见）
- 移动端：CSS `grid-template-rows` 过渡 + 箭头旋转
  - 展开：`grid-template-rows: 0fr → 1fr`，duration 250ms，ease-out
  - 箭头：`rotate(0deg) → rotate(180deg)`，duration 200ms，ease
  - 遵循 `prefers-reduced-motion`

## 5. 不涉及

- 后端接口不变（`ListAuditLogsRsp` / `AuditLogItem` 字段不变）
- 不修改排序、分页、时间范围、搜索逻辑
- 类型定义 `AuditLogItem` 已完整，无需修改

## 6. 组件拆分

改动范围 `web/src/app/(dashboard)/audit/page.tsx`：

- 重构桌面端 `<Table>` 列定义：新增 UA 列，Token 列合并缓存信息
- Token 格式化函数扩展：支持 `c: x / y` 缓存行
- 移动端卡片：新增展开/折叠状态管理（`expandedId`）
- 移动端卡片：实现 `grid-template-rows` 动画 + 手风琴模式
- 桌面端：状态 Badge 添加 errorMessage Tooltip

## 7. 验收

- [ ] 桌面端 10 列全部正确渲染
- [ ] Token 列缓存行仅非零时显示
- [ ] 状态 Badge hover 展示 errorMessage（当不为空时）
- [ ] 错误行淡红色背景
- [ ] UA 列截断 + hover 完整显示
- [ ] 移动端卡片折叠态展示核心字段
- [ ] 移动端卡片展开态展示全部字段
- [ ] 移动端展开/收起动画流畅
- [ ] 移动端一次只展开一个（手风琴）
- [ ] 移动端有错误时红色错误区域
- [ ] `npm run lint` 通过
- [ ] `npm run build` 通过
