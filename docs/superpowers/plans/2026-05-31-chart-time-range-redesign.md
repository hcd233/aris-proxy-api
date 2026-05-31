# Chart Time Range Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fixed time window + granularity toggle buttons with a flexible time range picker (shared across charts and audit pages), with granularity auto-derived from the selected range.

**Architecture:** Extract `TimeRangeKey` / `computeRange()` / `deriveGranularity()` into `web/src/lib/time-range.ts`; create a reusable `TimeRangePicker` UI component; use it in both chart components and the audit page.

**Tech Stack:** Next.js App Router, TypeScript, Tailwind v4, shadcn/ui (DropdownMenu), lucide-react

---

### Task 1: Create shared time-range utility

**Files:**
- Create: `web/src/lib/time-range.ts`

- [ ] **Step 1: Write the file**

```ts
import type { Granularity } from "@/lib/types";

export type TimeRangeKey = "1h" | "24h" | "7d" | "30d" | "custom";

export const TIME_RANGE_LABELS: Record<TimeRangeKey, string> = {
  "1h": "Last 1 hour",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  custom: "Custom",
};

export const TIME_RANGE_PRESETS: TimeRangeKey[] = ["1h", "24h", "7d", "30d"];

export function deriveGranularity(rangeMs: number): Granularity {
  const oneHour = 60 * 60 * 1000;
  const oneDay = 24 * oneHour;
  const sevenDays = 7 * oneDay;
  const thirtyDays = 30 * oneDay;
  if (rangeMs <= oneHour) return "minute";
  if (rangeMs <= sevenDays) return "hour";
  if (rangeMs <= thirtyDays) return "day";
  return "week";
}

export function computeRange(
  key: TimeRangeKey,
  customStart?: string,
  customEnd?: string,
): { startTime: string; endTime: string; granularity: Granularity } {
  const now = new Date();
  let start: Date;
  if (key === "custom") {
    start = customStart ? new Date(customStart) : new Date(now.getTime() - 24 * 60 * 60 * 1000);
    const end = customEnd ? new Date(customEnd) : now;
    return {
      startTime: start.toISOString(),
      endTime: end.toISOString(),
      granularity: deriveGranularity(end.getTime() - start.getTime()),
    };
  }
  start = new Date(now);
  if (key === "1h") start.setHours(start.getHours() - 1);
  else if (key === "24h") start.setHours(start.getHours() - 24);
  else if (key === "7d") start.setDate(start.getDate() - 7);
  else if (key === "30d") start.setDate(start.getDate() - 30);
  const rangeMs = now.getTime() - start.getTime();
  return {
    startTime: start.toISOString(),
    endTime: now.toISOString(),
    granularity: deriveGranularity(rangeMs),
  };
}
```

- [ ] **Step 2: Verify file is clean**

Run: `cd web && npm run lint`
Expected: no errors related to this file

---

### Task 2: Create shared TimeRangePicker UI component

**Files:**
- Create: `web/src/components/ui/time-range-picker.tsx`

- [ ] **Step 1: Write the component**

```tsx
"use client";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Check, Clock } from "lucide-react";
import type { TimeRangeKey } from "@/lib/time-range";
import { TIME_RANGE_LABELS, TIME_RANGE_PRESETS } from "@/lib/time-range";

export interface TimeRangePickerProps {
  value: TimeRangeKey;
  customStart: string;
  customEnd: string;
  onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
}

export function TimeRangePicker({ value, customStart, customEnd, onChange }: TimeRangePickerProps) {
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="outline" size="sm" className="gap-1.5" />}
        >
          <Clock className="size-3.5" />
          {TIME_RANGE_LABELS[value]}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {([...TIME_RANGE_PRESETS, "custom"] as TimeRangeKey[]).map((k) => (
            <DropdownMenuItem
              key={k}
              onClick={() => {
                onChange(k, customStart, customEnd);
              }}
            >
              {k === value && <Check className="size-4" />}
              <span className={k === value ? "ml-0" : "ml-6"}>
                {TIME_RANGE_LABELS[k]}
              </span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {value === "custom" && (
        <div className="flex items-center gap-2">
          <input
            type="datetime-local"
            value={customStart}
            onChange={(e) => onChange("custom", e.target.value, customEnd)}
            className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
          />
          <span className="text-xs text-muted-foreground">–</span>
          <input
            type="datetime-local"
            value={customEnd}
            onChange={(e) => onChange("custom", customStart, e.target.value)}
            className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
          />
        </div>
      )}
    </>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd web && npm run lint`
Expected: no errors

---

### Task 3: Refactor audit page to use shared TimeRangePicker

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx`

- [ ] **Step 1: Remove inline TimeRange types, labels, computeRange — replace with imports**

Replace imports (lines 30-36) – add `TimeRangePicker` and `timeRange` lib:

```tsx
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
```

Remove lines 38-64 (local `TimeRangeKey`, `TIME_RANGE_LABELS`, `computeRange`).

- [ ] **Step 2: Update `fetchLogs` to use new `computeRange` signature**

Update the `computeRange` call (line 101) — audit only needs `startTime`/`endTime`, destructure and pass:

```tsx
const { startTime, endTime } = computeRange(range, cs, ce);
await api.listAuditLogs({
  page,
  pageSize,
  query: query || undefined,
  startTime,
  endTime,
});
```

`computeRange` now returns `granularity` too, but audit doesn't use it — that's fine.

- [ ] **Step 3: Replace inline DropdownMenu + custom inputs with `<TimeRangePicker>`**

Replace lines 168-216 (the `<DropdownMenu>` block through the custom inputs) with:

```tsx
<TimeRangePicker
  value={timeRange}
  customStart={customStart}
  customEnd={customEnd}
  onChange={(key, cs, ce) => {
    setTimeRange(key);
    setCustomStart(cs);
    setCustomEnd(ce);
    if (key !== "custom") {
      fetchLogs(1, pageInfo.pageSize, searchQuery, key, cs, ce);
    }
  }}
/>
```

- [ ] **Step 4: Verify build**

Run: `cd web && npm run lint && npm run build`
Expected: no errors

---

### Task 4: Refactor ModelTrendChart to use TimeRangePicker + auto granularity

**Files:**
- Modify: `web/src/components/charts/model-trend-chart.tsx`

- [ ] **Step 1: Replace imports**

Remove:
```tsx
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
```
Remove the `granularityOptions` array (lines 20-24).
Remove the `toISODate` function (lines 26-28), since `computeRange` already returns ISO strings.

Add:
```tsx
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
```

- [ ] **Step 2: Replace state**

Replace:
```tsx
const [granularity, setGranularity] = useState<Granularity>("day");
```
With:
```tsx
const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
const [customStart, setCustomStart] = useState("");
const [customEnd, setCustomEnd] = useState("");
```

- [ ] **Step 3: Rewrite fetchData**

Replace the `fetchData` callback (lines 37-55) with:

```tsx
const fetchData = useCallback(async () => {
  setLoading(true);
  setError(false);
  try {
    const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
    const rsp = await api.fetchModelTrend({
      startTime,
      endTime,
      granularity,
    });
    setData(rsp.data ?? []);
  } catch {
    setError(true);
  } finally {
    setLoading(false);
  }
}, [timeRange, customStart, customEnd]);
```

- [ ] **Step 4: Replace the useEffect dependency**

The useEffect line 60 stays the same - it depends on `fetchData`. No change needed.

- [ ] **Step 5: Replace ToggleGroup in CardHeader**

Replace lines 90-100 (the `<ToggleGroup>` block) with:

```tsx
<TimeRangePicker
  value={timeRange}
  customStart={customStart}
  customEnd={customEnd}
  onChange={(key, cs, ce) => {
    setTimeRange(key);
    setCustomStart(cs);
    setCustomEnd(ce);
  }}
/>
```

- [ ] **Step 6: Update the empty state text**

Line 114: change `"No data for this period"` — this still reads fine, no change needed.

- [ ] **Step 7: Verify build**

Run: `cd web && npm run lint && npm run build`
Expected: no errors

---

### Task 5: Refactor RequestRateChart to use TimeRangePicker + auto granularity

**Files:**
- Modify: `web/src/components/charts/request-rate-chart.tsx`

- [ ] **Step 1: Replace imports**

Remove:
```tsx
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
```
Remove the `granularityOptions` array (lines 20-24).
Remove the `toISODate` function (lines 26-28).

Add:
```tsx
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
```

- [ ] **Step 2: Replace state**

Replace:
```tsx
const [granularity, setGranularity] = useState<Granularity>("hour");
```
With:
```tsx
const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
const [customStart, setCustomStart] = useState("");
const [customEnd, setCustomEnd] = useState("");
```

- [ ] **Step 3: Rewrite fetchData**

Replace the `fetchData` callback (lines 37-55) with:

```tsx
const fetchData = useCallback(async () => {
  setLoading(true);
  setError(false);
  try {
    const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
    const rsp = await api.fetchRequestRate({
      startTime,
      endTime,
      granularity,
    });
    setData(rsp.data ?? []);
  } catch {
    setError(true);
  } finally {
    setLoading(false);
  }
}, [timeRange, customStart, customEnd]);
```

- [ ] **Step 4: Replace ToggleGroup in CardHeader**

Replace lines 90-100 (the `<ToggleGroup>` block) with:

```tsx
<TimeRangePicker
  value={timeRange}
  customStart={customStart}
  customEnd={customEnd}
  onChange={(key, cs, ce) => {
    setTimeRange(key);
    setCustomStart(cs);
    setCustomEnd(ce);
  }}
/>
```

- [ ] **Step 5: Verify build**

Run: `cd web && npm run lint && npm run build`
Expected: no errors

---

### Task 6: Remove unused ToggleGroup import (optional)

Check if `ToggleGroup` / `ToggleGroupItem` are used elsewhere in the project. If not, they can be removed.

- [ ] **Step 1: Check usage**

Run: `cd web && grep -r "toggle-group" src/ --include="*.tsx" --include="*.ts"`
Expected: only `web/src/components/ui/toggle-group.tsx` itself

If no other files import from `toggle-group`, no further action needed — it remains in the ui directory but becomes dead code. Don't delete it proactively unless asked.

---

### Task 7: Final verification

- [ ] **Step 1: Run lint**

Run: `cd web && npm run lint`
Expected: clean

- [ ] **Step 2: Run build**

Run: `cd web && npm run build`
Expected: successful export build

- [ ] **Step 3: Run full Go tests** (if chart changes affect any backend contracts — unlikely but verify)

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test -count=1 ./...`
Expected: all passing (this is a pure frontend change, but good to check)
