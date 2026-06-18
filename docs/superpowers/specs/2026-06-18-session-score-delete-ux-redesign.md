# Session 评分与删除交互体验重设计

**日期**: 2026-06-18
**作者**: centonhuang

## 背景

当前 session 列表页和详情页的评分与删除交互存在布局稳定性问题：

1. **评分确认气泡挤压布局**：列表页点圆点后弹出的 `N? Yes/No` 内联气泡会改变行宽；详情页同样流程会挤压 header 中的 "Session #xxx" 标题。
2. **已评分的 `×` 清除按钮导致圆点左移**：`×` 出现时撑宽容器，把圆点向左推，与未评分状态不对齐。
3. **删除确认 UI 两页不一致**：列表页用 AlertDialog 居中弹窗，详情页用按钮下方 absolute 定位的小气泡，气泡还会挤压/遮挡相邻元素。

## 目标

- 评分：点击即提交，无确认气泡；已评分可直接覆盖、点 `×` 清除；圆点位置在所有状态下零位移。
- 删除：两页统一用 AlertDialog 居中弹窗，脱离文档流不挤压布局。
- 组件统一：列表页和详情页共用同一评分组件。

## 交互流程

### 评分（低风险，可随时修改）

| 状态 | 行为 |
|------|------|
| 未评分 | 5 个灰色圆点；hover 时实时预览（填充至 hover 位置）；点击即提交，无确认气泡 |
| 已评分 | 填充圆点显示当前分值；点击新分值直接覆盖；点 `×` 清除评分 |
| 提交中 | 所有圆点 `disabled` + `opacity-30`，防重复提交 |

### 删除（高风险，不可逆）

| 场景 | 行为 |
|------|------|
| 列表页单条删除 | 点 Trash2 图标 → AlertDialog 确认 → toast + 静默刷新列表 |
| 列表页批量删除 | 勾选后点 `Delete N` 按钮 → AlertDialog 确认 → toast + 刷新 |
| 详情页删除 | 点 Trash2 图标 → AlertDialog 确认 → toast + `router.push("/sessions/")` 返回列表 |

## 视觉形态

### 评分控件

放大圆点（沿用现有风格），5 个圆点表示 1-5 分。

- 圆点尺寸：16px（桌面）/ 20px（移动端）
- 触控 padding：桌面 4px（命中区 24px）/ 移动端 12px（命中区 44px，满足 `touch-target-size`）
- 填充色 `bg-primary`，未填充 `bg-muted-foreground/30`
- hover 预览：`v <= hover` 的圆点填充

### 删除 AlertDialog

- 标题：`Delete session?` + AlertTriangle 图标（destructive 色）
- 描述：`This will permanently delete Session #123 and all its messages. This action cannot be undone.`
- 按钮：`Cancel`（次级）+ `Delete`（destructive 变体，右对齐）
- 提交中：按钮 disabled + 文案变 `Deleting...`

## 布局稳定性（核心修正）

### 评分容器固定宽度

- 容器 `position: relative`，**固定宽度** = 5 圆点 + 4 gap + 36px 右侧预留
- 圆点组正常文档流，始终居左对齐
- `×` 用 `position: absolute; right: 4px` 浮在预留空间
- 未评分时不渲染 `×`，但容器宽度不变 → **圆点零位移**

### 删除 AlertDialog 脱离文档流

AlertDialog 通过 Portal 渲染，完全不挤压页面布局。

## 组件统一

### 新建 `ScoreDots` 组件

合并现有 `ScoreDotInput` + `ScoreStars` 的功能，列表页和详情页共用。

**文件**: `web/src/components/session-detail/score-dots.tsx`

**Props**:
```ts
interface ScoreDotsProps {
  score: number | undefined;
  scoring: boolean;
  onScore: (value: number) => void;
  onClear: () => void;
  size?: number; // 圆点尺寸，默认 16
}
```

**行为**:
- `score == null`：渲染 5 个可交互圆点，hover 预览，点击调用 `onScore`
- `score != null`：渲染填充圆点 + `×` 清除按钮（绝对定位），点击新分值调用 `onScore`，点 `×` 调用 `onClear`
- `scoring === true`：所有交互元素 disabled

### 删除文件

- `web/src/components/session-detail/score-dot-input.tsx`
- `web/src/components/session-detail/score-stars.tsx`

## 涉及文件

| 文件 | 改动 |
|------|------|
| `web/src/components/session-detail/score-dots.tsx` | **新建**：统一评分组件 |
| `web/src/app/(dashboard)/sessions/page.tsx` | 移除内联确认气泡逻辑（`scoreConfirm` state + Yes/No 气泡 JSX），改用 `ScoreDots`；移除 `ScoreDotInput` 引用 |
| `web/src/components/session-detail/session-detail-client.tsx` | 改用 `ScoreDots`（替换 `ScoreStars`）；移除 absolute 删除确认浮层，改用 `<AlertDialog>` |
| `web/src/components/session-detail/score-dot-input.tsx` | **删除** |
| `web/src/components/session-detail/score-stars.tsx` | **删除** |

## 无障碍

### 评分控件

- 每个圆点 `<button>` 带 `aria-label={`Rate ${v}`}`
- 容器 `role="group" aria-label="Session rating"`
- 已评分时容器 `aria-label={`Rated ${score} out of 5`}`
- `×` 清除按钮 `aria-label="Remove rating"`
- 触控目标：桌面 24px / 移动端 44px

### 删除 AlertDialog

- shadcn AlertDialog 自带 focus trap、`aria-modal`、`role="alertdialog"`
- ESC 关闭、点击遮罩关闭（组件内置）
- 确认按钮加载态保持 focus

## 响应式

| 元素 | 桌面 | 移动端 |
|------|------|--------|
| 列表页表格 Score 列 | 列宽 `w-[160px]`（容纳 5×16px + 4×8gap + 36px 预留 = 148px + padding），圆点 16px | 卡片右侧操作区，圆点 20px |
| 详情页 header 评分 | 圆点 16px，容器居中 | 圆点 20px，header 右侧操作区 |
| 删除 AlertDialog | 居中弹窗，max-width 28rem | 居中弹窗（shadcn 默认） |

## 动画

- 圆点 hover/填充：`transition-colors duration-150`
- 评分提交/清除：无额外动画，状态切换即时
- AlertDialog：沿用 shadcn 内置进出动画（fade + scale）

## Toast 反馈

| 场景 | 消息 |
|------|------|
| 评分成功 | `toast.success("Scored")` |
| 清除评分 | `toast.success("Score removed")` |
| 删除成功 | `toast.success("Session deleted")` |
| 批量删除成功 | `toast.success("${count} sessions deleted")` |
| 失败 | `toast.error("Failed to ...")` |

## 不在范围内

- 后端 API 改动（`scoreSession` / `deleteScoreSession` / `deleteSession` / `batchDeleteSessions` 接口不变）
- 删除的撤销机制（Undo toast / 软删除恢复）—— 当前后端为硬删除，不引入撤销
- 评分控件的星级视觉形态（本次确定用放大圆点）
