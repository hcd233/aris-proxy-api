# Provider 图标替换实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Endpoints 和 Audit 页面用 OpenAI / Anthropic 品牌图标替换 provider 相关文字。

**Architecture:** 新增一个纯展示组件 `ProviderIcon`，根据 protocol 字符串前缀决定渲染 OpenAI 或 Anthropic 图标；Endpoints 和 Audit 页面直接引入该组件替换现有文字前缀。

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, Tailwind v4, shadcn/ui, Lucide（品牌图标使用内联 SVG）

---

## 文件清单

- 创建：`web/src/components/provider-icon.tsx`
- 修改：`web/src/app/(dashboard)/endpoints/page.tsx`
- 修改：`web/src/app/(dashboard)/audit/page.tsx`

---

## Task 1: 创建 ProviderIcon 组件

**Files:**
- 创建：`web/src/components/provider-icon.tsx`

- [ ] **Step 1: 写入 ProviderIcon 组件代码**

```tsx
"use client";

import { cn } from "@/lib/utils";

function OpenAIIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 256 256"
      fill="currentColor"
      className={cn("shrink-0", className)}
      aria-label="OpenAI"
      role="img"
    >
      <path d="M224.32 114.24a56 56 0 0 0-60.07-76.57a56 56 0 0 0-96.32 13.77a56 56 0 0 0-36.25 90.32A56 56 0 0 0 69 217a56.4 56.4 0 0 0 14.59 2a56 56 0 0 0 8.17-.61a56 56 0 0 0 96.31-13.78a56 56 0 0 0 36.25-90.32Zm-41.47-59.81a40 40 0 0 1 28.56 48a51 51 0 0 0-2.91-1.81L164 74.88a8 8 0 0 0-8 0l-44 25.41V81.81l40.5-23.38a39.76 39.76 0 0 1 30.35-4M144 137.24l-16 9.24l-16-9.24v-18.48l16-9.24l16 9.24ZM80 72a40 40 0 0 1 67.53-29c-1 .51-2 1-3 1.62L100 70.27a8 8 0 0 0-4 6.92V128l-16-9.24ZM40.86 86.93a39.75 39.75 0 0 1 23.26-18.36A56 56 0 0 0 64 72v51.38a8 8 0 0 0 4 6.93l44 25.4L96 165l-40.5-23.43a40 40 0 0 1-14.64-54.64m32.29 114.64a40 40 0 0 1-28.56-48c.95.63 1.91 1.24 2.91 1.81L92 181.12a8 8 0 0 0 8 0l44-25.41v18.48l-40.5 23.38a39.76 39.76 0 0 1-30.35 4M176 184a40 40 0 0 1-67.52 29.05c1-.51 2-1.05 3-1.63L156 185.73a8 8 0 0 0 4-6.92V128l16 9.24Zm39.14-14.93a39.75 39.75 0 0 1-23.26 18.36c.07-1.14.12-2.28.12-3.43v-51.38a8 8 0 0 0-4-6.93l-44-25.4l16-9.24l40.5 23.38a40 40 0 0 1 14.64 54.64" />
    </svg>
  );
}

function AnthropicIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={cn("shrink-0", className)}
      aria-label="Anthropic"
      role="img"
    >
      <path d="M17.304 3.541h-3.672l6.696 16.918H24Zm-10.608 0L0 20.459h3.744l1.37-3.553h7.005l1.369 3.553h3.744L10.536 3.541Zm-.371 10.223L8.616 7.82l2.291 5.945Z" />
    </svg>
  );
}

export function ProviderIcon({
  protocol,
  className,
}: {
  protocol: string;
  className?: string;
}) {
  const normalized = protocol.toLowerCase();
  if (normalized.startsWith("openai")) {
    return <OpenAIIcon className={className} />;
  }
  if (normalized.startsWith("anthropic")) {
    return <AnthropicIcon className={className} />;
  }
  return null;
}
```

- [ ] **Step 2: 验证组件文件无语法错误**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npx tsc --noEmit --pretty
```

Expected: 无 `provider-icon.tsx` 相关报错。

- [ ] **Step 3: Commit 组件文件**

```bash
git add web/src/components/provider-icon.tsx
git commit -m "feat(web): add OpenAI/Anthropic provider icon component"
```

---

## Task 2: 替换 Endpoints 页面的 provider 文字

**Files:**
- 修改：`web/src/app/(dashboard)/endpoints/page.tsx`

- [ ] **Step 1: 导入 ProviderIcon**

在 `web/src/app/(dashboard)/endpoints/page.tsx` 顶部，现有 import 区块之后加入：

```tsx
import { ProviderIcon } from "@/components/provider-icon";
```

- [ ] **Step 2: 修改桌面端 Supported APIs 列**

定位到桌面端 Table 的 `<TableCell>`（Supported APIs），将三个 Badge 替换为图标 + 短文字：

```tsx
<div className="flex flex-wrap gap-1.5">
  {ep.supportOpenAIChatCompletion && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="openai-chat-completion" className="size-3.5" />
      Chat Completions
    </Badge>
  )}
  {ep.supportOpenAIResponse && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="openai-response" className="size-3.5" />
      Response
    </Badge>
  )}
  {ep.supportAnthropicMessage && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="anthropic-message" className="size-3.5" />
      Messages
    </Badge>
  )}
</div>
```

- [ ] **Step 3: 修改移动端 Supported APIs 卡片**

定位到 `isMobile` 分支中的 capabilities 区域，同样替换：

```tsx
<div className="mt-2 flex flex-wrap gap-1.5">
  {ep.supportOpenAIChatCompletion && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="openai-chat-completion" className="size-3.5" />
      OpenAI / Chat
    </Badge>
  )}
  {ep.supportOpenAIResponse && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="openai-response" className="size-3.5" />
      OpenAI / Response
    </Badge>
  )}
  {ep.supportAnthropicMessage && (
    <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
      <ProviderIcon protocol="anthropic-message" className="size-3.5" />
      Anthropic / Messages
    </Badge>
  )}
</div>
```

- [ ] **Step 4: 修改 Base URL 列的 O/A 前缀**

定位到 Base URL tooltip 区域，将 `O:` / `A:` 文字前缀替换为图标：

```tsx
<div className="space-y-0.5">
  <div className="flex items-center gap-1.5 truncate font-mono text-xs text-muted-foreground">
    <ProviderIcon protocol="openai-chat-completion" className="size-3.5" />
    {ep.openaiBaseURL || "—"}
  </div>
  <div className="flex items-center gap-1.5 truncate font-mono text-xs text-muted-foreground">
    <ProviderIcon protocol="anthropic-message" className="size-3.5" />
    {ep.anthropicBaseURL || "—"}
  </div>
</div>
```

Tooltip content 区域同步替换：

```tsx
<div className="space-y-1 font-mono text-xs">
  <p className="flex items-center gap-1.5">
    <ProviderIcon protocol="openai-chat-completion" className="size-3.5" />
    {ep.openaiBaseURL || "—"}
  </p>
  <p className="flex items-center gap-1.5">
    <ProviderIcon protocol="anthropic-message" className="size-3.5" />
    {ep.anthropicBaseURL || "—"}
  </p>
</div>
```

- [ ] **Step 5: 验证 Endpoints 页面无类型错误**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npx tsc --noEmit --pretty
```

Expected: 无新增报错。

- [ ] **Step 6: Commit Endpoints 改动**

```bash
git add web/src/app/(dashboard)/endpoints/page.tsx
git commit -m "feat(web): replace provider labels with icons on endpoints page"
```

---

## Task 3: 替换 Audit 页面的 Protocol 列

**Files:**
- 修改：`web/src/app/(dashboard)/audit/page.tsx`

- [ ] **Step 1: 导入 ProviderIcon**

在 `web/src/app/(dashboard)/audit/page.tsx` 顶部加入：

```tsx
import { ProviderIcon } from "@/components/provider-icon";
```

- [ ] **Step 2: 修改桌面端 Protocol 列**

定位到 Protocol `<TableCell>`，替换为：

```tsx
<TableCell className="whitespace-nowrap text-muted-foreground">
  <div className="flex items-center gap-1.5 text-xs">
    <ProviderIcon protocol={log.apiProtocol} className="size-3.5" />
    {log.apiProtocol || "—"}
  </div>
  <div className="flex items-center gap-1.5 text-xs text-muted-foreground/70">
    <ProviderIcon protocol={log.upstreamProtocol} className="size-3.5" />
    {log.upstreamProtocol || "—"}
  </div>
</TableCell>
```

- [ ] **Step 3: 修改移动端展开详情中的 Protocol 字段**

定位到 `isMobile` 分支展开详情中的两个字段：

```tsx
<div>
  <span className="text-muted-foreground">Upstream</span>
  <p className="flex items-center gap-1.5">
    <ProviderIcon protocol={log.upstreamProtocol} className="size-3.5" />
    {log.upstreamProtocol || "—"}
  </p>
</div>
<div>
  <span className="text-muted-foreground">API Protocol</span>
  <p className="flex items-center gap-1.5">
    <ProviderIcon protocol={log.apiProtocol} className="size-3.5" />
    {log.apiProtocol || "—"}
  </p>
</div>
```

- [ ] **Step 4: 验证 Audit 页面无类型错误**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npx tsc --noEmit --pretty
```

Expected: 无新增报错。

- [ ] **Step 5: Commit Audit 改动**

```bash
git add web/src/app/(dashboard)/audit/page.tsx
git commit -m "feat(web): replace protocol labels with provider icons on audit page"
```

---

## Task 4: 前端构建与 lint 验证

- [ ] **Step 1: 运行前端 lint**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint
```

Expected: 无新增 error/warning。

- [ ] **Step 2: 运行前端类型检查**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npx tsc --noEmit --pretty
```

Expected: 无报错。

- [ ] **Step 3: 运行前端构建**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run build
```

Expected: 构建成功，产物输出到 `web/out/`。

- [ ] **Step 4: Commit 验证结果（如有配置等改动）**

如无其他改动则无需 commit。

---

## 验收清单

- [ ] Endpoints 桌面端表格：Supported APIs 列显示图标 + 短文字。
- [ ] Endpoints 移动端卡片：Supported APIs 显示图标 + 短文字。
- [ ] Endpoints Base URL 列：`O:` / `A:` 前缀已移除，显示 OpenAI / Anthropic 图标。
- [ ] Audit 桌面端表格：Protocol 列显示图标 + 完整 protocol 文字。
- [ ] Audit 移动端展开详情：Upstream / API Protocol 字段显示图标。
- [ ] 未知 protocol 不崩溃，原文字保留。
- [ ] `npm run lint` 通过。
- [ ] `npx tsc --noEmit` 通过。
- [ ] `npm run build` 通过。

---

## 风险与回退

- 品牌 logo SVG 文件较大（OpenAI path 较长），可能会轻微增加 bundle 体积；因只在两个页面使用，影响可控。
- 若后续需要支持新的 provider，只需扩展 `ProviderIcon` 组件即可。
- 回退方式：还原三个文件的改动即可恢复文字显示。
