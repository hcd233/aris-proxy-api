# Session 详情 / 分享页 Claude Work 风格重构 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从头重构 Web 端 session 详情页和分享页，复刻 Claude Work 的美观简洁优雅——极简顶栏 + 内联星标评分 + 可隐藏工具右栏 + 6 项消息渲染改进。

**Architecture:** 提取 `shared/reading-layout.tsx` 共享布局骨架（桌面 sticky 顶栏 + 阅读栏 + 可隐藏右栏；移动 iOS sticky header + bottom sheet）。拆分 `chat-message.tsx`（552 行）为 8 个单一职责组件。两页通过 slot 注入 `ReadingLayout`，数据加载逻辑留在 page 级组件。

**Tech Stack:** Next.js 16.2.6 (App Router, output: "export") + React 19 + TypeScript + Tailwind v4 + shadcn/ui (base-nova) + @base-ui/react + lucide-react + sonner + react-markdown + remark-gfm + rehype-highlight + mermaid

**Spec:** `docs/superpowers/specs/2026-06-18-session-share-claude-work-redesign.md`

**Verification:** Web 前端无测试框架（AGENTS.md §12.4）。每个任务的验证是 `cd web && npm run lint`（类型检查）。全部完成后跑 `npm run build` + 本地启动 + Chrome MCP 视觉验证。

**Worktree:** 按 AGENTS.md §10，执行前需在 `.worktrees/` 下创建 worktree 并 checkout 分支 `feature/session-share-claude-work-redesign-2026-06-18`。

---

## 文件结构总览

### 新建文件

| 文件 | 职责 |
|------|------|
| `web/src/components/chat/content-extract.ts` | 纯函数：extractContent / imageURLOf / buildToolResultsByID / normalizeToolCallID / lookupToolResult |
| `web/src/components/chat/multimodal-parts.tsx` | 图片/音频/文件/refusal 渲染 |
| `web/src/components/chat/reasoning-block.tsx` | thinking 折叠块（Claude Work 风格：透明背景 + 左侧细线） |
| `web/src/components/chat/tool-call-card.tsx` | 工具调用卡片（中性边框 + 折叠态参数预览） |
| `web/src/components/chat/user-message.tsx` | 用户气泡（bg-accent 暖粉 + 圆角 20px） |
| `web/src/components/chat/assistant-message.tsx` | 助手 prose（头像 + 时间在下，无文字标签） |
| `web/src/components/chat/system-message.tsx` | system 消息（保留标签） |
| `web/src/components/shared/reading-layout.tsx` | 共享布局骨架（桌面/移动自动切换） |
| `web/src/components/session-detail/score-stars.tsx` | 内联星标评分 + 行内气泡确认 |
| `web/src/components/session-detail/tools-rail.tsx` | 桌面可隐藏右栏内容 |
| `web/src/components/session-detail/tool-sidebar-item.tsx` | 工具列表项（从 session-detail-client 提取） |
| `web/src/components/session-detail/collapsible-text.tsx` | Show more/less（从 session-detail-client 提取） |

### 重写文件

| 文件 | 变化 |
|------|------|
| `web/src/components/chat/chat-message.tsx` | 552 行 → ~80 行薄入口 + re-export buildToolResultsByID |
| `web/src/components/chat/markdown-lite.tsx` | 仅改 CodeBlock 样式（§5.5） |
| `web/src/components/session-detail/session-detail-client.tsx` | 968 行 → ~200 行，用 ReadingLayout + ScoreStars + ToolsRail 组合 |
| `web/src/app/share/page.tsx` | 641 行 → ~250 行，用 ReadingLayout，去掉左侧元数据栏 |

### 删除文件

| 文件 | 原因 |
|------|------|
| `web/src/components/session-detail/tool-drawer.tsx` | 被 tools-rail.tsx + ReadingLayout 右栏 slot 取代 |

### 不变文件

- `web/src/app/(dashboard)/sessions/detail/page.tsx`
- `web/src/components/session-detail/session-history-sheet.tsx`
- `web/src/components/session-detail/session-history-sidebar.tsx`
- `web/src/components/session-detail/session-history-list.tsx`
- `web/src/components/session-detail/swipe-dismiss-sheet-body.tsx`
- `web/src/components/share/share-dialog.tsx`
- `web/src/hooks/use-infinite-list.ts`
- `web/src/hooks/use-mobile.ts`
- `web/src/lib/types.ts`
- `web/src/lib/api-client.ts`
- `web/src/lib/utils.ts`
- `web/src/app/globals.css`

---

## Task 1: 提取 content-extract.ts（纯函数）

**Files:**
- Create: `web/src/components/chat/content-extract.ts`
- Reference: `web/src/components/chat/chat-message.tsx:38-77,496-550`

将 `chat-message.tsx` 中的纯函数提取到独立文件，供多个组件共享。

- [ ] **Step 1: 创建 content-extract.ts**

```typescript
// web/src/components/chat/content-extract.ts

import type { MessageItem } from "@/lib/types";

export interface ContentPart {
  type?: string;
  text?: string;
  image_url?: string | { url?: string; detail?: string };
  input_audio?: { data?: string; format?: string };
  file?: { filename?: string; file_id?: string; file_data?: string };
  refusal?: string;
  audio_data?: string;
  audio_format?: string;
  file_data?: string;
  file_id?: string;
  filename?: string;
  image_detail?: string;
}

export interface ExtractedContent {
  text: string;
  parts: ContentPart[];
}

export function extractContent(content: unknown): ExtractedContent {
  if (!content) return { text: "", parts: [] };
  if (typeof content === "string") return { text: content, parts: [] };
  if (!Array.isArray(content)) return { text: "", parts: [] };

  const textBuf: string[] = [];
  const parts: ContentPart[] = [];
  for (const raw of content as Record<string, unknown>[]) {
    const part = raw as ContentPart;
    if (part.type === "text" && typeof part.text === "string") {
      textBuf.push(part.text);
    } else {
      parts.push(part);
    }
  }
  return { text: textBuf.join("\n"), parts };
}

export function imageURLOf(part: ContentPart): string | undefined {
  if (typeof part.image_url === "string") return part.image_url;
  if (part.image_url && typeof part.image_url === "object") {
    return part.image_url.url;
  }
  return undefined;
}

export function normalizeToolCallID(id: string): string {
  return id.replace(/[_-]/g, "").toLowerCase();
}

export function lookupToolResult(
  map: Record<string, string>,
  id: string,
): string | undefined {
  if (id in map) return map[id];
  const normalized = normalizeToolCallID(id);
  for (const key of Object.keys(map)) {
    if (normalizeToolCallID(key) === normalized) return map[key];
  }
  return undefined;
}

export function buildToolResultsByID(
  messages: MessageItem[],
): Record<string, string> {
  const map: Record<string, string> = {};
  for (const m of messages) {
    const id = m.message.tool_call_id;
    if (!id) continue;
    if (m.message.role !== "tool" && m.message.role !== "user") continue;
    const { text } = extractContent(m.message.content);
    map[id] = text;
  }
  return map;
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS（新文件尚无引用，不会报错）

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/content-extract.ts
git commit -m "refactor: extract content-extract pure functions from chat-message"
```

---

## Task 2: 提取 multimodal-parts.tsx

**Files:**
- Create: `web/src/components/chat/multimodal-parts.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:87-172`

- [ ] **Step 1: 创建 multimodal-parts.tsx**

```tsx
// web/src/components/chat/multimodal-parts.tsx

import { FileText, Music2, ShieldAlert } from "lucide-react";
import type { ContentPart } from "./content-extract";
import { imageURLOf } from "./content-extract";

function PartImage({ url }: { url: string }) {
  return (
    <div className="my-2 inline-block max-w-sm overflow-hidden rounded-lg border border-border/60 bg-muted/40">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={url} alt="" className="block h-auto max-h-80 w-full object-contain" />
    </div>
  );
}

function PartIconCard({
  icon,
  label,
  meta,
}: {
  icon: React.ReactNode;
  label: string;
  meta?: string;
}) {
  return (
    <div className="my-2 inline-flex items-center gap-2.5 rounded-lg border border-border/60 bg-muted/40 px-3 py-2">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground">
        {icon}
      </div>
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-foreground">{label}</p>
        {meta && <p className="truncate text-[11px] text-muted-foreground">{meta}</p>}
      </div>
    </div>
  );
}

function PartRefusal({ text }: { text: string }) {
  return (
    <div className="my-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
      <ShieldAlert className="mt-0.5 size-4 shrink-0" />
      <span className="whitespace-pre-wrap break-words">{text}</span>
    </div>
  );
}

export function MultimodalParts({ parts }: { parts: ContentPart[] }) {
  if (parts.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {parts.map((part, i) => {
        switch (part.type) {
          case "image_url": {
            const url = imageURLOf(part);
            return url ? <PartImage key={i} url={url} /> : null;
          }
          case "input_audio": {
            const fmt = part.input_audio?.format ?? part.audio_format ?? "audio";
            return (
              <PartIconCard
                key={i}
                icon={<Music2 className="size-4" />}
                label="Audio attachment"
                meta={String(fmt).toUpperCase()}
              />
            );
          }
          case "file": {
            const filename = part.file?.filename ?? part.filename ?? "file";
            const fileID = part.file?.file_id ?? part.file_id;
            return (
              <PartIconCard
                key={i}
                icon={<FileText className="size-4" />}
                label={filename}
                meta={fileID ? `id: ${fileID}` : undefined}
              />
            );
          }
          case "refusal":
            return part.refusal || part.text ? (
              <PartRefusal key={i} text={(part.refusal ?? part.text) as string} />
            ) : null;
          default:
            return null;
        }
      })}
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/multimodal-parts.tsx
git commit -m "refactor: extract multimodal-parts from chat-message"
```

---

## Task 3: 提取 + 重做 reasoning-block.tsx（Claude Work 风格）

**Files:**
- Create: `web/src/components/chat/reasoning-block.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:174-204` + spec §5.3

样式变化（spec §5.3）：去掉灰背景 → 透明背景 + 左侧细线 `border-border` + 圆角 `rounded-r-md`。

- [ ] **Step 1: 创建 reasoning-block.tsx**

```tsx
// web/src/components/chat/reasoning-block.tsx

import { useState } from "react";
import { Brain, ChevronDown, ChevronRight } from "lucide-react";

export function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  if (!text.trim()) return null;

  return (
    <div className="mb-3">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[12px] text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
      >
        <Brain className="size-3.5 text-primary/70" />
        <span className="font-medium tracking-wide">Thought process</span>
        {open ? (
          <ChevronDown className="size-3 opacity-60" />
        ) : (
          <ChevronRight className="size-3 opacity-60" />
        )}
      </button>
      {open && (
        <div className="mt-2 rounded-r-md border-l-2 border-border pl-4 pr-2 py-2">
          <p className="whitespace-pre-wrap break-words text-[13.5px] italic leading-[1.55] text-muted-foreground">
            {text}
          </p>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/reasoning-block.tsx
git commit -m "refactor: extract reasoning-block with Claude Work style"
```

---

## Task 4: 提取 + 重做 tool-call-card.tsx（紧凑 + 参数预览）

**Files:**
- Create: `web/src/components/chat/tool-call-card.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:249-328` + spec §5.4

样式变化（spec §5.4）：橙色边框 → 中性 `border-border`；背景 `bg-card`；折叠态显示一行参数预览。

- [ ] **Step 1: 创建 tool-call-card.tsx**

```tsx
// web/src/components/chat/tool-call-card.tsx

import { useState } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import type { UnifiedToolCall } from "@/lib/types";
import { lookupToolResult } from "./content-extract";

function prettyJSON(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

/**
 * Extract a one-line preview of the first key-value pair from the tool
 * call's JSON arguments string. Returns "" if arguments are empty or
 * not parseable as a JSON object.
 *
 * Example: '{"path":"foo.go","mode":"read"}' → 'path: "foo.go"'
 */
function previewFirstArg(argsJSON: string): string {
  if (!argsJSON) return "";
  try {
    const parsed = JSON.parse(argsJSON) as Record<string, unknown>;
    const entries = Object.entries(parsed);
    if (entries.length === 0) return "";
    const [key, value] = entries[0];
    const valStr =
      typeof value === "string" ? `"${value}"` : String(value);
    return `${key}: ${valStr}`;
  } catch {
    return "";
  }
}

interface ToolCallCardProps {
  call: UnifiedToolCall;
  result?: string;
}

export function ToolCallCard({ call, result }: ToolCallCardProps) {
  const [open, setOpen] = useState(false);
  const args = prettyJSON(call.arguments);
  const out = result ? prettyJSON(result) : undefined;
  const preview = previewFirstArg(call.arguments);

  return (
    <div
      className={cn(
        "mt-3 overflow-hidden rounded-lg border border-border bg-card",
      )}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2.5 px-3 py-2 text-left transition-colors hover:bg-muted/30"
      >
        <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-primary/12 text-primary">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-[13px] font-medium text-foreground">
              {call.name || "tool"}
            </span>
            {!open && preview && (
              <span className="ml-1 flex-1 truncate font-mono text-[11px] text-muted-foreground">
                {preview}
              </span>
            )}
          </div>
          {call.id && open && (
            <span className="font-mono text-[10px] text-muted-foreground/60">
              {call.id}
            </span>
          )}
        </div>
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {open && (
        <div className="border-t border-border bg-background/40">
          <div className="px-3 py-2.5">
            <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Input
            </p>
            <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90">
              {args || "{}"}
            </pre>
          </div>
          {out !== undefined && (
            <div className="border-t border-border px-3 py-2.5">
              <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                Output
              </p>
              <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90">
                {out}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/tool-call-card.tsx
git commit -m "refactor: extract tool-call-card with compact style + param preview"
```

---

## Task 5: 提取 + 重做 user-message.tsx（暖色圆润气泡）

**Files:**
- Create: `web/src/components/chat/user-message.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:418-437` + spec §5.1, §5.2

样式变化（spec §5.1, §5.2）：去掉 "YOU" 标签；`bg-muted` → `bg-accent`；圆角 16→20px；行距 1.7→1.6。

- [ ] **Step 1: 创建 user-message.tsx**

```tsx
// web/src/components/chat/user-message.tsx

import type { MessageItem } from "@/lib/types";
import { extractContent } from "./content-extract";
import { MultimodalParts } from "./multimodal-parts";
import { MarkdownLite } from "./markdown-lite";

interface UserMessageProps {
  message: MessageItem;
  index: number;
}

export function UserMessage({ message, index }: UserMessageProps) {
  const { content } = message.message;
  const { text, parts } = extractContent(content);

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <div
      style={style}
      className="animate-in fade-in slide-in-from-bottom-1 flex justify-end duration-300"
    >
      <div className="w-full max-w-[85%] md:max-w-[80%]">
        <div className="rounded-[20px] rounded-br-[6px] bg-accent px-5 py-3.5 text-[15px] leading-[1.6]">
          {parts.length > 0 && <MultimodalParts parts={parts} />}
          {text && <MarkdownLite text={text} raw />}
          {!text && parts.length === 0 && (
            <span className="text-muted-foreground/60">—</span>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/user-message.tsx
git commit -m "refactor: extract user-message with warm rounded bubble"
```

---

## Task 6: 提取 + 重做 assistant-message.tsx（头像 + 时间，无文字标签）

**Files:**
- Create: `web/src/components/chat/assistant-message.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:330-494` + spec §5.1

样式变化（spec §5.1）：去掉 "AI" 大写标签；时间移到头像下方；正文 `leading-[1.6]`。

- [ ] **Step 1: 创建 assistant-message.tsx**

```tsx
// web/src/components/chat/assistant-message.tsx

import { Sparkles, ShieldAlert } from "lucide-react";
import type { MessageItem, UnifiedToolCall } from "@/lib/types";
import { ProviderIcon } from "@/components/provider-icon";
import { cn } from "@/lib/utils";
import { extractContent, lookupToolResult } from "./content-extract";
import { MultimodalParts } from "./multimodal-parts";
import { ReasoningBlock } from "./reasoning-block";
import { ToolCallCard } from "./tool-call-card";
import { MarkdownLite } from "./markdown-lite";

function PartRefusal({ text }: { text: string }) {
  return (
    <div className="my-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
      <ShieldAlert className="mt-0.5 size-4 shrink-0" />
      <span className="whitespace-pre-wrap break-words">{text}</span>
    </div>
  );
}

function modelIcon(model: string) {
  return <ProviderIcon protocol={model} size={14} className="shrink-0" />;
}

interface AssistantMessageProps {
  message: MessageItem;
  index: number;
  toolResultsByID: Record<string, string>;
}

export function AssistantMessage({
  message,
  index,
  toolResultsByID,
}: AssistantMessageProps) {
  const { role, content, tool_calls, reasoning_content, refusal, model } =
    message.message;
  const { text, parts } = extractContent(content);
  const isAssistant = role === "assistant";

  const time = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : undefined;

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <div
      style={style}
      className="animate-in fade-in slide-in-from-bottom-1 flex gap-3 duration-300"
    >
      {/* Avatar column: icon + time below */}
      <div className="flex flex-col items-center gap-1 pt-0.5">
        <div
          className={cn(
            "flex size-7 shrink-0 items-center justify-center rounded-full",
            isAssistant
              ? "bg-primary/15 text-primary"
              : "bg-muted text-muted-foreground",
          )}
        >
          {isAssistant ? (
            model ? (
              modelIcon(model) ?? <Sparkles className="size-3.5" />
            ) : (
              <Sparkles className="size-3.5" />
            )
          ) : (
            <ShieldAlert className="size-3.5" />
          )}
        </div>
        {time && (
          <span className="text-[9px] leading-none text-muted-foreground/60">
            {time}
          </span>
        )}
      </div>

      <div className="min-w-0 flex-1">
        {reasoning_content && <ReasoningBlock text={reasoning_content} />}

        <div className="text-[15px] leading-[1.6] text-foreground">
          {parts.length > 0 && <MultimodalParts parts={parts} />}
          {text && <MarkdownLite text={text} />}
          {!text &&
            parts.length === 0 &&
            !tool_calls?.length &&
            !refusal && (
              <span className="text-muted-foreground/60">—</span>
            )}
        </div>

        {refusal && <PartRefusal text={refusal} />}

        {tool_calls && tool_calls.length > 0 && (
          <div>
            {tool_calls.map((call: UnifiedToolCall, i: number) => (
              <ToolCallCard
                key={call.id ?? i}
                call={call}
                result={
                  call.id
                    ? lookupToolResult(toolResultsByID, call.id)
                    : undefined
                }
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/assistant-message.tsx
git commit -m "refactor: extract assistant-message with avatar+time, no text labels"
```

---

## Task 7: 提取 system-message.tsx

**Files:**
- Create: `web/src/components/chat/system-message.tsx`
- Reference: `web/src/components/chat/chat-message.tsx:206-247` + spec §5.1

system 消息保留 "System" 文字标签（无头像，需要文字标识）。

- [ ] **Step 1: 创建 system-message.tsx**

```tsx
// web/src/components/chat/system-message.tsx

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { MarkdownLite } from "./markdown-lite";

const SYSTEM_MSG_PREVIEW_CHARS = 200;

interface SystemMessageProps {
  text: string;
  time?: string;
  index: number;
}

export function SystemMessage({ text, time, index }: SystemMessageProps) {
  const [open, setOpen] = useState(false);
  const trimmed = text.trim();
  const isLong = trimmed.length > SYSTEM_MSG_PREVIEW_CHARS;
  const display = !isLong || open
    ? trimmed
    : `${trimmed.slice(0, SYSTEM_MSG_PREVIEW_CHARS).trimEnd()}…`;

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <div
      style={style}
      className="animate-in fade-in slide-in-from-bottom-1 duration-300"
    >
      <div className="mb-1.5 flex items-center gap-2 text-[11px] text-muted-foreground/70">
        <span className="font-medium uppercase tracking-[0.14em]">System</span>
        {time && (
          <>
            <span className="text-muted-foreground/30">·</span>
            <span>{time}</span>
          </>
        )}
      </div>
      <div className="rounded-xl border border-dashed border-border bg-muted/40 px-4 py-3 text-[13.5px] leading-relaxed text-muted-foreground">
        <MarkdownLite text={display} />
        {isLong && (
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="mt-2 inline-flex items-center gap-1 font-medium text-primary/90 transition-colors hover:text-primary"
          >
            {open ? "Show less" : "Show more"}
            {open ? (
              <ChevronDown className="size-3" />
            ) : (
              <ChevronRight className="size-3" />
            )}
          </button>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/system-message.tsx
git commit -m "refactor: extract system-message component"
```

---

## Task 8: 重写 chat-message.tsx 为薄入口

**Files:**
- Rewrite: `web/src/components/chat/chat-message.tsx`

将 552 行重写为 ~80 行薄入口：角色分发 + re-export `buildToolResultsByID`（保持调用方 import 路径不变）。

- [ ] **Step 1: 重写 chat-message.tsx**

```tsx
// web/src/components/chat/chat-message.tsx

"use client";

/**
 * Chat message entry point — role dispatch + re-exports.
 *
 * Rendering is delegated to per-role components:
 *  - user-message.tsx    (warm rounded bubble, no text label)
 *  - assistant-message.tsx (avatar + time below, no text label)
 *  - system-message.tsx  (retains "System" label, no avatar)
 *
 * Tool-result messages (role="tool" or role="user" with tool_call_id)
 * are skipped here — they're inlined into the matched ToolCallCard.
 */

import type { MessageItem } from "@/lib/types";
import { extractContent } from "./content-extract";
import { UserMessage } from "./user-message";
import { AssistantMessage } from "./assistant-message";
import { SystemMessage } from "./system-message";

// Re-export for backward compatibility — call sites import
// buildToolResultsByID from "@/components/chat/chat-message".
export { buildToolResultsByID } from "./content-extract";

interface ChatMessageProps {
  message: MessageItem;
  index: number;
  toolResultsByID: Record<string, string>;
}

export function ChatMessage({
  message,
  index,
  toolResultsByID,
}: ChatMessageProps) {
  const { role } = message.message;

  // Tool results may arrive as role="tool" (Anthropic) or role="user"
  // with tool_call_id (OpenAI). Both are inlined into the matched
  // ToolCallCard above, so skip them here.
  const isToolResult =
    role === "tool" ||
    (role === "user" && !!message.message.tool_call_id);
  if (isToolResult) return null;

  if (role === "user") {
    return <UserMessage message={message} index={index} />;
  }

  if (role === "system") {
    const { text } = extractContent(message.message.content);
    const time = message.createdAt
      ? new Date(message.createdAt).toLocaleTimeString([], {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })
      : undefined;
    return <SystemMessage text={text} time={time} index={index} />;
  }

  // assistant (and any unhandled role via fallback)
  return (
    <AssistantMessage
      message={message}
      index={index}
      toolResultsByID={toolResultsByID}
    />
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: 验证 build 通过**

Run: `cd web && npm run build`
Expected: PASS（所有 import 路径正确，re-export 生效）

- [ ] **Step 4: Commit**

```bash
git add web/src/components/chat/chat-message.tsx
git commit -m "refactor: rewrite chat-message as thin entry, delegate to per-role components"
```

---

## Task 9: 更新 markdown-lite.tsx 代码块样式

**Files:**
- Modify: `web/src/components/chat/markdown-lite.tsx:120-172`（CodeBlock 组件）

样式变化（spec §5.5）：背景 `#1F1A14` → `#26211C`；圆角 `rounded-lg` → `rounded-xl`；加暖色外边框 `border border-[#3A322B]`；语言标签更柔；行距 `leading-relaxed` → `leading-[1.55]`。

- [ ] **Step 1: 修改 CodeBlock 组件**

在 `web/src/components/chat/markdown-lite.tsx` 中，找到 CodeBlock 函数（约 120 行），替换其 return 部分。

Old:
```tsx
  return (
    <div className="group/code my-3 overflow-hidden rounded-lg border border-border/60 bg-[#1F1A14] dark:bg-[#15110d]">
      <div className="flex items-center justify-between border-b border-white/5 px-3 py-1.5">
        <span className="font-mono text-[10px] uppercase tracking-[0.12em] text-white/40">
          {lang || "text"}
        </span>
        <button
          type="button"
          onClick={onCopy}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-[10px] text-white/40 transition-colors hover:bg-white/5 hover:text-white/80"
          aria-label="Copy code"
        >
          {copied ? (
            <>
              <Check className="size-3" />
              copied
            </>
          ) : (
            <>
              <Copy className="size-3" />
              copy
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto px-4 py-3 font-mono text-[12.5px] leading-relaxed text-[#E8DDD3]">
        <code className={cn("hljs bg-transparent !p-0", highlightedClassName)}>
          {children ?? value}
        </code>
      </pre>
    </div>
  );
```

New:
```tsx
  return (
    <div className="group/code my-3 overflow-hidden rounded-xl border border-[#3A322B] bg-[#26211C] dark:bg-[#1F1A14] dark:border-[#2A2520]">
      <div className="flex items-center justify-between border-b border-[#3A322B] px-3.5 py-1.5 dark:border-[#2A2520]">
        <span className="font-mono text-[10px] tracking-[0.12em] text-[#E8DDD3]/35">
          {lang || "text"}
        </span>
        <button
          type="button"
          onClick={onCopy}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-[10px] text-[#E8DDD3]/35 transition-colors hover:bg-white/5 hover:text-[#E8DDD3]/70"
          aria-label="Copy code"
        >
          {copied ? (
            <>
              <Check className="size-3" />
              copied
            </>
          ) : (
            <>
              <Copy className="size-3" />
              copy
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto px-4 py-3 font-mono text-[12.5px] leading-[1.55] text-[#E8DDD3]">
        <code className={cn("hljs bg-transparent !p-0", highlightedClassName)}>
          {children ?? value}
        </code>
      </pre>
    </div>
  );
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/markdown-lite.tsx
git commit -m "style: warmer code blocks with larger radius and softer labels"
```

---

## Task 10: 创建 score-stars.tsx（内联星标 + 行内气泡确认）

**Files:**
- Create: `web/src/components/session-detail/score-stars.tsx`
- Reference: `web/src/components/session-detail/session-detail-client.tsx:278-308,576-625,811-858` + spec §6.1, D2, D3

5 颗小星，未评分淡色 hover 变橙，已评分前 N 颗亮橙 + 尾部 × 清除。点击触发行内气泡 "Rate N? Yes/No" 确认。

- [ ] **Step 1: 创建 score-stars.tsx**

```tsx
// web/src/components/session-detail/score-stars.tsx

"use client";

import { useState } from "react";

interface ScoreStarsProps {
  /** Current score (1-5), or undefined if not scored. */
  score: number | undefined;
  /** Whether a score API call is in flight. */
  scoring: boolean;
  /** Called when user confirms a new score. */
  onScore: (value: number) => void;
  /** Called when user clicks the × to clear the score. */
  onClear: () => void;
  /** Star size in px. Desktop 11, mobile 9. */
  size?: number;
}

export function ScoreStars({
  score,
  scoring,
  onScore,
  onClear,
  size = 11,
}: ScoreStarsProps) {
  const [confirmValue, setConfirmValue] = useState<number | null>(null);

  // Three display states:
  // 1. score != null → show N filled stars + × clear button
  // 2. confirmValue != null → show inline "Rate N? Yes/No" bubble
  // 3. default → 5 dim stars, hover turns orange, click sets confirmValue

  if (score != null) {
    return (
      <span className="inline-flex items-center gap-1">
        <span className="inline-flex items-center gap-0.5">
          {[1, 2, 3, 4, 5].map((v) => (
            <span
              key={v}
              className={v <= score ? "text-primary" : "text-muted-foreground/30"}
              style={{ fontSize: `${size}px`, lineHeight: 1 }}
              aria-hidden
            >
              ★
            </span>
          ))}
        </span>
        <button
          type="button"
          onClick={onClear}
          disabled={scoring}
          className="ml-0.5 rounded text-muted-foreground/40 transition-colors hover:text-destructive disabled:opacity-30"
          aria-label="Remove score"
        >
          ×
        </button>
      </span>
    );
  }

  if (confirmValue != null) {
    return (
      <div className="inline-flex items-center gap-1 rounded-md border border-border bg-secondary/50 px-2 py-1">
        <span className="text-xs text-muted-foreground">
          Rate {confirmValue}?
        </span>
        <button
          type="button"
          onClick={() => {
            onScore(confirmValue);
            setConfirmValue(null);
          }}
          disabled={scoring}
          className="rounded px-1.5 py-0.5 text-xs font-medium text-foreground transition-colors hover:bg-green-500/10 hover:text-green-600 disabled:opacity-50"
        >
          Yes
        </button>
        <button
          type="button"
          onClick={() => setConfirmValue(null)}
          className="rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          No
        </button>
      </div>
    );
  }

  return (
    <span className="inline-flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((v) => (
        <button
          key={v}
          type="button"
          disabled={scoring}
          onClick={() => setConfirmValue(v)}
          className="text-muted-foreground/30 transition-colors hover:text-primary disabled:opacity-30"
          style={{ fontSize: `${size}px`, lineHeight: 1 }}
          aria-label={`Rate ${v} star${v > 1 ? "s" : ""}`}
        >
          ★
        </button>
      ))}
    </span>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/session-detail/score-stars.tsx
git commit -m "feat: add ScoreStars component with inline bubble confirmation"
```

---

## Task 11: 提取 collapsible-text.tsx 和 tool-sidebar-item.tsx

**Files:**
- Create: `web/src/components/session-detail/collapsible-text.tsx`
- Create: `web/src/components/session-detail/tool-sidebar-item.tsx`
- Reference: `web/src/components/session-detail/session-detail-client.tsx:68-206`

从 `session-detail-client.tsx` 提取这两个组件，逻辑不变，只是移到独立文件。

- [ ] **Step 1: 创建 collapsible-text.tsx**

```tsx
// web/src/components/session-detail/collapsible-text.tsx

"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

export function CollapsibleText({
  text,
  previewChars = 140,
  className,
}: {
  text: string;
  previewChars?: number;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const trimmed = text.trim();
  const isLong = trimmed.length > previewChars;
  const display =
    !isLong || open
      ? trimmed
      : `${trimmed.slice(0, previewChars).trimEnd()}…`;

  return (
    <div className={className}>
      <p className="whitespace-pre-wrap break-words">{display}</p>
      {isLong && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            setOpen((v) => !v);
          }}
          className="mt-1 inline-flex items-center gap-0.5 font-medium text-primary/90 transition-colors hover:text-primary"
        >
          {open ? "Show less" : "Show more"}
          {open ? (
            <ChevronDown className="size-3" />
          ) : (
            <ChevronRight className="size-3" />
          )}
        </button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 创建 tool-sidebar-item.tsx**

```tsx
// web/src/components/session-detail/tool-sidebar-item.tsx

"use client";

import { useState } from "react";
import {
  Braces,
  ChevronDown,
  ChevronRight,
  FileText,
  Hash,
  Wrench,
} from "lucide-react";
import type { ToolItem, UnifiedTool } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { CollapsibleText } from "./collapsible-text";

export function ToolSidebarItem({ tool }: { tool: ToolItem }) {
  const [expanded, setExpanded] = useState(false);
  const toolData: UnifiedTool = tool.tool;

  const params = toolData.parameters;
  const paramProperties =
    (params?.properties as Record<string, Record<string, unknown>>) ?? {};
  const requiredParams = (params?.required as string[]) ?? [];

  return (
    <div className="rounded-xl border border-border/70 bg-card/60">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex min-h-[52px] w-full items-center gap-3 px-3.5 py-2.5 text-left transition-colors active:bg-accent/50 md:hover:bg-accent/40"
      >
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
          <Wrench className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-mono text-[13.5px] font-medium text-foreground">
            {toolData.name}
          </p>
          <p className="truncate text-[12px] leading-snug text-muted-foreground">
            {toolData.description || "No description"}
          </p>
        </div>
        {expanded ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="space-y-3 border-t border-border/60 px-3.5 py-3">
          {toolData.description && (
            <div>
              <p className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <FileText className="size-3" />
                Description
              </p>
              <CollapsibleText
                text={toolData.description}
                previewChars={140}
                className="text-[13px] leading-relaxed text-foreground/85"
              />
            </div>
          )}
          {Object.keys(paramProperties).length > 0 && (
            <div>
              <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <Braces className="size-3" />
                Parameters
              </p>
              <div className="space-y-1.5">
                {Object.entries(paramProperties).map(([name, schema]) => (
                  <div
                    key={name}
                    className="rounded-md bg-muted/50 px-2.5 py-1.5"
                  >
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono text-[12px] font-medium text-foreground">
                        {name}
                      </span>
                      {requiredParams.includes(name) && (
                        <span className="text-[9px] font-medium uppercase tracking-wider text-rose-500">
                          required
                        </span>
                      )}
                      {schema.type !== undefined && (
                        <Badge
                          variant="secondary"
                          className="ml-auto px-1.5 py-0 font-mono text-[9px]"
                        >
                          {schema.type as string}
                        </Badge>
                      )}
                    </div>
                    {schema.description !== undefined && (
                      <CollapsibleText
                        text={schema.description as string}
                        previewChars={100}
                        className="mt-1 text-[11.5px] leading-relaxed text-muted-foreground"
                      />
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          {params?.type !== undefined && (
            <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground/60">
              <Hash className="size-3" />
              Schema type: {params.type as string}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/components/session-detail/collapsible-text.tsx web/src/components/session-detail/tool-sidebar-item.tsx
git commit -m "refactor: extract CollapsibleText and ToolSidebarItem to separate files"
```

---

## Task 12: 创建 reading-layout.tsx（共享布局骨架）

**Files:**
- Create: `web/src/components/shared/reading-layout.tsx`
- Reference: spec §4.3, §6, §7

这是核心共享组件。桌面端：sticky 顶栏 + 阅读栏 680px + 可隐藏右栏。移动端：iOS sticky header + bottom sheet。通过 slot props 注入内容。

- [ ] **Step 1: 创建 reading-layout.tsx**

```tsx
// web/src/components/shared/reading-layout.tsx

"use client";

import { type ReactNode, type Ref, type UIEvent, useRef } from "react";
import { Wrench } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import { SwipeDismissSheetBody } from "@/components/session-detail/swipe-dismiss-sheet-body";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";

export interface ReadingLayoutProps {
  /** Full header content (caller arranges back/title/actions). */
  header: ReactNode;
  /** Reading column content (message list). */
  children: ReactNode;
  /** Tools panel content (injected into right rail or bottom sheet). */
  toolsPanel: ReactNode;
  /** Whether tools panel is open. */
  toolsOpen: boolean;
  /** Toggle tools panel. */
  onToolsOpenChange: (open: boolean) => void;
  /** Tool count (>0 to render tools container). */
  toolsCount: number;
  /** Mobile: whether header should be in compact state. */
  headerCompact: boolean;
  /** Ref for the header sentinel (mobile scroll detection). */
  headerSentinelRef?: Ref<HTMLDivElement>;
  /** Mobile: messages scroll root ref for IntersectionObserver. */
  messagesScrollRootRef?: Ref<HTMLDivElement>;
  /** Mobile: messages scroll handler for infinite load. */
  onMessagesScroll?: (e: UIEvent<HTMLDivElement>) => void;
  /** Mobile: tools scroll handler for infinite load. */
  onToolsScroll?: (e: UIEvent<HTMLDivElement>) => void;
  /** Mobile: callback to capture tools scroll root node. */
  onToolsScrollRootChange?: (node: HTMLDivElement | null) => void;
}

export function ReadingLayout({
  header,
  children,
  toolsPanel,
  toolsOpen,
  onToolsOpenChange,
  toolsCount,
  headerCompact,
  headerSentinelRef,
  messagesScrollRootRef,
  onMessagesScroll,
  onToolsScroll,
  onToolsScrollRootChange,
}: ReadingLayoutProps) {
  const isMobile = useIsMobile();
  const toolsScrollRef = useRef<HTMLDivElement | null>(null);

  // ── Mobile layout: iOS sticky header + bottom sheet ──
  if (isMobile) {
    return (
      <div className="-mx-4 -mt-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
        <div ref={headerSentinelRef} aria-hidden className="h-px w-full" />

        <header
          className={cn(
            "sticky top-[-1rem] z-30 -mt-px",
            "transition-[border-color,background-color,box-shadow] duration-200 ease-out",
            "supports-[backdrop-filter]:backdrop-blur",
            headerCompact
              ? "border-b border-border bg-background/92 supports-[backdrop-filter]:bg-background/75 shadow-[0_1px_0_rgba(0,0,0,0.04)]"
              : "border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70",
          )}
        >
          <div
            className={cn(
              "flex items-center gap-1 px-2 pt-[calc(1rem+0.25rem)]",
              "transition-[padding] duration-200 ease-out",
              headerCompact ? "pb-1.5" : "pb-2",
            )}
          >
            {header}
          </div>
        </header>

        <div
          ref={messagesScrollRootRef}
          onScroll={onMessagesScroll}
          className={cn(
            "flex-1 overflow-y-auto px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]",
            "[-webkit-overflow-scrolling:touch] overscroll-contain",
          )}
        >
          {children}
        </div>

        {toolsCount > 0 && (
          <Sheet open={toolsOpen} onOpenChange={onToolsOpenChange}>
            <SheetContent
              side="bottom"
              showCloseButton={false}
              className={cn(
                "!h-[88dvh] max-h-[88dvh] rounded-t-[20px] border-border/70 p-0",
                "shadow-[0_-8px_32px_rgba(0,0,0,0.16)]",
                "flex flex-col",
                "!duration-[320ms] !ease-[cubic-bezier(0.32,0.72,0,1)]",
                "data-[side=bottom]:data-starting-style:!translate-y-[100%]",
                "data-[side=bottom]:data-ending-style:!translate-y-[100%]",
              )}
            >
              <SwipeDismissSheetBody
                onDismiss={() => onToolsOpenChange(false)}
                title="Available Tools"
                count={toolsCount}
                onScroll={onToolsScroll}
                onScrollRootChange={onToolsScrollRootChange}
              >
                {toolsPanel}
              </SwipeDismissSheetBody>
            </SheetContent>
          </Sheet>
        )}
      </div>
    );
  }

  // ── Desktop layout: sticky header + reading column + collapsible right rail ──
  return (
    <div className="-mx-4 -mt-4 flex min-h-[calc(100vh-6rem)] flex-col bg-background md:-mx-8 md:-mt-8 lg:-mx-10 lg:-mt-10">
      <header className="sticky top-[-1rem] z-30 border-b border-border/70 bg-background/95 supports-[backdrop-filter]:backdrop-blur md:top-[-2rem] lg:top-[-2.5rem]">
        <div className="mx-auto flex max-w-[680px] items-center gap-3 px-4 pt-[calc(1rem+0.25rem)] pb-3 md:pt-[calc(2rem+0.25rem)] lg:pt-[calc(2.5rem+0.25rem)]">
          {header}
        </div>
      </header>

      <div className="flex flex-1">
        {/* Reading column */}
        <div className="mx-auto w-full max-w-[680px] flex-1 px-4 py-6 sm:px-6">
          {children}
        </div>

        {/* Collapsible right rail */}
        {toolsCount > 0 && (
          <aside
            className={cn(
              "border-l border-border/70 bg-card overflow-hidden transition-[width] duration-200 ease-out",
              toolsOpen ? "w-[280px]" : "w-0",
            )}
          >
            <div
              className="flex h-full w-[280px] flex-col"
              style={{ visibility: toolsOpen ? "visible" : "hidden" }}
            >
              <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
                <div className="flex items-center gap-2">
                  <Wrench className="size-4 text-muted-foreground" />
                  <span className="font-display text-[14px] font-semibold text-foreground">
                    Available Tools
                  </span>
                  <Badge variant="secondary" className="text-[10px]">
                    {toolsCount}
                  </Badge>
                </div>
                <button
                  type="button"
                  onClick={() => onToolsOpenChange(false)}
                  className="text-[14px] text-muted-foreground transition-colors hover:text-foreground"
                  aria-label="Close tools panel"
                >
                  ✕
                </button>
              </div>
              <div
                ref={toolsScrollRef}
                className="flex-1 space-y-2 overflow-y-auto p-4"
              >
                {toolsPanel}
              </div>
            </div>
          </aside>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/shared/reading-layout.tsx
git commit -m "feat: add ReadingLayout shared component for desktop+mobile layouts"
```

---

## Task 13: 创建 tools-rail.tsx（桌面右栏内容）

**Files:**
- Create: `web/src/components/session-detail/tools-rail.tsx`

这只是右栏内容的 slot 包装器——工具列表 + 滚动加载 sentinel。`ReadingLayout` 负责右栏容器（边框/宽度/标题/关闭），本组件只提供内容。

- [ ] **Step 1: 创建 tools-rail.tsx**

```tsx
// web/src/components/session-detail/tools-rail.tsx

"use client";

import { type Ref } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import type { ToolItem } from "@/lib/types";
import { ToolSidebarItem } from "./tool-sidebar-item";

export interface ToolsRailProps {
  tools: ToolItem[];
  hasMore: boolean;
  sentinelRef?: Ref<HTMLDivElement>;
}

export function ToolsRail({ tools, hasMore, sentinelRef }: ToolsRailProps) {
  return (
    <div className="space-y-2">
      {tools.map((t) => (
        <ToolSidebarItem key={t.id} tool={t} />
      ))}
      {hasMore && (
        <div ref={sentinelRef} className="flex justify-center py-3">
          <Skeleton className="h-4 w-24" />
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/components/session-detail/tools-rail.tsx
git commit -m "feat: add ToolsRail component for desktop right rail content"
```

---

## Task 14: 重写 session-detail-client.tsx

**Files:**
- Rewrite: `web/src/components/session-detail/session-detail-client.tsx`

968 行 → ~200 行。用 `ReadingLayout` + `ScoreStars` + `ToolsRail` 组合。数据加载逻辑保留。移动端 header 的 IntersectionObserver 逻辑保留。

- [ ] **Step 1: 重写 session-detail-client.tsx**

```tsx
// web/src/components/session-detail/session-detail-client.tsx

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  MessagesSquare,
  Share2,
  Trash2,
  Wrench,
  ChevronRight,
} from "lucide-react";
import { api } from "@/lib/api-client";
import type { SessionMetadata, MessageItem, ToolItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { ShareDialog } from "@/components/share/share-dialog";
import { useIsMobile } from "@/hooks/use-mobile";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { SessionHistoryList } from "./session-history-list";
import { ScoreStars } from "./score-stars";
import { ToolsRail } from "./tools-rail";
import { ReadingLayout } from "@/components/shared/reading-layout";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { AlertTriangle } from "lucide-react";

export default function SessionDetailClient({
  sessionId,
}: {
  sessionId: number;
}) {
  const router = useRouter();
  const isMobile = useIsMobile();
  const [metadata, setMetadata] = useState<SessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [toolsOpen, setToolsOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [score, setScore] = useState<number | undefined>(undefined);
  const [scoring, setScoring] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsScrollRootRef = useRef<HTMLDivElement | null>(null);

  /* eslint-disable react-hooks/set-state-in-effect -- IntersectionObserver callback inherently sets state on visibility changes */
  useEffect(() => {
    if (!isMobile) {
      setHeaderCompact(false);
      return;
    }
    const sentinel = headerSentinelRef.current;
    if (!sentinel) return;
    const io = new IntersectionObserver(
      ([entry]) => setHeaderCompact(!entry.isIntersecting),
      { threshold: 0, rootMargin: "0px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
  }, [isMobile, loading, metadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const fetchMetadata = useCallback(async () => {
    if (!sessionId || Number.isNaN(sessionId)) return;
    setLoading(true);
    try {
      const rsp = await api.getSessionMetadata(sessionId);
      if (rsp.session) {
        setMetadata(rsp.session);
        setScore(rsp.session.score);
      }
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  const handleDelete = useCallback(async () => {
    setDeleting(true);
    try {
      await api.deleteSession(sessionId);
      toast.success("Session deleted");
      router.push("/sessions/");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete session");
    } finally {
      setDeleting(false);
      setDeleteConfirmOpen(false);
    }
  }, [sessionId, router]);

  const handleScore = useCallback(
    async (value: number) => {
      if (!sessionId || scoring) return;
      setScoring(true);
      try {
        await api.scoreSession({ sessionId, score: value });
        setScore(value);
        toast.success("Scored");
      } catch {
        toast.error("Failed to score");
      } finally {
        setScoring(false);
      }
    },
    [sessionId, scoring],
  );

  const handleDeleteScore = useCallback(async () => {
    if (!sessionId || scoring) return;
    setScoring(true);
    try {
      await api.deleteScoreSession(sessionId);
      setScore(undefined);
      toast.success("Score removed");
    } catch {
      toast.error("Failed to remove score");
    } finally {
      setScoring(false);
    }
  }, [sessionId, scoring]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const listEnabled =
    !!sessionId && !Number.isNaN(sessionId) && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    ((!isMobile && toolsOpen) || (isMobile && toolsOpen));

  const messagesList = useInfiniteList<MessageItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listSessionMessages(sessionId, page, limit);
        return {
          items: rsp.messages ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 50,
    enabled: listEnabled,
  });

  const toolsList = useInfiniteList<ToolItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listSessionTools(sessionId, page, limit);
        return {
          items: rsp.tools ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 20,
    enabled: toolsListEnabled,
  });

  // messages 滚动加载 sentinel
  useEffect(() => {
    const root = isMobile ? messagesScrollRootRef.current : null;
    const sentinel = messagesSentinelRef.current;
    if (!sentinel || !messagesList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void messagesList.loadMore();
        }
      },
      { root, rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isMobile, messagesList.hasMore, messagesList.loadMore]);

  // tools 滚动加载 sentinel (desktop: right rail scroll; mobile: bottom sheet scroll)
  useEffect(() => {
    const sentinel = toolsSentinelRef.current;
    if (!sentinel || !toolsList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void toolsList.loadMore();
        }
      },
      { root: toolsScrollRootRef.current, rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [toolsOpen, isMobile, toolsList.hasMore, toolsList.loadMore]);

  const messages = messagesList.items;
  const tools = toolsList.items;
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  if (!sessionId || Number.isNaN(sessionId)) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">Invalid session ID</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/sessions/")}
        >
          Back to Sessions
        </Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="mx-auto w-full max-w-[680px] space-y-5 py-6">
        <Skeleton className="h-8 w-48" />
        <div className="space-y-5">
          <Skeleton className="ml-auto h-20 w-3/4 rounded-[20px]" />
          <Skeleton className="h-32 w-full rounded-xl" />
          <Skeleton className="ml-auto h-16 w-2/3 rounded-[20px]" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
      </div>
    );
  }

  if (!metadata) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">Session not found</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/sessions/")}
        >
          Back to Sessions
        </Button>
      </div>
    );
  }

  const messageCount = metadata.messageCount;

  // ── Header content (shared by desktop + mobile via ReadingLayout) ──
  const headerContent = (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => router.push("/sessions/")}
        className="size-10 text-foreground/70 hover:text-foreground"
        aria-label="Back to sessions"
      >
        <ArrowLeft className="size-5" />
      </Button>

      {/* Mobile: history sheet trigger */}
      {isMobile && (
        <Sheet>
          <SheetTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                className="size-10 text-foreground/70 hover:text-foreground"
                aria-label="Session history"
              />
            }
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="size-5"
            >
              <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
              <path d="M3 3v5h5" />
              <path d="M12 7v5l4 2" />
            </svg>
          </SheetTrigger>
          <SheetContent
            side="bottom"
            showCloseButton={false}
            className="h-[80dvh] max-h-[80dvh] rounded-t-[20px] border-border/70 p-0"
          >
            <div className="flex h-full flex-col">
              <SheetHeader className="border-b border-border/60 px-4 py-3 text-left">
                <SheetTitle>History</SheetTitle>
              </SheetHeader>
              <div className="min-h-0 flex-1">
                <SessionHistoryList
                  activeSessionId={sessionId}
                  onSelect={(id) => router.push(`/sessions/detail?id=${id}`)}
                />
              </div>
            </div>
          </SheetContent>
        </Sheet>
      )}

      <div className="flex min-w-0 flex-1 flex-col items-center px-1 leading-tight">
        <h1
          className={[
            "truncate font-display font-semibold tracking-tight text-foreground",
            "transition-[font-size] duration-200 ease-out",
            isMobile && headerCompact ? "text-[14px]" : "text-[15px]",
          ].filter(Boolean).join(" ")}
        >
          Session #{metadata.id}
        </h1>
        <p
          className={[
            "truncate text-[11px] text-muted-foreground",
            "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
            isMobile && headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
          ].filter(Boolean).join(" ")}
        >
          {messageCount} message{messageCount === 1 ? "" : "s"}
          {metadata.apiKeyName ? ` · ${metadata.apiKeyName}` : ""}
        </p>
      </div>

      <ScoreStars
        score={score}
        scoring={scoring}
        onScore={handleScore}
        onClear={handleDeleteScore}
        size={isMobile ? 9 : 11}
      />

      <Button
        variant={metadata.shareID ? "secondary" : "ghost"}
        size="icon-sm"
        onClick={() => setShareOpen(true)}
        className={[
          "size-10",
          metadata.shareID
            ? "text-primary"
            : "text-foreground/70 hover:text-foreground",
        ].join(" ")}
        aria-label={metadata.shareID ? "Manage share link" : "Share session"}
        title={metadata.shareID ? "Shared" : "Share"}
      >
        <Share2 className="size-5" />
      </Button>

      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setDeleteConfirmOpen(true)}
        className="size-10 text-foreground/70 hover:text-destructive"
        aria-label="Delete session"
        title="Delete session"
      >
        <Trash2 className="size-5" />
      </Button>

      {metadata.toolCount > 0 && (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setToolsOpen((v) => !v)}
          className={[
            "relative size-10",
            toolsOpen
              ? "bg-secondary text-foreground"
              : "text-foreground/70 hover:text-foreground",
          ].join(" ")}
          aria-label="Toggle available tools"
          title="Available tools"
        >
          <Wrench className="size-5" />
          <span
            className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold tabular-nums text-primary-foreground"
            aria-hidden
          >
            {metadata.toolCount}
          </span>
        </Button>
      )}
    </>
  );

  // ── Reading column content ──
  const readingContent = (
    <>
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">
            No messages in this session
          </p>
        </div>
      ) : (
        <div className="space-y-5">
          {messages.map((msg, idx) => (
            <ChatMessage
              key={msg.id}
              message={msg}
              index={idx}
              toolResultsByID={toolResultsByID}
            />
          ))}
          {messagesList.hasMore && (
            <div
              ref={messagesSentinelRef}
              className="flex justify-center py-3"
            >
              <Skeleton className="h-4 w-32" />
            </div>
          )}
          {!messagesList.hasMore && messages.length > 0 && (
            <div className="pt-3 pb-1 text-center">
              <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                end of conversation
              </span>
            </div>
          )}
        </div>
      )}
    </>
  );

  // ── Tools panel content ──
  const toolsPanelContent = (
    <ToolsRail
      tools={tools}
      hasMore={toolsList.hasMore}
      sentinelRef={toolsSentinelRef}
    />
  );

  const setToolsScrollRoot = useCallback(
    (node: HTMLDivElement | null) => {
      toolsScrollRootRef.current = node;
    },
    [],
  );

  return (
    <>
      <ReadingLayout
        header={headerContent}
        toolsPanel={toolsPanelContent}
        toolsOpen={toolsOpen}
        onToolsOpenChange={setToolsOpen}
        toolsCount={metadata.toolCount}
        headerCompact={isMobile && headerCompact}
        headerSentinelRef={headerSentinelRef}
        messagesScrollRootRef={messagesScrollRootRef}
        onToolsScrollRootChange={setToolsScrollRoot}
      >
        {readingContent}
      </ReadingLayout>

      <ShareDialog
        sessionId={metadata.id}
        existingShareID={metadata.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />

      <AlertDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="size-5 text-destructive" />
              Are you sure?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete this session and all its messages.
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: 验证 build 通过**

Run: `cd web && npm run build`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/components/session-detail/session-detail-client.tsx
git commit -m "refactor: rewrite session-detail-client using ReadingLayout + ScoreStars + ToolsRail"
```

---

## Task 15: 重写 share/page.tsx

**Files:**
- Rewrite: `web/src/app/share/page.tsx`

641 行 → ~250 行。用 `ReadingLayout`，去掉左侧元数据栏。顶栏只有标题 + 创建时间/消息数 + 工具按钮。保留错误状态机和 IP 限流处理。

- [ ] **Step 1: 重写 share/page.tsx**

```tsx
// web/src/app/share/page.tsx

"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type UIEvent,
} from "react";
import { useSearchParams } from "next/navigation";
import { Clock, MessagesSquare, Share2, Wrench } from "lucide-react";

import { api, ApiError } from "@/lib/api-client";
import type { ShareSessionMetadata } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { ToolSidebarItem } from "@/components/session-detail/tool-sidebar-item";
import { useIsMobile } from "@/hooks/use-mobile";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { formatRelativeTime } from "@/lib/utils";
import { ReadingLayout } from "@/components/shared/reading-layout";

// ─── Empty / error states ──────────────────────────────────────────────────

type ShareError =
  | { kind: "missing-id" }
  | { kind: "rate-limited" }
  | { kind: "not-found" }
  | { kind: "unknown"; message: string };

function ShareErrorView({ error }: { error: ShareError }) {
  const { title, description } = (() => {
    switch (error.kind) {
      case "missing-id":
        return {
          title: "Invalid share link",
          description:
            "This share link is missing a required identifier. Please ask the sender for a fresh link.",
        };
      case "rate-limited":
        return {
          title: "Too many requests",
          description:
            "You've opened this link too frequently. Please wait a moment and try again.",
        };
      case "not-found":
        return {
          title: "Link expired or unavailable",
          description:
            "This share link is no longer valid. It may have expired (after 24 hours) or been revoked by the owner.",
        };
      default:
        return {
          title: "Unable to load shared session",
          description: error.message,
        };
    }
  })();

  return (
    <div className="flex min-h-[100dvh] items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-3xl border bg-card p-8 text-center shadow-[0_24px_70px_rgba(92,62,29,0.14)]">
        <div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-muted">
          <Share2 className="size-5 text-muted-foreground" />
        </div>
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
    </div>
  );
}

// ─── Loading skeleton ──────────────────────────────────────────────────────

function ShareLoading() {
  return (
    <div className="mx-auto w-full max-w-[680px] space-y-5 px-4 py-10 sm:px-6">
      <Skeleton className="h-8 w-48" />
      <div className="space-y-5">
        <Skeleton className="ml-auto h-20 w-3/4 rounded-[20px]" />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="ml-auto h-16 w-2/3 rounded-[20px]" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    </div>
  );
}

// ─── Main view ─────────────────────────────────────────────────────────────

function SharedSessionView() {
  const searchParams = useSearchParams();
  const shareID = searchParams.get("id") ?? "";
  const isMobile = useIsMobile();

  const [metadata, setMetadata] = useState<ShareSessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ShareError | null>(null);
  const [toolsOpen, setToolsOpen] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const toolsScrollRootRef = useRef<HTMLDivElement | null>(null);

  const fetchMetadata = useCallback(async () => {
    if (!shareID) {
      setError({ kind: "missing-id" });
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const rsp = await api.getShareMetadata(shareID);
      if (rsp.error || !rsp.session) {
        setError({ kind: "not-found" });
        return;
      }
      setMetadata(rsp.session);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 404) {
          setError({ kind: "not-found" });
        } else if (err.status === 429) {
          setError({ kind: "rate-limited" });
        } else {
          setError({
            kind: "unknown",
            message: `Request failed (${err.status})`,
          });
        }
      } else {
        setError({
          kind: "unknown",
          message:
            err instanceof Error ? err.message : "Unexpected network error",
        });
      }
    } finally {
      setLoading(false);
    }
  }, [shareID]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const listEnabled = !!shareID && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    toolsOpen;

  const messagesList = useInfiniteList({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listShareMessages(shareID, page, limit);
        return {
          items: rsp.messages ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [shareID],
    ),
    pageSize: 50,
    enabled: listEnabled,
  });

  const toolsList = useInfiniteList({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listShareTools(shareID, page, limit);
        return {
          items: rsp.tools ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [shareID],
    ),
    pageSize: 20,
    enabled: toolsListEnabled,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- IntersectionObserver callback inherently sets state on visibility changes */
  useEffect(() => {
    if (!isMobile) {
      setHeaderCompact(false);
      return;
    }
    const sentinel = headerSentinelRef.current;
    if (!sentinel) return;
    const io = new IntersectionObserver(
      ([entry]) => setHeaderCompact(!entry.isIntersecting),
      { threshold: 0, rootMargin: "0px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
  }, [isMobile, loading, metadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  useEffect(() => {
    const sentinel = messagesSentinelRef.current;
    if (!sentinel || !messagesList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void messagesList.loadMore();
        }
      },
      {
        root: messagesScrollRootRef.current,
        rootMargin: "200px",
      },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isMobile, messagesList.hasMore, messagesList.loadMore]);

  useEffect(() => {
    const sentinel = toolsSentinelRef.current;
    if (!sentinel || !toolsList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void toolsList.loadMore();
        }
      },
      {
        root: toolsScrollRootRef.current,
        rootMargin: "200px",
      },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [toolsOpen, isMobile, toolsList.hasMore, toolsList.loadMore]);

  const messages = messagesList.items;
  const tools = toolsList.items;
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  if (error) return <ShareErrorView error={error} />;
  if (loading) return <ShareLoading />;
  if (!metadata) return <ShareErrorView error={{ kind: "not-found" }} />;

  // ── Header content ──
  const headerContent = (
    <>
      <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
        <Share2 className="size-[18px]" />
      </div>
      <div className="flex min-w-0 flex-1 flex-col items-center leading-tight">
        <h1
          className={[
            "truncate font-display font-semibold tracking-tight text-foreground",
            "transition-[font-size] duration-200 ease-out",
            isMobile && headerCompact ? "text-[14px]" : "text-[15px]",
          ].filter(Boolean).join(" ")}
        >
          Shared session #{metadata.id}
        </h1>
        <p
          className={[
            "truncate text-[11px] text-muted-foreground",
            "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
            isMobile && headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
          ].filter(Boolean).join(" ")}
        >
          {formatRelativeTime(metadata.createdAt)} · {metadata.messageCount} message{metadata.messageCount === 1 ? "" : "s"}
        </p>
      </div>
      {metadata.toolCount > 0 && (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setToolsOpen((v) => !v)}
          className={[
            "relative size-10 shrink-0",
            toolsOpen
              ? "bg-secondary text-foreground"
              : "text-foreground/70 hover:text-foreground",
          ].join(" ")}
          aria-label="Toggle available tools"
          title="Available tools"
        >
          <Wrench className="size-5" />
          <span
            className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold tabular-nums text-primary-foreground"
            aria-hidden
          >
            {metadata.toolCount}
          </span>
        </Button>
      )}
    </>
  );

  // ── Reading column content ──
  const readingContent = (
    <>
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">
            No messages in this session
          </p>
        </div>
      ) : (
        <div className="space-y-5">
          {messages.map((msg, idx) => (
            <ChatMessage
              key={msg.id}
              message={msg}
              index={idx}
              toolResultsByID={toolResultsByID}
            />
          ))}
          {messagesList.hasMore && (
            <div
              ref={messagesSentinelRef}
              className="flex justify-center py-3"
            >
              <Skeleton className="h-4 w-32" />
            </div>
          )}
          {!messagesList.hasMore && messages.length > 0 && (
            <div className="pt-3 pb-1 text-center">
              <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                end of conversation
              </span>
            </div>
          )}
        </div>
      )}
    </>
  );

  // ── Tools panel content ──
  const toolsPanelContent = (
    <div className="space-y-2">
      {tools.map((t) => (
        <ToolSidebarItem key={t.id} tool={t} />
      ))}
      {toolsList.hasMore && (
        <div ref={toolsSentinelRef} className="flex justify-center py-3">
          <Skeleton className="h-4 w-24" />
        </div>
      )}
    </div>
  );

  const setToolsScrollRoot = useCallback((node: HTMLDivElement | null) => {
    toolsScrollRootRef.current = node;
  }, []);

  return (
    <ReadingLayout
      header={headerContent}
      toolsPanel={toolsPanelContent}
      toolsOpen={toolsOpen}
      onToolsOpenChange={setToolsOpen}
      toolsCount={metadata.toolCount}
      headerCompact={isMobile && headerCompact}
      headerSentinelRef={headerSentinelRef}
      messagesScrollRootRef={messagesScrollRootRef}
      onToolsScrollRootChange={setToolsScrollRoot}
    >
      {readingContent}
    </ReadingLayout>
  );
}

export default function SharedSessionPage() {
  return (
    <Suspense fallback={<ShareLoading />}>
      <SharedSessionView />
    </Suspense>
  );
}
```

- [ ] **Step 2: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 3: 验证 build 通过**

Run: `cd web && npm run build`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/app/share/page.tsx
git commit -m "refactor: rewrite share page using ReadingLayout, remove left metadata sidebar"
```

---

## Task 16: 删除 tool-drawer.tsx + 最终验证

**Files:**
- Delete: `web/src/components/session-detail/tool-drawer.tsx`

`tool-drawer.tsx` 被 `tools-rail.tsx` + `ReadingLayout` 右栏 slot 取代。需确认无其他文件引用它。

- [ ] **Step 1: 检查 tool-drawer.tsx 是否还被引用**

Run: `cd web && grep -r "tool-drawer\|ToolDrawer" src/ --include="*.tsx" --include="*.ts"`
Expected: 无输出（或仅 tool-drawer.tsx 自身）。如果有其他引用，需先更新那些引用再删除。

- [ ] **Step 2: 删除 tool-drawer.tsx**

```bash
rm web/src/components/session-detail/tool-drawer.tsx
```

- [ ] **Step 3: 验证 lint 通过**

Run: `cd web && npm run lint`
Expected: PASS

- [ ] **Step 4: 验证 build 通过**

Run: `cd web && npm run build`
Expected: PASS（静态导出成功，无类型错误）

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: remove tool-drawer.tsx (replaced by ToolsRail + ReadingLayout)"
```

---

## Task 17: 同步前端产物 + 最终 build 验证

**Files:**
- Run: `make web-build`（构建前端并同步到 `internal/web/dist/`）

AGENTS.md §12.4 要求：改动后至少运行 `cd web && npm run lint && npm run build` 验证。`make web-build` 会自动跑 `npm run build` 并把 `web/out/` 拷贝到 `internal/web/dist/`。

- [ ] **Step 1: 运行 make web-build**

Run: `make web-build`
Expected: 成功输出 `web/out/` → `internal/web/dist/` 同步完成

- [ ] **Step 2: 运行 make build（Go 编译，会先跑 web-build）**

Run: `make build`
Expected: 成功生成二进制（前端已 embed）

- [ ] **Step 3: Commit（如有 internal/web/dist/ 变化）**

```bash
git add -A
git commit -m "build: sync web dist for Claude Work style session/share pages"
```

如果 `internal/web/dist/` 被 `.gitignore` 排除（AGENTS.md §12.1 提到已 gitignore），则跳过此 commit。

---

## Task 18: 本地视觉验证（Chrome MCP）

**Files:** 无代码改动，纯验证

按 spec §9 的验证计划，本地启动后端 + 前端，用 Chrome MCP 访问页面对比 Claude Work。

- [ ] **Step 1: 启动后端**

Run: `go run main.go server start --host localhost --port 8080`
Expected: 服务在 localhost:8080 启动

- [ ] **Step 2: 启动前端 dev server**

Run: `cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev`
Expected: 前端在 localhost:3000 启动

- [ ] **Step 3: Chrome MCP 验证桌面端 session 详情页**

用 Chrome MCP 导航到 `http://localhost:3000/web/sessions/`，登录后点进一个有工具调用的 session 详情页。

验证项：
- 顶栏：返回 + 标题 + 消息数 + 内联星标 + 分享 + 删除 + 工具按钮
- 阅读栏居中 ~680px，消息间距 space-y-5
- 用户气泡：bg-accent 暖粉，圆角 20px
- 助手消息：头像 + 时间在下方，无 "AI" 文字标签
- thinking 块：透明背景 + 左侧细线
- 工具卡片：中性边框，折叠态有参数预览
- 代码块：背景 #26211C，圆角 12px，暖色边框
- 点击工具按钮 → 右栏展开 280px
- 点击星标 → 行内气泡 "Rate N? Yes/No"

- [ ] **Step 4: Chrome MCP 验证移动端 session 详情页**

用 Chrome MCP emulate 移动端视口（375px），刷新页面。

验证项：
- iOS 风格 sticky header（背板模糊、滚动收缩）
- 顶栏内容同桌面但尺寸略小
- 工具点击打开 bottom sheet
- 消息渲染同桌面

- [ ] **Step 5: Chrome MCP 验证分享页**

导航到 `http://localhost:3000/web/share/?id=<有效 shareID>`。

验证项：
- 顶栏：Share 图标 + 标题 + 创建时间/消息数 + 工具按钮（无返回/评分/删除）
- 阅读栏居中，与 session 详情页风格一致
- 无左侧元数据栏
- 工具右栏可隐藏

- [ ] **Step 6: Chrome MCP 验证暗色模式**

在 Chrome MCP 中切换暗色模式，刷新两个页面。

验证项：
- 暖色 token 在暗色模式下观感正常
- 代码块 `dark:bg-[#1F1A14]` 生效
- 用户气泡 `bg-accent` 在暗色下可读

- [ ] **Step 7: 回归测试**

在桌面端 session 详情页验证：
- 评分：点击星标 → 确认 → toast "Scored" → 星标显示已评分
- 清除评分：点击 × → toast "Score removed"
- 分享：点击分享 → ShareDialog 弹出 → 创建链接 → 复制
- 删除：点击删除 → 确认弹窗 → 确认 → 跳转回列表
- 工具列表：展开右栏 → 工具列表加载 → 滚动到底加载更多
- 消息无限滚动：滚动到底部 → 加载更多消息
- reasoning 折叠/展开
- tool call 卡片折叠/展开
- 代码块复制按钮

---

## 自审

### 1. Spec coverage

| Spec 章节 | 覆盖任务 |
|-----------|---------|
| §4.2 新组件结构 | Task 1-7, 10-13 |
| §4.3 ReadingLayout slot 契约 | Task 12 |
| §5.1 去掉 YOU/AI 标签 | Task 5, 6, 7 |
| §5.2 用户气泡更暖更圆润 | Task 5 |
| §5.3 Thinking 块更轻 | Task 3 |
| §5.4 工具卡片紧凑+预览 | Task 4 |
| §5.5 代码块更暖 | Task 9 |
| §5.6 间距/行距节奏 | Task 5, 6, 12, 14, 15 |
| §6 桌面端布局 B2 | Task 12, 14 |
| §7 移动端布局 | Task 12, 14, 15 |
| §8 数据流与逻辑保留 | Task 14, 15 |
| §9 验证计划 | Task 17, 18 |
| 删除 tool-drawer.tsx | Task 16 |

### 2. Placeholder scan

无占位符。所有代码步骤包含完整可执行代码。所有命令包含预期输出。

### 3. Type consistency

- `ContentPart` / `ExtractedContent` 在 Task 1 定义，Task 2-6 引用 ✓
- `buildToolResultsByID` 在 Task 1 定义，Task 8 re-export，Task 14/15 引用 ✓
- `ScoreStarsProps` 在 Task 10 定义，Task 14 引用 ✓
- `ReadingLayoutProps` 在 Task 12 定义，Task 14/15 引用 ✓
- `ToolsRailProps` 在 Task 13 定义，Task 14 引用 ✓
- `ToolSidebarItem` 在 Task 11 定义，Task 13/15 引用 ✓
- `CollapsibleText` 在 Task 11 定义，Task 11 (tool-sidebar-item) 引用 ✓
