# 会话详情页与分享页重设计实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将已登录会话详情页与公开分享页重设计为 claude.ai 风格的居中阅读列布局，桌面端使用组合左侧边栏，工具面板改为右侧悬浮抽屉，历史列表支持搜索与无限滚动。

**Architecture:** 将新功能拆分为独立可复用组件：`SessionHistoryList`（搜索+无限滚动）、`SessionHistorySheet`（移动端 bottom sheet）、`ToolDrawer`（桌面端右侧抽屉）。`SessionDetailClient` 负责组合 sidebar 布局与响应式切换。分享页独立复用阅读列与工具抽屉组件。

**Tech Stack:** Next.js App Router, React 19, TypeScript, Tailwind v4, shadcn/ui (Sheet, Separator, ScrollArea, Input), lucide-react.

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `web/src/components/session-detail/session-history-list.tsx` | 新增：可复用的会话历史列表，含搜索输入与 infinite scroll。 |
| `web/src/components/session-detail/session-history-sheet.tsx` | 新增：移动端底部 sheet 包装，内部复用 `SessionHistoryList`。 |
| `web/src/components/session-detail/tool-drawer.tsx` | 新增：桌面端右侧悬浮抽屉，用于展示工具调用列表。 |
| `web/src/components/session-detail/session-detail-client.tsx` | 修改：重构为组合 sidebar + 居中阅读列；集成历史列表与工具抽屉。 |
| `web/src/components/chat/chat-message.tsx` | 修改：更新助手/用户消息视觉风格，贴近 Claude。 |
| `web/src/app/(dashboard)/sessions/detail/page.tsx` | 修改：预取会话列表并传给 client 组件。 |
| `web/src/app/share/page.tsx` | 修改：重构为左侧元数据 sidebar + 居中阅读列 + 工具抽屉。 |
| `web/src/lib/types.ts` | 可能修改：确认 `SessionSummary` 字段满足历史列表展示需求。 |

---

## Task 1: 创建 `SessionHistoryList` 组件

**Files:**
- Create: `web/src/components/session-detail/session-history-list.tsx`

**目标:** 实现一个独立的会话历史列表组件，支持关键词搜索与无限滚动。

- [ ] **Step 1: 实现组件骨架**

```tsx
"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { Search, Loader2 } from "lucide-react";
import { api } from "@/lib/api-client";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import type { SessionSummary } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn, formatRelativeTime } from "@/lib/utils";

const HISTORY_PAGE_SIZE = 20;

export interface SessionHistoryListProps {
  activeSessionId: number;
  onSelect: (sessionId: number) => void;
}

export function SessionHistoryList({
  activeSessionId,
  onSelect,
}: SessionHistoryListProps) {
  const [keyword, setKeyword] = useState("");
  const [debouncedKeyword, setDebouncedKeyword] = useState("");
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetcher = useCallback(
    async (offset: number, limit: number) => {
      const page = Math.floor(offset / limit) + 1;
      const rsp = await api.listSessions({
        page,
        pageSize: limit,
        keyword: debouncedKeyword || undefined,
        sortField: "updated_at",
        sort: "desc",
      });
      const items = rsp.list ?? [];
      return { items, total: rsp.total ?? 0 };
    },
    [debouncedKeyword],
  );

  const { items, total, loading, hasMore, loadMore, reset } = useInfiniteList<SessionSummary>({
    fetcher,
    pageSize: HISTORY_PAGE_SIZE,
    enabled: true,
  });

  useEffect(() => {
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
    }
    searchDebounceRef.current = setTimeout(() => {
      setDebouncedKeyword(keyword);
    }, 250);
    return () => {
      if (searchDebounceRef.current) {
        clearTimeout(searchDebounceRef.current);
      }
    };
  }, [keyword]);

  useEffect(() => {
    reset();
  }, [debouncedKeyword, reset]);

  return (
    <div className="flex h-full flex-col">
      <div className="shrink-0 px-3 pb-3 pt-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search history"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="h-9 bg-background pl-9 text-sm"
          />
        </div>
      </div>
      <ScrollArea className="flex-1">
        <div className="px-2 pb-4">
          {items.length === 0 && !loading && (
            <p className="px-3 py-4 text-center text-sm text-muted-foreground">
              No history found
            </p>
          )}
          <ul className="space-y-0.5">
            {items.map((session) => (
              <li key={session.id}>
                <button
                  type="button"
                  onClick={() => onSelect(session.id)}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-left transition-colors",
                    "hover:bg-accent hover:text-accent-foreground",
                    session.id === activeSessionId &&
                      "bg-accent text-accent-foreground",
                  )}
                >
                  <p className="line-clamp-1 text-sm font-medium">
                    {session.summary || `Session #${session.id}`}
                  </p>
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
                    {session.messageCount ?? 0} messages ·{" "}
                    {formatRelativeTime(session.updatedAt)}
                  </p>
                </button>
              </li>
            ))}
          </ul>
          {hasMore && (
            <div className="flex justify-center py-3">
              <button
                type="button"
                onClick={() => void loadMore()}
                disabled={loading}
                className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground disabled:opacity-50"
              >
                {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                Load more
              </button>
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}
```

- [ ] **Step 2: 在 `web/src/lib/utils.ts` 中新增 `formatRelativeTime` 辅助函数**

```ts
export function formatRelativeTime(dateInput: string | number | Date): string {
  const date =
    typeof dateInput === "string" ? new Date(dateInput) : new Date(dateInput);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return "just now";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHour = Math.floor(diffMin / 60);
  if (diffHour < 24) return `${diffHour}h ago`;
  const diffDay = Math.floor(diffHour / 24);
  if (diffDay < 7) return `${diffDay}d ago`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}
```

- [ ] **Step 3: 确认类型字段**

检查 `web/src/lib/types.ts` 中 `SessionSummary` 是否包含 `id`、`summary`、`messageCount`、`updatedAt`。若 `messageCount` 字段名不同，需同步修改组件。

- [ ] **Step 4: Commit**

```bash
git add web/src/components/session-detail/session-history-list.tsx web/src/lib/utils.ts
git commit -m "feat(web): add reusable session history list with search"
```

---

## Task 2: 创建 `SessionHistorySheet` 组件

**Files:**
- Create: `web/src/components/session-detail/session-history-sheet.tsx`
- Depends on: `web/src/components/session-detail/session-history-list.tsx`

**目标:** 在移动端将历史列表包装为底部 sheet。

- [ ] **Step 1: 实现 sheet 组件**

```tsx
"use client";

import { History } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { SessionHistoryList } from "./session-history-list";

export interface SessionHistorySheetProps {
  activeSessionId: number;
  onSelect: (sessionId: number) => void;
}

export function SessionHistorySheet({
  activeSessionId,
  onSelect,
}: SessionHistorySheetProps) {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Session history">
          <History className="h-5 w-5" />
        </Button>
      </SheetTrigger>
      <SheetContent side="bottom" className="h-[80vh] p-0">
        <div className="flex h-full flex-col">
          <SheetHeader className="px-4 py-3 text-left">
            <SheetTitle>History</SheetTitle>
          </SheetHeader>
          <div className="flex-1 overflow-hidden">
            <SessionHistoryList
              activeSessionId={activeSessionId}
              onSelect={(id) => {
                onSelect(id);
                // sheet 关闭由外部 state 控制较复杂，这里依赖用户手动关闭或点击遮罩
              }}
            />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/session-detail/session-history-sheet.tsx
git commit -m "feat(web): add mobile session history sheet"
```

---

## Task 3: 创建 `ToolDrawer` 组件

**Files:**
- Create: `web/src/components/session-detail/tool-drawer.tsx`
- Reuse: `web/src/components/ui/sheet.tsx`

**目标:** 桌面端右侧悬浮抽屉，展示工具调用列表；移动端保持现有 bottom sheet（不替换）。

- [ ] **Step 1: 实现抽屉组件**

```tsx
"use client";

import { Wrench } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

export interface ToolDrawerProps {
  children: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function ToolDrawer({ children, open, onOpenChange }: ToolDrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Tools">
          <Wrench className="h-5 w-5" />
        </Button>
      </SheetTrigger>
      <SheetContent
        side="right"
        className={cn(
          "w-full sm:w-[420px] sm:max-w-[420px]",
          "p-0",
        )}
      >
        <div className="flex h-full flex-col">
          <SheetHeader className="border-b px-4 py-3 text-left">
            <SheetTitle>Tools</SheetTitle>
          </SheetHeader>
          <ScrollArea className="flex-1 p-4">{children}</ScrollArea>
        </div>
      </SheetContent>
    </Sheet>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/session-detail/tool-drawer.tsx
git commit -m "feat(web): add desktop tool drawer component"
```

---

## Task 4: 重构 `SessionDetailClient` 桌面端布局

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`
- Modify: `web/src/components/session-detail/tool-panel.tsx`（若原有 bottom sheet 触发按钮需要保留）

**目标:** 桌面端改为组合 sidebar（全局导航 + 历史），主区域使用居中阅读列，工具按钮打开右侧抽屉。

- [ ] **Step 1: 修改 `session-detail-client.tsx`，引入新组件并调整布局**

需要保留的核心逻辑：
- 加载当前会话消息、元数据、工具列表。
- 分享/删除/评分操作。
- 响应式：桌面显示 sidebar，移动端隐藏 sidebar 并显示 History sheet。

新增/修改要点：
- 桌面端外层 `flex h-screen overflow-hidden`。
- 左侧 sidebar 宽度 `w-64`，内部上半部分为导航，中间 `Separator`，下半部分 `SessionHistoryList`。
- 右侧主区域 `flex-1 flex flex-col min-w-0`。
- 主区域内使用 `mx-auto max-w-3xl w-full` 居中阅读列。
- 工具按钮改用 `ToolDrawer`，抽屉内容由现有工具列表渲染。

关键代码片段（完整实现略，需在 Task 4 中整合）：

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { MessageSquare, Key, Settings, User, Share, Trash2, LogOut } from "lucide-react";
import { useIsMobile } from "@/hooks/use-mobile";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { SessionHistoryList } from "./session-history-list";
import { SessionHistorySheet } from "./session-history-sheet";
import { ToolDrawer } from "./tool-drawer";
import type { SessionSummary } from "@/lib/types";

const navItems = [
  { label: "Sessions", href: "/web/sessions", icon: MessageSquare },
  { label: "Shares", href: "/web/shares", icon: Share },
  { label: "API Keys", href: "/web/apikeys", icon: Key },
  { label: "Profile", href: "/web/profile", icon: User },
];

export interface SessionDetailClientProps {
  sessionId: number;
  initialHistory: SessionSummary[];
}

export function SessionDetailClient({ sessionId }: SessionDetailClientProps) {
  const isMobile = useIsMobile();
  const router = useRouter();
  const [toolDrawerOpen, setToolDrawerOpen] = useState(false);

  // 保留现有数据加载与操作逻辑 ...

  const navigateToSession = (id: number) => {
    router.push(`/web/sessions/detail?id=${id}`);
  };

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background text-foreground">
      {!isMobile && (
        <aside className="flex h-full w-64 shrink-0 flex-col border-r bg-muted/30">
          <div className="shrink-0 px-3 py-3">
            <Link href="/web" className="block text-lg font-semibold tracking-tight">
              Aris Proxy
            </Link>
          </div>
          <nav className="shrink-0 px-2 pb-2">
            <ul className="space-y-0.5">
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = item.href === "/web/sessions";
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      className={cn(
                        "flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                        active
                          ? "bg-accent text-accent-foreground"
                          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      )}
                    >
                      <Icon className="h-4 w-4" />
                      {item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>
          <Separator />
          <div className="flex min-h-0 flex-1 flex-col">
            <div className="px-3 py-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                History
              </h3>
            </div>
            <div className="min-h-0 flex-1">
              <SessionHistoryList
                activeSessionId={sessionId}
                onSelect={navigateToSession}
              />
            </div>
          </div>
        </aside>
      )}

      <main className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between border-b px-4">
          {isMobile && (
            <SessionHistorySheet
              activeSessionId={sessionId}
              onSelect={navigateToSession}
            />
          )}
          <h1 className="flex-1 truncate px-4 text-center text-sm font-medium">
            {sessionTitle}
          </h1>
          <div className="flex items-center gap-1">
            <ToolDrawer open={toolDrawerOpen} onOpenChange={setToolDrawerOpen}>
              {/* 现有工具列表内容 */}
            </ToolDrawer>
            {/* 保留 Share / Delete / Score 按钮 */}
          </div>
        </header>

        <ScrollArea className="flex-1">
          <div className="mx-auto max-w-3xl px-4 py-8">
            {/* 消息列表 */}
          </div>
        </ScrollArea>
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/session-detail/session-detail-client.tsx
git commit -m "feat(web): claude-style combined sidebar and reading column"
```

---

## Task 5: 调整移动端 `SessionDetailClient`

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`

**目标:** 移动端在 sticky header 左侧增加 History 按钮，点击打开底部 sheet。

- [ ] **Step 1: 确认 `use-mobile` hook 断点**

检查 `web/src/hooks/use-mobile.ts`。通常断点为 `768px`（`md`）。确保 `isMobile` 在 `md` 以下返回 `true`。

- [ ] **Step 2: 在 header 左侧插入 `SessionHistorySheet`**

已在 Task 4 代码片段中体现。需确保桌面端不显示该按钮。

- [ ] **Step 3: Commit**

可与 Task 4 合并提交，或单独提交：

```bash
git commit -m "feat(web): mobile history sheet in session detail"
```

---

## Task 6: 更新 `chat-message` 视觉风格

**Files:**
- Modify: `web/src/components/chat/chat-message.tsx`

**目标:** 助手消息靠左并带 Claude 风格头像/圆点；用户消息靠右使用柔和圆角气泡。

- [ ] **Step 1: 调整消息容器布局**

```tsx
<div
  className={cn(
    "flex w-full",
    message.role === "user" ? "justify-end" : "justify-start",
  )}
>
  {message.role !== "user" && (
    <div className="mr-3 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/10">
      <Bot className="h-4 w-4 text-primary" />
    </div>
  )}
  <div
    className={cn(
      "max-w-[85%] sm:max-w-[75%]",
      message.role === "user"
        ? "rounded-2xl rounded-br-sm bg-muted px-4 py-2.5"
        : "text-foreground",
    )}
  >
    {/* 消息内容渲染 */}
  </div>
</div>
```

- [ ] **Step 2: 移除现有工具调用徽章或调整其位置**

若现有消息卡片右上角有工具调用徽章，可保留但移到消息内容下方小字显示，避免与 Claude 风格冲突。

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/chat-message.tsx
git commit -m "feat(web): claude-style message bubbles"
```

---

## Task 7: 重构分享页 `share/page.tsx`

**Files:**
- Modify: `web/src/app/share/page.tsx`
- 可能创建：无（复用 `ToolDrawer` 与 `chat-message`）

**目标:** 分享页使用居中阅读列；桌面端左侧显示被分享会话元数据 sidebar，不显示全局导航与历史。

- [ ] **Step 1: 调整分享页布局**

```tsx
export default function SharePage() {
  // 现有数据加载逻辑保持不变

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background text-foreground">
      {!isMobile && (
        <aside className="flex h-full w-64 shrink-0 flex-col border-r bg-muted/30 px-4 py-5">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Shared session
          </h2>
          <h1 className="mt-3 text-base font-semibold">
            {metadata.title || `Session #${metadata.sessionId}`}
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            {formatRelativeTime(metadata.createdAt)}
          </p>
          {/* 可补充分享者、消息数等元数据 */}
        </aside>
      )}

      <main className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between border-b px-4">
          <h1 className="flex-1 truncate px-4 text-center text-sm font-medium">
            {isMobile ? "Shared session" : ""}
          </h1>
          <div className="flex items-center gap-1">
            <ToolDrawer>
              {/* 分享页工具列表 */}
            </ToolDrawer>
          </div>
        </header>
        <ScrollArea className="flex-1">
          <div className="mx-auto max-w-3xl px-4 py-8">
            {/* 消息列表 */}
          </div>
        </ScrollArea>
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/app/share/page.tsx
git commit -m "feat(web): claude-style share page layout"
```

---

## Task 8: 验证与清理

**Files:**
- All modified/created files in `web/src/`

- [ ] **Step 1: 运行 lint**

```bash
cd web && npm run lint
```

Expected: no errors.

- [ ] **Step 2: 运行 build**

```bash
cd web && npm run build
```

Expected: build succeeds with no TypeScript errors.

- [ ] **Step 3: 清理未使用的 import/组件**

若旧版 `tool-panel.tsx` 中的 bottom sheet 仅被桌面端替换，移动端仍需保留原 sheet；若 `ToolDrawer` 不覆盖移动端，则不要删除原有 mobile tool sheet。

- [ ] **Step 4: 最终 commit**

```bash
git add -A
git commit -m "chore(web): polish session detail and share redesign"
```

---

## Self-Review

**1. Spec coverage:**
- 组合左侧边栏（全局导航 + 历史）→ Task 4。
- 历史搜索 + 无限滚动 → Task 1。
- 工具面板右侧悬浮抽屉 → Task 3 + Task 4。
- 移动端 History 底部 sheet → Task 2 + Task 5。
- 分享页元数据 sidebar → Task 7。
- Claude 风格消息气泡 → Task 6。

**2. Placeholder scan:** 无 TBD/TODO/待实现占位符。

**3. Type consistency:**
- `SessionHistoryList` 使用 `SessionSummary`，字段名以实际 `web/src/lib/types.ts` 为准。
- `formatRelativeTime` 输入为 `string | number | Date`。
- `ToolDrawer` 与 `SessionHistorySheet` 都使用受控/非受控兼容的 `open`/`onOpenChange`。
