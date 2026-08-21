# Demo Sessions 管理入口迁移实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** demo tab 删除候选选择器，sessions 列表页与详情页新增「添加到 demo」toggle 按钮（admin 且 demo 登录开启时显示）。

**Architecture:** 纯前端改动（后端接口已齐备，零改动）。新增 `use-demo-whitelist` hook（页面级拉取 loginEnabled + 全量白名单 ID）与 `DemoAddButton` 展示组件；`demo-sessions-manager.tsx` 内部删除候选选择器部分，保留已选列表。

**Tech Stack:** Next.js 16 App Router / React 19 / TypeScript / Tailwind v4 / lucide-react / sonner。

## Global Constraints

- **提交纪律**：git 提交/推送须经用户明确要求（`docs/agents/workflow.md` 硬约束），任务步骤不包含 commit；全部完成后统一询问用户。
- **分支**：按用户既往偏好（memory: feedback-direct-master-dev），快速任务直接在 master 开发，不开 worktree。
- **后端零改动**：不碰 `internal/**`、不新增接口；复用现有 `getDemoConfig` / `listDemoSessions` / `addDemoSessions` / `removeDemoSessions`。
- **按钮可见性**：`isAdmin() && loginEnabled`（`loginEnabled` 来自 `GET /api/v1/demo/config`）；不满足时不渲染（返回 null），fail-closed。
- **样式**：仅 Tailwind v4 + `cn()`；图标统一 lucide-react；toast 用 sonner 的 `toast.*`；禁止 `alert/confirm`。
- **i18n**：`web/src/locales/{zh,en,ja}.json` 为扁平 key 结构，三语必须同步增删；新增 key：`demo.add_tooltip` / `demo.in_demo_tooltip` / `demo.added` / `demo.removed`；删除候选选择器专属 key（见 Task 1）。
- **验证**：任务内用 `cd web && npx tsc --noEmit`（快速类型检查）；最终 Task 7 跑 `npm run lint && npm run build` + 浏览器交互验证。
- **接口类型**：`AddDemoSessionsReqBody = { sessionIds: number[] }`；`removeDemoSessions(ids: number[])`；`ListDemoSessionsRsp.sessions: DemoSession[]`（含 `id`）；单页上限 100。

---

### Task 1: i18n 文案增删（zh/en/ja 三语）

**Files:**
- Modify: `web/src/locales/zh.json`（`demo.*` 扁平 key 区）
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Produces: 新 key `demo.add_tooltip` / `demo.in_demo_tooltip` / `demo.added` / `demo.removed`（Task 3/5/6 引用）；删除 `demo.selector_title` / `demo.selector_desc` / `demo.add_selected` / `demo.search_placeholder` / `demo.no_candidates` / `demo.candidates_load_error` / `demo.add_success` / `demo.add_error`（Task 4 删除候选选择器后无引用）。

- [ ] **Step 1: 三语各新增 4 个 key**

zh.json `demo.remove_error = 移出会话失败` 之后追加：
```json
"demo.add_tooltip": "加入演示账户",
"demo.in_demo_tooltip": "已加入演示账户，点击移出",
"demo.added": "已加入演示账户",
"demo.removed": "已移出演示账户",
```
en.json 对应追加：
```json
"demo.add_tooltip": "Add to demo",
"demo.in_demo_tooltip": "In demo — click to remove",
"demo.added": "Added to demo",
"demo.removed": "Removed from demo",
```
ja.json 对应追加：
```json
"demo.add_tooltip": "デモに追加",
"demo.in_demo_tooltip": "デモに追加済み、クリックで解除",
"demo.added": "デモに追加しました",
"demo.removed": "デモから解除しました",
```

- [ ] **Step 2: 三语各删除 8 个候选选择器 key**

用 python 按 key 删除（不依赖行号/顺序），三文件同跑：
```python
import json
removed = [
  "demo.selector_title", "demo.selector_desc", "demo.add_selected",
  "demo.search_placeholder", "demo.no_candidates", "demo.candidates_load_error",
  "demo.add_success", "demo.add_error",
]
for f in ["zh", "en", "ja"]:
    path = f"src/locales/{f}.json"
    d = json.load(open(path))
    for k in removed:
        d.pop(k, None)
    json.dump(d, open(path, "w"), ensure_ascii=False, indent=2)
    open(path, "a").write("\n")
```

- [ ] **Step 3: 验证**

Run: `cd web && python3 -c "import json; [json.load(open(f'src/locales/{f}.json')) for f in ('zh','en','ja')]" && grep -rn "demo.add_tooltip\|demo.in_demo_tooltip\|demo.added\|demo.removed" src/locales/*.json | wc -l`
Expected: 12（3 文件 × 4 key）；三文件 JSON 解析无报错。

---

### Task 2: `use-demo-whitelist` hook

**Files:**
- Create: `web/src/hooks/use-demo-whitelist.ts`

**Interfaces:**
- Consumes: `api.getDemoConfig()` / `api.listDemoSessions(page, pageSize)` / `api.addDemoSessions({sessionIds})` / `api.removeDemoSessions(ids)`（`src/lib/api-client.ts:265-301`）；`showErrorToast(err)`（title 可省略，从 err 提取）；`useT()`。
- Produces: `useDemoWhitelist()` → `{ loginEnabled: boolean, pending: boolean, isInDemo(id: number): boolean, toggle(id: number): Promise<void> }`（Task 3/5/6 消费；`loading` 无消费者，实现时已删，见 Task 2 说明）。

- [ ] **Step 1: 写 hook 实现**

创建 `web/src/hooks/use-demo-whitelist.ts`，完整内容：
```typescript
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useT } from "@/lib/i18n";

/** 与后端 DemoSessionMaxPageSize 一致（internal/common/constant/string.go） */
const DEMO_WHITELIST_PAGE_SIZE = 100;

/**
 * demo 白名单视角（admin 用）：loginEnabled + 全部白名单 ID 集合。
 * 挂载时拉取一次 getDemoConfig 与 listDemoSessions 全量；toggle(id) 单条添加/移除并同步本地集合。
 */
export function useDemoWhitelist() {
  const t = useT();
  const [loginEnabled, setLoginEnabled] = useState(false);
  const [demoIds, setDemoIds] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const pendingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const cfg = await api.getDemoConfig();
        if (!cancelled) setLoginEnabled(cfg.config?.loginEnabled ?? false);
      } catch (err) {
        if (!cancelled) showErrorToast(err); // 失败保持 false → 按钮不渲染（fail-closed）
      }
      try {
        const ids = new Set<number>();
        let page = 1;
        for (;;) {
          const rsp = await api.listDemoSessions(page, DEMO_WHITELIST_PAGE_SIZE);
          if (cancelled) return;
          for (const s of rsp.sessions ?? []) ids.add(s.id);
          const total = Number(rsp.pageInfo?.total ?? ids.size);
          if (page * DEMO_WHITELIST_PAGE_SIZE >= total) break;
          page += 1;
        }
        if (!cancelled) setDemoIds(ids);
      } catch (err) {
        if (!cancelled) showErrorToast(err);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const isInDemo = useCallback((id: number) => demoIds.has(id), [demoIds]);

  const toggle = useCallback(
    async (id: number) => {
      if (pendingRef.current) return;
      pendingRef.current = true;
      setPending(true);
      const wasInDemo = demoIds.has(id);
      try {
        if (wasInDemo) {
          await api.removeDemoSessions([id]);
          toast.success(t("demo.removed"));
          setDemoIds((prev) => {
            const next = new Set(prev);
            next.delete(id);
            return next;
          });
        } else {
          await api.addDemoSessions({ sessionIds: [id] });
          toast.success(t("demo.added"));
          setDemoIds((prev) => new Set(prev).add(id));
        }
      } catch (err) {
        showErrorToast(err);
      } finally {
        pendingRef.current = false;
        setPending(false);
      }
    },
    [demoIds, t],
  );

  return { loginEnabled, loading, pending, isInDemo, toggle };
}
```

- [ ] **Step 2: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增错误（`loginEnabled`/`demoIds` 等无未使用警告——tsc 不查 unused，交给 lint 的 Task 7）。

---

### Task 3: `DemoAddButton` 展示组件

**Files:**
- Create: `web/src/components/demo-add-button.tsx`

**Interfaces:**
- Consumes: `useAuth().isAdmin()`；`useT()`；props 由父组件（Task 5/6）传入。
- Produces: `<DemoAddButton sessionId: number, inDemo: boolean, pending: boolean, loginEnabled: boolean, onToggle: (id: number) => void />` —— `!isAdmin() || !loginEnabled` 时渲染 null。

- [ ] **Step 1: 写组件实现**

创建 `web/src/components/demo-add-button.tsx`，完整内容：
```typescript
"use client";

import { BadgePlus, Check } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

interface DemoAddButtonProps {
  sessionId: number;
  /** 该 session 是否已在 demo 白名单 */
  inDemo: boolean;
  /** 添加/移除请求进行中 */
  pending: boolean;
  /** demo 登录开关（false 时不渲染按钮） */
  loginEnabled: boolean;
  onToggle: (id: number) => void;
}

/** sessions 页「添加到 demo」按钮：admin 且 demo 登录开启时显示，点击 toggle 白名单 */
export function DemoAddButton({
  sessionId,
  inDemo,
  pending,
  loginEnabled,
  onToggle,
}: DemoAddButtonProps) {
  const t = useT();
  const { isAdmin } = useAuth();

  if (!isAdmin() || !loginEnabled) return null;

  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Button
              variant={inDemo ? "secondary" : "ghost"}
              size="icon-sm"
              disabled={pending}
              onClick={(e) => {
                e.stopPropagation();
                onToggle(sessionId);
              }}
              className={inDemo ? "text-primary" : "text-foreground/70 hover:text-foreground"}
              aria-label={inDemo ? t("demo.in_demo_tooltip") : t("demo.add_tooltip")}
            >
              {inDemo ? <Check className="size-4" /> : <BadgePlus className="size-4" />}
            </Button>
          }
        />
        <TooltipContent side="top">
          {inDemo ? t("demo.in_demo_tooltip") : t("demo.add_tooltip")}
        </TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}
```

- [ ] **Step 2: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增错误。

---

### Task 4: `demo-sessions-manager.tsx` 删除候选选择器

**Files:**
- Modify: `web/src/components/demo-sessions-manager.tsx`

**Interfaces:**
- Consumes: 无（独立瘦身）。
- Produces: 组件行为不变（已选列表 + 批量移除 + 分页），候选选择器及相关状态/函数/import 全部移除。Task 5/6 不依赖本文件。

- [ ] **Step 1: 清理 imports**

删除以下 import（候选选择器专用）：
```typescript
import { useMemo } from "react";          // 改为 { useCallback, useEffect, useState }
import { Plus } from "lucide-react";      // 改为 { Check, MessageSquare, Trash2 }
import { useI18n } from "@/lib/i18n";     // 整个删除
import type { SessionSummary } from "@/lib/types";  // 改为 type { DemoSession, PageInfo }
import { FilterBar } from "@/components/filter-bar/filter-bar";        // 整个删除
import { useFilterBar } from "@/components/filter-bar/use-filter-bar"; // 整个删除
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types"; // 整个删除
import { TimeRangePicker } from "@/components/ui/time-range-picker";   // 整个删除
import type { TimeRangeKey } from "@/lib/time-range";                  // 整个删除
import { computeRange } from "@/lib/time-range";                       // 整个删除
```
保留：`useCallback, useEffect, useState`、`Check, MessageSquare, Trash2`、`api`、`showErrorToast`、`useT`、`DemoSession, PageInfo`、Button/Card/Table 系列/ListEmptyState/TableSkeleton/PaginationBar/DeleteConfirmDialog/ProviderIcon/usePersistentState/useIsMobile/toast/cn/formatDateTime`。

- [ ] **Step 2: 删除候选选择器状态与函数**

删除以下代码块（原文件行号可参考，按标识符定位）：
1. 第 262-337 行附近「─── 选择器（候选 sessions）───」注释下全部 state、`const { locale } = useI18n();`、`CandidatesQuery` 接口与 `fetchCandidates`（含 `interface CandidatesQuery` 整体）。
2. `fetchOptionsFor` 与 `facets` 两个 useCallback/useMemo 块（含 `locale` 依赖注释）。
3. `filterBar` / `useFilterBar(...)` 及其 `const { queryParams } = filterBar;`。
4. 第 384-388 行「Initial fetch on mount with persisted pagination and filters」的 useEffect。
5. `toggleCandidateId` / `toggleCandidateAll` 两个 useCallback。
6. `handleAdd` useCallback（含 `adding` state 声明）。
7. state 声明行：`candidatePage*`（4 个 usePersistentState）、`candidatePageInfo`、`candidateLoading`、`candidateTimeRange`、`candidateCustomStart`、`candidateCustomEnd`、`candidateIds`、`adding`。

- [ ] **Step 3: 重写 render 结构**

`return (...)` 改为只渲染已选列表 Card（删除整个「选择器」Card 与其中 TimeRangePicker/FilterBar/添加按钮/`filterBar.tokens` 提示），保留 `DeleteConfirmDialog`。最终结构：
```tsx
return (
  <div className="space-y-8">
    <Card>
      <CardHeader>
        <CardTitle className="font-display">{t("demo.selected_title")}</CardTitle>
        <CardDescription>{t("demo.selected_desc")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          {selectedIds.size > 0 ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setRemoveConfirmOpen(true)}
              className="gap-1.5"
            >
              <Trash2 className="size-3.5" />
              {t("demo.remove_selected")} {selectedIds.size}
            </Button>
          ) : (
            <span />
          )}
        </div>

        {selectedLoading ? (
          <TableSkeleton rows={3} />
        ) : (
          <SessionListTable
            items={selectedSessions}
            selectedIds={selectedIds}
            onToggle={toggleSelectedId}
            onToggleAll={toggleSelectedAll}
            emptyMessage={t("demo.no_selected")}
            isMobile={isMobile}
          />
        )}

        <PaginationBar
          pageInfo={selectedPageInfo}
          onChange={(page, pageSize) => fetchSelected(page, pageSize)}
          totalLabel={t("pagination.sessions")}
        />
      </CardContent>
    </Card>

    <DeleteConfirmDialog
      open={removeConfirmOpen}
      onOpenChange={setRemoveConfirmOpen}
      title={t("demo.remove_confirm_title")}
      description={t("demo.remove_confirm_desc").replace("{count}", String(selectedIds.size))}
      confirmLabel={`${t("demo.remove_selected")} ${selectedIds.size}`}
      loadingLabel={t("common.processing")}
      loading={removing}
      onConfirm={handleRemove}
    />
  </div>
);
```
其余代码（SelectCheckbox、SessionListTable、selected 系列 state、`fetchSelected`、`toggleSelectedId/All`、`handleRemove`）保持原样不动。

- [ ] **Step 4: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增错误（候选选择器相关未使用 import/变量已全部删除）。

---

### Task 5: sessions 列表页集成按钮

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

**Interfaces:**
- Consumes: `useDemoWhitelist`（Task 2）、`DemoAddButton`（Task 3）。
- Produces: 桌面 Table actions 列与移动端卡片操作区出现按钮；行点击跳转不受影响（按钮内 `stopPropagation`）。

- [ ] **Step 1: 引入 hook 与组件**

```typescript
import { DemoAddButton } from "@/components/demo-add-button";
import { useDemoWhitelist } from "@/hooks/use-demo-whitelist";
```
在 `SessionsPage` 组件内（`const { isDemo } = useAuth();` 之后）：
```typescript
const { loginEnabled, pending, isInDemo, toggle } = useDemoWhitelist();
```

- [ ] **Step 2: 桌面 Table actions 列加宽并排按钮**

表头（原 `sessions/page.tsx:495`）：
```tsx
<TableHead className="w-16 sr-only">{t("common.actions")}</TableHead>
```
改为：
```tsx
<TableHead className="w-[104px] sr-only">{t("common.actions")}</TableHead>
```
行末单元格（原 `:565-579`）中删除按钮旁加按钮，改为：
```tsx
<TableCell className="w-[104px]">
  <div className="flex items-center justify-center gap-1">
    <DemoAddButton
      sessionId={s.id}
      inDemo={isInDemo(s.id)}
      pending={pending}
      loginEnabled={loginEnabled}
      onToggle={toggle}
    />
    <DeleteIconButton
      locked={isDemo()}
      disabled={deleteConfirm.loading && deleteConfirm.target?.id === s.id}
      onClick={(e) => {
        e.stopPropagation();
        deleteConfirm.openDelete(s);
      }}
      aria-label={t("sessions.delete_aria")}
    />
  </div>
</TableCell>
```

- [ ] **Step 3: 移动端卡片操作区加按钮**

在移动端卡片操作区（原 `:391-418`，`DeleteIconButton` 之前）插入：
```tsx
<DemoAddButton
  sessionId={s.id}
  inDemo={isInDemo(s.id)}
  pending={pending}
  loginEnabled={loginEnabled}
  onToggle={toggle}
/>
```

- [ ] **Step 4: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增错误。

---

### Task 6: 详情页集成按钮

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`

**Interfaces:**
- Consumes: `useDemoWhitelist`（Task 2）、`DemoAddButton`（Task 3）。
- Produces: header 操作区（分享与删除之间）出现按钮；demo 用户与普通 user 不可见（组件内 `isAdmin` 判断）。

- [ ] **Step 1: 引入 hook 与组件**

```typescript
import { DemoAddButton } from "@/components/demo-add-button";
import { useDemoWhitelist } from "@/hooks/use-demo-whitelist";
```
在 `SessionDetailClient` 组件内（`const { isDemo } = useAuth();` 之后）：
```typescript
const { loginEnabled, pending, isInDemo, toggle } = useDemoWhitelist();
```

- [ ] **Step 2: header 分享按钮与删除按钮之间插入**

在 `headerContent` 中 `{!isDemo() && (TooltipProvider ... Share2 ...)}` 块之后、`<TooltipProvider>`（删除按钮）之前插入：
```tsx
<DemoAddButton
  sessionId={metadata.id}
  inDemo={isInDemo(metadata.id)}
  pending={pending}
  loginEnabled={loginEnabled}
  onToggle={toggle}
/>
```

- [ ] **Step 3: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增错误。

---

### Task 7: 全量验证（lint + build + 浏览器交互）

**Files:**
- 无代码改动。

- [ ] **Step 1: lint 与 build**

Run: `cd web && npm run lint && npm run build`
Expected: lint 0 issues；`next build` 成功（output: export 产出 `out/`）。

- [ ] **Step 2: 浏览器验证（chrome MCP，admin 登录 https://api.lvlvko.top/web/）**

1. demo tab：只剩配置卡片 + 已选列表（候选选择器消失）。
2. demo tab 开关「开放登录入口」打开 → sessions 列表页：行内出现「添加到 demo」按钮（BadgePlus 图标），未添加行点击 → toast「已加入演示账户」→ 按钮变勾选态（Check，primary 色）。
3. 详情页：header 分享与删除之间出现按钮，点击 toggle 行为一致。
4. demo tab 已选列表出现刚添加的行；「移出已选」批量移除仍可用。
5. 开关关闭 → sessions 列表/详情按钮消失。
6. 退出登录，以 demo 账户登录 → 白名单 sessions 可见、无按钮；以普通 user 账户登录 → 无按钮。

- [ ] **Step 3: 汇总报告**

报告：改动文件清单、lint/build 结果、浏览器各验证点结果、未验证项（如有）。
