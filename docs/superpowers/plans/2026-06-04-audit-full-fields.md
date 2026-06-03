# Audit List Full Fields Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display all 17 AuditLogItem fields on desktop table (10 columns) and mobile cards (expandable with accordion + animation).

**Architecture:** Single-file change in `web/src/app/(dashboard)/audit/page.tsx`. Desktop: extend table columns, add UA column, merge cache info into Token cell, add two-line sub-row format for Provider and Latency. Mobile: add expandedId state for accordion, conditionally render detail grid, use CSS grid-template-rows transition for expand/collapse animation, respect prefers-reduced-motion.

**Tech Stack:** Next.js 16 (App Router), React 19, TypeScript, Tailwind v4, shadcn/ui

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `web/src/app/(dashboard)/audit/page.tsx` | Modify | All UI changes: desktop table, mobile cards, format helpers |
| `web/src/lib/types.ts` | No change | Already contains all 17 fields |
| `web/src/lib/api-client.ts` | No change | Already returns full AuditLogItem |

---

### Task 1: Extend formatTokens helper with cache support

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx:65-68`

- [ ] **Step 1: Update formatTokens and add formatCacheTokens**

Replace the existing `formatTokens` function (lines 65-68) with:

```typescript
function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)} / ${fmt(output)}`;
}

function formatCacheTokens(write: number, read: number): string | null {
  if (write === 0 && read === 0) return null;
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `c: ${fmt(write)} / ${fmt(read)}`;
}
```

- [ ] **Step 2: Verify no type errors**

Run: `cd web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/app/\(dashboard\)/audit/page.tsx
git commit -m "feat: add formatCacheTokens helper for audit list"
```

---

### Task 2: Desktop table — add UA column and merge cache into Token column

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx:284-327` (TableHeader + TableBody)

- [ ] **Step 1: Replace desktop table header (lines 275-284)**

Replace:
```tsx
<TableHead>Time</TableHead>
<TableHead>Model</TableHead>
<TableHead>User</TableHead>
<TableHead>API Key</TableHead>
<TableHead>Status</TableHead>
<TableHead>Tokens</TableHead>
<TableHead>Latency</TableHead>
<TableHead>TraceID</TableHead>
```

With:
```tsx
<TableHead>Time</TableHead>
<TableHead>Model</TableHead>
<TableHead>User</TableHead>
<TableHead>Provider</TableHead>
<TableHead>Status</TableHead>
<TableHead>Tokens</TableHead>
<TableHead>Latency</TableHead>
<TableHead>UA</TableHead>
<TableHead>TraceID</TableHead>
```

- [ ] **Step 2: Replace desktop table body (lines 287-329)**

Replace the entire `<TableBody>` content with:

```tsx
<TableBody>
  {logs.map((log) => {
    const ok = log.upstreamStatusCode === 200;
    const hasError = !!log.errorMessage;
    const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);
    return (
      <TableRow
        key={log.id}
        className={ok ? "" : "bg-destructive/5"}
      >
        <TableCell className="whitespace-nowrap text-muted-foreground">
          <div>{new Date(log.createdAt).toLocaleTimeString()}</div>
          <div className="text-xs text-muted-foreground/70">
            {new Date(log.createdAt).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
          </div>
        </TableCell>
        <TableCell className="max-w-[180px] truncate">{log.model || "—"}</TableCell>
        <TableCell>
          <div className="text-sm">{log.userName || "—"}</div>
          <div className="text-xs text-muted-foreground">
            {log.apiKeyName || ""}{log.userEmail ? ` · ${log.userEmail}` : ""}
          </div>
        </TableCell>
        <TableCell>
          <div className="text-sm">{log.apiProvider || "—"}</div>
          <div className="text-xs text-muted-foreground">
            upstream: {log.upstreamProvider || "—"}
          </div>
        </TableCell>
        <TableCell>
          <Badge
            variant={ok ? "secondary" : "destructive"}
            className="text-xs"
            title={hasError ? log.errorMessage : undefined}
          >
            {log.upstreamStatusCode}
          </Badge>
        </TableCell>
        <TableCell className="whitespace-nowrap">
          <div>{formatTokens(log.inputTokens, log.outputTokens)}</div>
          {cacheInfo && (
            <div className="text-xs text-muted-foreground">{cacheInfo}</div>
          )}
        </TableCell>
        <TableCell className="whitespace-nowrap text-muted-foreground">
          <div>{log.firstTokenLatencyMs}ms</div>
          {log.streamDurationMs > 0 && (
            <div className="text-xs text-muted-foreground/70">{log.streamDurationMs / 1000}s</div>
          )}
        </TableCell>
        <TableCell
          className="max-w-[160px] truncate text-xs"
          title={log.userAgent || undefined}
        >
          {log.userAgent || "—"}
        </TableCell>
        <TableCell
          className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
          onClick={() => handleCopyTrace(log.traceId)}
          title="Click to copy full traceID"
        >
          {log.traceId.slice(-6) || "—"}
        </TableCell>
      </TableRow>
    );
  })}
</TableBody>
```

- [ ] **Step 2: Verify type check**

Run: `cd web && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/app/\(dashboard\)/audit/page.tsx
git commit -m "feat: add UA column and cache info to desktop audit table"
```

---

### Task 3: Mobile cards — expandable detail with accordion + CSS animation

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx:70,236-270` (add expandedId state + replace mobile card JSX)

- [ ] **Step 1: Add expandedId state**

After line 79 (`const [pageInputValue, setPageInputValue] = useState("1");`), add:

```typescript
const [expandedId, setExpandedId] = useState<number | null>(null);
```

- [ ] **Step 2: Replace mobile card JSX (lines 236-270)**

Replace the entire mobile `logs.map(...)` block with:

```tsx
<div className="space-y-3">
  {logs.map((log) => {
    const ok = log.upstreamStatusCode === 200;
    const hasError = !!log.errorMessage;
    const isExpanded = expandedId === log.id;
    const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);

    return (
      <div
        key={log.id}
        className={`rounded-lg border border-border bg-card ${ok ? "" : "bg-destructive/5"}`}
      >
        {/* Collapsed header — always visible */}
        <div
          className="cursor-pointer p-4"
          onClick={() => setExpandedId(isExpanded ? null : log.id)}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{log.model || "—"}</p>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">
                {log.userName || "—"} · {log.apiKeyName || "—"} · {log.apiProvider || "—"}
              </p>
            </div>
            <Badge
              variant={ok ? "secondary" : "destructive"}
              className="shrink-0 text-xs"
              title={hasError ? log.errorMessage : undefined}
            >
              {log.upstreamStatusCode}
            </Badge>
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
            <span>{log.firstTokenLatencyMs}ms</span>
            {cacheInfo && <span>{cacheInfo}</span>}
            <span
              className="cursor-pointer font-mono underline-offset-2 hover:underline"
              onClick={(e) => {
                e.stopPropagation();
                handleCopyTrace(log.traceId);
              }}
              title="Click to copy full traceID"
            >
              {log.traceId.slice(-6) || "—"}
            </span>
          </div>
          <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground/70">
            <span>{new Date(log.createdAt).toLocaleString()}</span>
            <span
              className="inline-flex items-center gap-1 text-muted-foreground transition-transform duration-200"
              style={{
                transform: isExpanded ? "rotate(180deg)" : "rotate(0deg)",
              }}
            >
              ▾
            </span>
          </div>
        </div>

        {/* Expanded detail — animated height */}
        <div
          className="grid overflow-hidden transition-all duration-250 ease-out motion-reduce:transition-none"
          style={{
            gridTemplateRows: isExpanded ? "1fr" : "0fr",
          }}
        >
          <div className="min-h-0">
            <div className="border-t border-border px-4 pb-4 pt-3">
              {/* Error block */}
              {hasError && (
                <div className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-xs">
                  <span className="font-medium text-destructive">Error: </span>
                  <span className="text-destructive">{log.errorMessage}</span>
                </div>
              )}

              {/* Detail grid */}
              <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                <div>
                  <span className="text-muted-foreground">Input Tokens</span>
                  <p>{log.inputTokens.toLocaleString()}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Output Tokens</span>
                  <p>{log.outputTokens.toLocaleString()}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Cache Write</span>
                  <p>{log.cacheCreationInputTokens.toLocaleString()}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Cache Hit</span>
                  <p>{log.cacheReadInputTokens.toLocaleString()}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">First Token</span>
                  <p>{log.firstTokenLatencyMs}ms</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Stream Duration</span>
                  <p>{log.streamDurationMs > 0 ? `${(log.streamDurationMs / 1000).toFixed(1)}s` : "—"}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Upstream</span>
                  <p>{log.upstreamProvider || "—"}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">User</span>
                  <p>{log.userName || "—"}</p>
                </div>
              </div>

              {/* UA */}
              <div className="mt-3 border-t border-border pt-2 text-xs">
                <span className="text-muted-foreground">UA: </span>
                <span className="break-all">{log.userAgent || "—"}</span>
              </div>

              {/* Footer */}
              <div className="mt-2 flex items-center justify-between border-t border-border pt-2 text-xs">
                <span className="text-muted-foreground">
                  {new Date(log.createdAt).toLocaleString()}
                </span>
                <span
                  className="cursor-pointer font-mono text-muted-foreground underline-offset-2 hover:underline"
                  onClick={() => handleCopyTrace(log.traceId)}
                >
                  Copy TraceID
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  })}
</div>
```

- [ ] **Step 2: Verify type check**

Run: `cd web && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/app/\(dashboard\)/audit/page.tsx
git commit -m "feat: expandable mobile audit cards with accordion animation"
```

---

### Task 4: Lint, typecheck, and build verification

**Files:**
- None

- [ ] **Step 1: Run lint**

```bash
cd web && npm run lint
```
Expected: Pass.

- [ ] **Step 2: Run build**

```bash
cd web && npm run build
```
Expected: Build succeeds, static export generated.

- [ ] **Step 3: Commit (if any lint fixes needed)**

If no fixes needed, skip.

---
