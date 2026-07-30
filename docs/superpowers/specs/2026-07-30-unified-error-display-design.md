# 统一后端接口错误展示（Web 前端）

## 背景

之前各页面处理 API 错误的方式不统一：
- `catch { toast.error(t("...")) }` — 直接丢弃原始错误信息
- `catch (err) { toast.error(err instanceof Error ? err.message : t("...")) }` — 手写三元链
- `catch (err) { const msg = err instanceof ApiError ? ... : ... }` — 六层 instanceof 判断

且没有任何组件支持**内联错误横幅**、**错误边界兜底**或**异步状态机管理**。

## 设计目标

1. 统一所有 API 错误的解析和展示入口
2. 自动区分严重级别（critical / error / warning / info），控制展示时长和样式
3. 提供从纯函数到 React 组件的多层抽象，按需选用
4. 存量代码可逐页迁移，无需一次性重构

## 架构

```
api-client.ts                  HTTP 请求层
     ↓
api-error-handler.ts           错误解析 + toast 快捷方式
     ↓
api-errors.ts                  错误码 + 错误结构化
     ↓
hooks/use-api-error.ts         异步状态管理（idle/loading/success/error）
     ↓
components/ui/error-banner.tsx  内联错误横幅（页面/卡片级）
components/ui/error-state.tsx   全状态组件（空状态/错误状态）
components/ui/error-boundary.tsx React Error Boundary（渲染异常捕获）
```

## 文件清单

| 文件 | 职责 |
|------|------|
| `web/src/lib/api-errors.ts` | 错误码常量 + `parseError()` 纯函数 |
| `web/src/lib/api-error-handler.ts` | `showErrorToast()` toast 快捷方式 + 工具函数 |
| `web/src/hooks/use-api-error.ts` | React Hook — 异步操作状态机 |
| `web/src/components/ui/error-banner.tsx` | 内联错误横幅 |
| `web/src/components/ui/error-state.tsx` | 页面级全状态组件 (ErrorState + EmptyState) |
| `web/src/components/ui/error-boundary.tsx` | React Error Boundary |

## 核心 API

### parseError(err) — 纯函数

将任意形式的错误（ApiError、Error、string、unknown）统一解析为 `StructuredError`。

```typescript
interface StructuredError {
  code: number;          // 业务错误码，0 表示非业务层错误
  message: string;       // 用户可读的错误描述
  severity: ErrorSeverity; // critical | error | warning | info
  httpStatus?: number;   // HTTP 状态码
  rawBody?: string;      // 后端返回的原始 body
}
```

### showErrorToast(err, opts) — Toast 快捷方式

一键展示错误 toast，自动判断严重级别和显示时长。

```typescript
showErrorToast(err, { title: t("apikeys.load_error") });
```

| 严重级别 | 默认显示时长 |
|----------|-------------|
| critical | 10s |
| error | 6s |
| warning | 4s |
| info | 3s |

### useApiError(fn) — React Hook

管理异步操作状态机：idle → loading → success / error。

```typescript
const { execute, loading, error, data, clearError } = useApiError(api.listSessions);
```

### 组件

- **ErrorBanner** — 内联错误横幅，左 accent 边框 + 斜线条纹装饰，支持折叠详情和重试按钮
- **ErrorState** — 页面级错误/空状态，替代数据加载失败时的空白
- **EmptyState** — 空数据占位，支持自定义图标和操作按钮
- **ErrorBoundary** — React Error Boundary，兜底未捕获的渲染异常

## 迁移效果

将 15 个文件中约 40 处不一致的 catch 块统一替换为 `showErrorToast(err, { title: t("...") })`。

### 替换模式

| 原有模式 | 替换为 |
|----------|--------|
| `catch { toast.error(t("...")) }` | `catch (err) { showErrorToast(err, { title: t("...") }) }` |
| `catch (err) { toast.error(err instanceof Error ? err.message : t("...")) }` | `catch (err) { showErrorToast(err, { title: t("...") }) }` |
| `if (rsp.error) { toast.error(rsp.error.message ?? t("...")) }` | `if (rsp.error) { showErrorToast(rsp.error, { title: t("...") }) }` |
| 6 层 ApiError instanceof 三元链 | 1 行 `showErrorToast(err)` |

### 保留不变的场景

- 图表组件使用内部 `setError(true)` 状态 — 非 toast 展示，保留
- 登录/OAuth 回调/分享页使用内联 `setError()` 展示自定义 UI — 保留
- `// handled silently` / `console.error` 的主动静默处理 — 保留

## 设计决策

- `parseError()` 放在 `api-errors.ts`（纯函数层），`showErrorToast()` 放在 `api-error-handler.ts`（UI 层），职责分离
- 严重级别 `critical/error/warning/info` 控制颜色、图标和 toast 时长，由 `parseError` 自动推断
- `ApiError` 构造时自动解析 body JSON 到 `.structured` 字段，避免重复 JSON.parse
- HTTP 429/5xx 自动标记为可重试（`isRetryable()`）
- ErrorBanner 使用斜线装饰纹理 + 左 accent 边框的工业风格，与现有 shadcn/ui 主题融合
