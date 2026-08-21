# Faceted 筛选栏实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 10 个 dashboard 列表页的筛选 UI 统一为 Faceted 组合式筛选栏（token + 自由文本混排输入框），已选条件全程可见，fetch 参数收敛为对象传参。

**Architecture:** 新增 `web/src/components/filter-bar/` 四模块（纯函数 DSL 层 + 类型 + 状态 hook + 组件），页面以声明式 `FacetDef` 数组接入；后端零改动（`internal/common/filter/parser.go` 的 DSL 已完备）。设计 spec：`docs/superpowers/specs/2026-08-21-faceted-filter-bar-design.md`。

**Tech Stack:** Next.js 16.2.6 + React 19 + Tailwind v4 + base-ui + vitest（新引入，仅测纯函数）。

## Global Constraints

- 仅改 `web/` 目录；Go 后端、`internal/` 任何文件零改动。
- i18n 必须 zh/en/ja 三语同步（`web/src/locales/{zh,en,ja}.json`）；`t()` 引用永久稳定（见 `lib/i18n.tsx` 注释），可直接进 hook 依赖数组。
- **禁止原生 `title` 属性**（自定义 eslint 规则 `no-native-title` 会报错）；按钮提示用 `aria-label`。
- effect 内 setState 触发 `react-hooks/set-state-in-effect` 报错时，按代码库惯例加带理由的 `/* eslint-disable ... -- 理由 */` 注释（参考 sessions/page.tsx 现有用法）。
- localStorage 持久化一律经 `@/hooks/use-persistent-state`。
- 提交信息：英文 conventional commits（`feat(web): ...` / `test(web): ...` / `refactor(web): ...` / `chore(web): ...`）。
- 仓库 pre-commit hook 会跑 gofmt 与 Go conv lint，对纯 web 提交无影响。
- 每个 Task 完成后验证：`cd web && npm run lint && npm run test`（涉及页面改动时加 `npm run build`）。

## 与 spec 的两处偏离（有意简化）

1. `FacetDef` 不设 `kind: "range"` 字段：range 选项（如 `0-10`）对前端只是不透明字符串，序列化原样传递，后端按 `IsRange` 解析，前端无需区分。
2. spec 中 `useFilterBar` 的 `freeText: { placeholder }` 选项简化为 `freeTextPlaceholder?: string` 扁平字段。

---

### Task 1: vitest 接入 + filter-dsl.ts（TDD）

**Files:**
- Create: `web/src/components/filter-bar/filter-dsl.ts`
- Test: `web/src/components/filter-bar/filter-dsl.test.ts`
- Modify: `web/package.json`（devDependencies + scripts）

**Interfaces:**
- Produces（后续所有 Task 依赖）:
  - `interface FilterToken { key: string | null; value: string }`（key=null 为自由文本 token）
  - `serializeTokens(tokens: FilterToken[]): string | undefined`
  - `parseFilterString(expr: string, knownKeys: ReadonlySet<string>): FilterToken[]`
  - `migrateLegacyTokens(legacy: Record<string, unknown>): FilterToken[]`

- [ ] **Step 1: 建分支 + 装 vitest**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
git checkout -b feature/faceted-filter-bar-2026-08-21
cd web && npm i -D vitest
```

在 `web/package.json` 的 scripts 中加一行：

```json
"test": "vitest run",
```

- [ ] **Step 2: 写失败的测试**

创建 `web/src/components/filter-bar/filter-dsl.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import {
  migrateLegacyTokens,
  parseFilterString,
  serializeTokens,
  type FilterToken,
} from "./filter-dsl";

describe("serializeTokens", () => {
  it("空数组返回 undefined", () => {
    expect(serializeTokens([])).toBeUndefined();
  });

  it("自由文本 token 不进入 DSL", () => {
    expect(serializeTokens([{ key: null, value: "退款" }])).toBeUndefined();
  });

  it("单 token 序列化", () => {
    expect(serializeTokens([{ key: "score", value: "5" }])).toBe("score:5");
  });

  it("同 key 多 token 合并为 | 分隔", () => {
    const tokens: FilterToken[] = [
      { key: "score", value: "5" },
      { key: "score", value: "4" },
    ];
    expect(serializeTokens(tokens)).toBe("score:5|4");
  });

  it("多 key 以空格连接且保持首次出现顺序", () => {
    const tokens: FilterToken[] = [
      { key: "model", value: "gpt-5.2" },
      { key: "score", value: "5" },
      { key: "model", value: "kimi-k3" },
    ];
    expect(serializeTokens(tokens)).toBe("model:gpt-5.2|kimi-k3 score:5");
  });

  it("含空格的值整体加引号", () => {
    expect(serializeTokens([{ key: "model", value: "hello world" }])).toBe('model:"hello world"');
  });

  it("含 | 的值整体加引号（后端按单值字面量处理）", () => {
    expect(serializeTokens([{ key: "model", value: "a|b" }])).toBe('model:"a|b"');
  });

  it("range 桶值原样传递", () => {
    expect(serializeTokens([{ key: "messageCount", value: "0-10" }])).toBe("messageCount:0-10");
  });

  it("同 key 多值仅部分需转义时逐值处理", () => {
    const tokens: FilterToken[] = [
      { key: "model", value: "gpt-5.2" },
      { key: "model", value: "hello world" },
    ];
    expect(serializeTokens(tokens)).toBe('model:gpt-5.2|"hello world"');
  });
});

describe("parseFilterString", () => {
  const keys = new Set(["score", "model", "messageCount"]);

  it("空串与空白返回空数组", () => {
    expect(parseFilterString("", keys)).toEqual([]);
    expect(parseFilterString("   ", keys)).toEqual([]);
  });

  it("单值解析", () => {
    expect(parseFilterString("score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("多值 | 拆分为多 token", () => {
    expect(parseFilterString("score:5|4", keys)).toEqual([
      { key: "score", value: "5" },
      { key: "score", value: "4" },
    ]);
  });

  it("引号值去引号且不按 | 拆", () => {
    expect(parseFilterString('model:"a|b c"', keys)).toEqual([{ key: "model", value: "a|b c" }]);
  });

  it("未知 key 丢弃并保留其余", () => {
    expect(parseFilterString("unknown:1 score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("非法片段（无冒号/空 key）丢弃", () => {
    expect(parseFilterString("garbage :5 score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("UI 不生成的操作符片段（! > < = 开头值）丢弃", () => {
    expect(parseFilterString("score:!5 score:>3 score:5", keys)).toEqual([
      { key: "score", value: "5" },
    ]);
  });

  it("引号内空格不参与切分", () => {
    expect(parseFilterString('model:"hello world" score:5', keys)).toEqual([
      { key: "model", value: "hello world" },
      { key: "score", value: "5" },
    ]);
  });
});

describe("round-trip", () => {
  it("典型集合 parse(serialize) 还原", () => {
    const keys = new Set(["score", "model", "messageCount"]);
    const tokens: FilterToken[] = [
      { key: "score", value: "5" },
      { key: "score", value: "4" },
      { key: "model", value: "gpt-5.2" },
      { key: "messageCount", value: "0-10" },
    ];
    expect(parseFilterString(serializeTokens(tokens)!, keys)).toEqual(tokens);
  });

  it("含空格值 round-trip", () => {
    const keys = new Set(["model"]);
    const tokens: FilterToken[] = [{ key: "model", value: "hello world" }];
    expect(parseFilterString(serializeTokens(tokens)!, keys)).toEqual(tokens);
  });
});

describe("migrateLegacyTokens", () => {
  it("旧 string[] 与 string 混合迁移", () => {
    const tokens = migrateLegacyTokens({
      score: ["5", "4"],
      model: ["gpt-5.2"],
      keyword: "退款",
    });
    expect(tokens).toEqual([
      { key: "score", value: "5" },
      { key: "score", value: "4" },
      { key: "model", value: "gpt-5.2" },
      { key: null, value: "退款" },
    ]);
  });

  it("空旧值返回空数组", () => {
    expect(migrateLegacyTokens({ score: [], keyword: "" })).toEqual([]);
  });

  it("非预期类型安全忽略", () => {
    expect(migrateLegacyTokens({ score: 42, model: null })).toEqual([]);
  });
});
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/filter-bar/filter-dsl.test.ts`
Expected: FAIL（模块不存在，`Failed to resolve import "./filter-dsl"`）

- [ ] **Step 4: 实现 filter-dsl.ts**

创建 `web/src/components/filter-bar/filter-dsl.ts`：

```ts
/**
 * filter DSL 序列化/解析，与 internal/common/filter/parser.go 对齐：
 *   field:value field2:a|b field3:"含空格值"
 * 同 key 多 token 序列化合并为 key:v1|v2；值含空格/|/" 时整体加双引号
 * （后端 splitValues 对整体引号包裹的段按单值字面量处理，不再按 | 拆分）。
 */

export interface FilterToken {
  /** facet key；自由文本（关键词）token 为 null，不进入 DSL */
  key: string | null;
  value: string;
}

function escapeValue(value: string): string {
  if (/[\s|"]/.test(value)) {
    return `"${value.replace(/"/g, "")}"`;
  }
  return value;
}

export function serializeTokens(tokens: FilterToken[]): string | undefined {
  const groups = new Map<string, string[]>();
  for (const token of tokens) {
    if (token.key === null) continue;
    const values = groups.get(token.key) ?? [];
    values.push(token.value);
    groups.set(token.key, values);
  }
  if (groups.size === 0) return undefined;
  return Array.from(groups, ([key, values]) => `${key}:${values.map(escapeValue).join("|")}`).join(
    " ",
  );
}

/** 按空格切分表达式，保留引号内空格（移植 Go splitExpression） */
function splitExpression(expr: string): string[] {
  const parts: string[] = [];
  let current = "";
  let inQuote = false;
  for (const ch of expr) {
    if (ch === '"') {
      inQuote = !inQuote;
      current += ch;
    } else if (ch === " " && !inQuote) {
      if (current) {
        parts.push(current);
        current = "";
      }
    } else {
      current += ch;
    }
  }
  if (current) parts.push(current);
  return parts;
}

function splitValues(raw: string): string[] {
  const trimmed = raw.trim();
  if (trimmed.length >= 2 && trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return [trimmed.slice(1, -1)];
  }
  return trimmed
    .split("|")
    .map((p) => p.trim().replace(/^"|"$/g, ""))
    .filter((p) => p !== "");
}

/** UI 不生成的比较/取反操作符（值首字符命中即视为高级语法，恢复时丢弃） */
const OPERATOR_PREFIX = /^[!><=]/;

/**
 * 解析 filter DSL 为 token 列表（持久化恢复/容错用）。
 * 未知 key、非法片段、含比较操作符的片段直接丢弃，其余保留。
 */
export function parseFilterString(expr: string, knownKeys: ReadonlySet<string>): FilterToken[] {
  const tokens: FilterToken[] = [];
  for (const part of splitExpression(expr.trim())) {
    const idx = part.indexOf(":");
    if (idx <= 0) continue;
    const key = part.slice(0, idx);
    if (!knownKeys.has(key)) continue;
    const rawValue = part.slice(idx + 1);
    if (OPERATOR_PREFIX.test(rawValue)) continue;
    for (const value of splitValues(rawValue)) {
      tokens.push({ key, value });
    }
  }
  return tokens;
}

/**
 * 旧 localStorage 筛选数据迁移为 token 列表（纯函数）。
 * legacy 的 value 为 string[]（facet 值）或 string（关键词，key 用 "keyword"）。
 */
export function migrateLegacyTokens(legacy: Record<string, unknown>): FilterToken[] {
  const tokens: FilterToken[] = [];
  for (const [key, value] of Object.entries(legacy)) {
    if (key === "keyword") {
      if (typeof value === "string" && value !== "") tokens.push({ key: null, value });
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (typeof item === "string" && item !== "") tokens.push({ key, value: item });
      }
    }
  }
  return tokens;
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd web && npx vitest run src/components/filter-bar/filter-dsl.test.ts`
Expected: PASS（23 个用例全绿）

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/src/components/filter-bar/
git commit -m "test(web): introduce vitest and add filter DSL serialize/parse module"
```

---

### Task 2: types.ts + use-filter-bar.ts

**Files:**
- Create: `web/src/components/filter-bar/types.ts`
- Create: `web/src/components/filter-bar/use-filter-bar.ts`

**Interfaces:**
- Consumes: Task 1 的 `FilterToken` / `serializeTokens` / `migrateLegacyTokens`；`@/hooks/use-persistent-state` 的 `usePersistentState<T>(key, defaultValue)`。
- Produces（Task 3 与各页面依赖）:
  - `interface FacetDef { key; label; options; target?; paramName?; single?; formatValue? }`
  - `interface FilterBarQueryParams { filter?: string; freeText: string; params: Record<string, string> }`
  - `useFilterBar(opts: UseFilterBarOptions): UseFilterBarReturn`（签名见下）

- [ ] **Step 1: 创建 types.ts**

创建 `web/src/components/filter-bar/types.ts`：

```ts
import type { FilterToken } from "./filter-dsl";

/** 列表页筛选维度的声明式配置 */
export interface FacetDef {
  /** 后端 DSL 字段名（如 "score" / "model" / "messageCount"） */
  key: string;
  /** i18n 后的展示标签（如 "评分"），参与 facet 建议匹配 */
  label: string;
  /** 静态选项，或异步加载（展开值建议时调用，结果缓存至 optionsCacheKey 变化） */
  options: string[] | (() => Promise<string[]>);
  /** 序列化目标：filter DSL（默认）或独立 query 参数（如 users 页 permission） */
  target?: "filter" | "param";
  /** target="param" 时的 query 参数名，默认同 key */
  paramName?: string;
  /** true 时同 key 只保留一个 token（选新值即替换旧值），用于单选参数 */
  single?: boolean;
  /** token 与值建议的展示格式化（如 "5" → "★5"、"unscored" → 未评分） */
  formatValue?: (value: string) => string;
}

/** 筛选状态的结构化输出，页面合并进 list 请求 */
export interface FilterBarQueryParams {
  /** filter DSL 字符串；无 filter 类 token 时 undefined */
  filter?: string;
  /** 自由文本（关键词）；无关键词 token 时 "" */
  freeText: string;
  /** target="param" facet 的 query 参数集合 */
  params: Record<string, string>;
}

export type { FilterToken };
```

- [ ] **Step 2: 创建 use-filter-bar.ts**

创建 `web/src/components/filter-bar/use-filter-bar.ts`：

```ts
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { migrateLegacyTokens, serializeTokens, type FilterToken } from "./filter-dsl";
import type { FacetDef, FilterBarQueryParams } from "./types";

export interface UseFilterBarOptions {
  /** 持久化命名空间（如 "dashboard.sessions"），token 存 "<persistKey>.filters" */
  persistKey: string;
  facets: FacetDef[];
  /** 自由文本（关键词）输入占位；不传则 bar 不接受自由文本 */
  freeTextPlaceholder?: string;
  /** 旧 localStorage key 迁移：facetKey → 旧 key（值为 string[]） */
  legacyKeys?: Record<string, string>;
  /** 旧关键词 key（值为 string，如 "dashboard.sessions.keyword"） */
  legacyFreeTextKey?: string;
  /** 异步选项缓存失效指纹（如 `${startTime}:${endTime}`），变化即清空缓存 */
  optionsCacheKey?: string;
}

export interface UseFilterBarReturn {
  tokens: FilterToken[];
  addToken: (token: FilterToken) => void;
  removeToken: (index: number) => void;
  clearTokens: () => void;
  /** 展开某 facet 值建议时调用；静态选项直接返回，异步选项带缓存 */
  loadOptions: (facet: FacetDef) => Promise<string[]>;
  /** 正在异步加载选项的 facet key（无则 null） */
  loadingKey: string | null;
  /** 结构化查询参数；identity 仅在 tokens 变化时更新（供页面 effect 依赖） */
  queryParams: FilterBarQueryParams;
}

/** 读取旧 localStorage 数据并迁移为 token（仅在新 key 不存在时作为默认值生效） */
function readLegacyTokens(opts: UseFilterBarOptions): FilterToken[] {
  if (typeof window === "undefined") return [];
  try {
    const legacy: Record<string, unknown> = {};
    for (const [facetKey, storageKey] of Object.entries(opts.legacyKeys ?? {})) {
      const raw = localStorage.getItem(storageKey);
      if (raw !== null) legacy[facetKey] = JSON.parse(raw);
    }
    if (opts.legacyFreeTextKey) {
      const raw = localStorage.getItem(opts.legacyFreeTextKey);
      if (raw !== null) legacy.keyword = JSON.parse(raw);
    }
    return migrateLegacyTokens(legacy);
  } catch {
    return [];
  }
}

export function useFilterBar(opts: UseFilterBarOptions): UseFilterBarReturn {
  /* eslint-disable-next-line react-hooks/exhaustive-deps -- 仅挂载时读取一次旧数据做迁移 */
  const initialTokens = useMemo(() => readLegacyTokens(opts), []);
  const [tokens, setTokens] = usePersistentState<FilterToken[]>(
    `${opts.persistKey}.filters`,
    initialTokens,
  );

  // facets 每渲染都是新数组，经 ref 读取以避免 queryParams memo 频繁失效
  const facetsRef = useRef(opts.facets);
  facetsRef.current = opts.facets;

  // 挂载后清理旧 key（迁移产物已由 usePersistentState 的 effect 写入新 key）
  /* eslint-disable-next-line react-hooks/exhaustive-deps -- 仅挂载时执行一次清理 */
  useEffect(() => {
    const staleKeys = [...Object.values(opts.legacyKeys ?? {})];
    if (opts.legacyFreeTextKey) staleKeys.push(opts.legacyFreeTextKey);
    for (const key of staleKeys) {
      try {
        localStorage.removeItem(key);
      } catch {
        // 隐私模式等场景忽略
      }
    }
  }, []);

  // optionsCacheKey 变化时清空异步选项缓存
  const optionsCacheRef = useRef(new Map<string, string[]>());
  const cacheKeyRef = useRef(opts.optionsCacheKey);
  if (cacheKeyRef.current !== opts.optionsCacheKey) {
    cacheKeyRef.current = opts.optionsCacheKey;
    optionsCacheRef.current.clear();
  }

  const [loadingKey, setLoadingKey] = useState<string | null>(null);

  const loadOptions = useCallback(async (facet: FacetDef): Promise<string[]> => {
    if (Array.isArray(facet.options)) return facet.options;
    const cached = optionsCacheRef.current.get(facet.key);
    if (cached) return cached;
    setLoadingKey(facet.key);
    try {
      const items = (await facet.options()) ?? [];
      optionsCacheRef.current.set(facet.key, items);
      return items;
    } catch {
      return [];
    } finally {
      setLoadingKey(null);
    }
  }, []);

  const addToken = useCallback(
    (token: FilterToken) => {
      setTokens((prev) => {
        const facet = token.key
          ? facetsRef.current.find((f) => f.key === token.key)
          : undefined;
        // 自由文本与 single facet：同 key 替换而非追加
        const replace = token.key === null || facet?.single;
        const base = replace ? prev.filter((t) => t.key !== token.key) : prev;
        if (base.some((t) => t.key === token.key && t.value === token.value)) return base;
        return [...base, token];
      });
    },
    [setTokens],
  );

  const removeToken = useCallback(
    (index: number) => setTokens((prev) => prev.filter((_, i) => i !== index)),
    [setTokens],
  );

  const clearTokens = useCallback(() => setTokens([]), [setTokens]);

  const queryParams = useMemo<FilterBarQueryParams>(() => {
    const facets = facetsRef.current;
    const filter = serializeTokens(
      tokens.filter((t) => {
        if (t.key === null) return false;
        return facets.find((f) => f.key === t.key)?.target !== "param";
      }),
    );
    const params: Record<string, string> = {};
    for (const facet of facets) {
      if (facet.target !== "param") continue;
      const token = tokens.find((t) => t.key === facet.key);
      if (token) params[facet.paramName ?? facet.key] = token.value;
    }
    const freeText = tokens.find((t) => t.key === null)?.value ?? "";
    return { filter, freeText, params };
    /* eslint-disable-next-line react-hooks/exhaustive-deps -- facets 经 ref 读取，仅 tokens 驱动更新 */
  }, [tokens]);

  return { tokens, addToken, removeToken, clearTokens, loadOptions, loadingKey, queryParams };
}
```

- [ ] **Step 3: 验证编译与 lint**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无 error

- [ ] **Step 4: Commit**

```bash
git add web/src/components/filter-bar/types.ts web/src/components/filter-bar/use-filter-bar.ts
git commit -m "feat(web): add useFilterBar hook with legacy filter migration"
```

---

### Task 3: filter-bar.tsx 组件

**Files:**
- Create: `web/src/components/filter-bar/filter-bar.tsx`

**Interfaces:**
- Consumes: Task 2 的 `FacetDef` / `FilterToken` / `UseFilterBarReturn`；`@/lib/i18n` 的 `useT`；`@/lib/utils` 的 `cn`；lucide-react 的 `Search` / `X` / `Loader2`。
- Produces: `<FilterBar {...props} />`，props 与 `UseFilterBarReturn` + `facets` + `placeholder` 对齐（页面直接 `<FilterBar {...filterBar} facets={facets} placeholder={...} />` 展开传入）。

- [ ] **Step 1: 创建 filter-bar.tsx**

创建 `web/src/components/filter-bar/filter-bar.tsx`：

```tsx
"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import type { FilterToken } from "./filter-dsl";
import type { FacetDef } from "./types";
import type { UseFilterBarReturn } from "./use-filter-bar";

export interface FilterBarProps
  extends Pick<
    UseFilterBarReturn,
    "tokens" | "addToken" | "removeToken" | "clearTokens" | "loadOptions" | "loadingKey"
  > {
  facets: FacetDef[];
  placeholder?: string;
  className?: string;
}

type SuggestionRow =
  | { kind: "facet"; facet: FacetDef }
  | { kind: "value"; facet: FacetDef; value: string }
  | { kind: "keyword"; text: string };

const COLON_RE = /[:：]/;

/** input 是否为「facet 前缀 + 冒号」的值编辑态；前缀按 key 或 label 匹配（首个命中） */
function matchDraftFacet(input: string, facets: FacetDef[]) {
  const match = input.match(COLON_RE);
  if (!match || match.index === undefined || match.index === 0) return null;
  const prefix = input.slice(0, match.index);
  const facet = facets.find((f) => f.key.startsWith(prefix) || f.label.startsWith(prefix));
  if (!facet) return null;
  return { facet, valueQuery: input.slice(match.index + 1) };
}

export function FilterBar({
  facets,
  tokens,
  addToken,
  removeToken,
  clearTokens,
  loadOptions,
  loadingKey,
  placeholder,
  className,
}: FilterBarProps) {
  const t = useT();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const [valueOptions, setValueOptions] = useState<string[]>([]);

  const draft = matchDraftFacet(input, facets);
  const draftKey = draft?.facet.key ?? null;

  // 值编辑态变化时解析该 facet 的选项（静态直接给，异步经 loadOptions 缓存）
  /* eslint-disable react-hooks/set-state-in-effect -- 值建议选项随 draft facet 切换而解析，与 sessions 页 options 加载同模式 */
  useEffect(() => {
    if (!draft) {
      setValueOptions([]);
      return;
    }
    let cancelled = false;
    void loadOptions(draft.facet).then((items) => {
      if (!cancelled) setValueOptions(items);
    });
    return () => {
      cancelled = true;
    };
    /* eslint-disable-next-line react-hooks/exhaustive-deps -- 仅 draft facet 切换驱动；loadOptions 引用稳定 */
  }, [draftKey]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // 点击组件外收起建议
  useEffect(() => {
    const onDocMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDocMouseDown);
    return () => document.removeEventListener("mousedown", onDocMouseDown);
  }, []);

  const { heading, rows } = useMemo((): { heading: string; rows: SuggestionRow[] } => {
    if (draft) {
      const q = draft.valueQuery.toLowerCase();
      const candidates = valueOptions
        .filter((v) => !q || v.toLowerCase().includes(q))
        .filter((v) => !tokens.some((tk) => tk.key === draft.facet.key && tk.value === v));
      return {
        heading: t("filter_bar.suggest_values").replace("{facet}", draft.facet.label),
        rows: candidates.map((v) => ({ kind: "value", facet: draft.facet, value: v })),
      };
    }
    const q = input.trim().toLowerCase();
    const matched = facets.filter(
      (f) => !q || f.key.toLowerCase().includes(q) || f.label.includes(input.trim()),
    );
    if (q && matched.length === 0) {
      return { heading: t("filter_bar.keyword"), rows: [{ kind: "keyword", text: input.trim() }] };
    }
    return {
      heading: t("filter_bar.suggest_facets"),
      rows: matched.map((f) => ({ kind: "facet", facet: f })),
    };
  }, [draft, valueOptions, tokens, facets, input, t]);

  const hl = Math.min(highlight, Math.max(rows.length - 1, 0));

  const pick = (row: SuggestionRow) => {
    if (row.kind === "facet") {
      setInput(`${row.facet.key}:`);
      setHighlight(0);
      inputRef.current?.focus();
      return;
    }
    if (row.kind === "value") {
      addToken({ key: row.facet.key, value: row.value });
    } else {
      addToken({ key: null, value: row.text });
    }
    setInput("");
    setHighlight(0);
    inputRef.current?.focus();
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (rows.length === 0) return;
      setOpen(true);
      setHighlight((hl + (e.key === "ArrowDown" ? 1 : -1) + rows.length) % rows.length);
      return;
    }
    if (e.key === "Enter" || e.key === "Tab") {
      if (open && rows.length > 0) {
        e.preventDefault();
        pick(rows[hl]);
      } else if (input.trim()) {
        e.preventDefault();
        pick({ kind: "keyword", text: input.trim() });
      }
      return;
    }
    if (e.key === "Backspace" && !input && tokens.length > 0) {
      removeToken(tokens.length - 1);
      return;
    }
    if (e.key === "Escape") setOpen(false);
  };

  const tokenLabel = (tk: FilterToken) => {
    if (tk.key === null) return { k: t("filter_bar.keyword"), v: `“${tk.value}”`, free: true };
    const facet = facets.find((f) => f.key === tk.key);
    return {
      k: facet?.label ?? tk.key,
      v: facet?.formatValue ? facet.formatValue(tk.value) : tk.value,
      free: false,
    };
  };

  return (
    <div ref={rootRef} className={cn("relative min-w-0 flex-1", className)}>
      <div
        className="flex min-h-10 cursor-text flex-wrap items-center gap-1.5 rounded-lg border border-input bg-background px-2.5 py-1.5 transition-colors focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30"
        onClick={() => inputRef.current?.focus()}
      >
        <Search className="size-4 shrink-0 text-muted-foreground" />
        {tokens.map((tk, i) => {
          const label = tokenLabel(tk);
          return (
            <span
              key={`${tk.key ?? "kw"}:${tk.value}:${i}`}
              className={cn(
                "inline-flex h-7 items-center rounded-md border text-xs",
                label.free
                  ? "border-dashed border-input bg-transparent"
                  : "border-ring/40 bg-accent/60",
              )}
            >
              <span className="pl-2 pr-1 text-muted-foreground">{label.k}</span>
              <span className="max-w-40 truncate pr-1 font-medium">{label.v}</span>
              <button
                type="button"
                aria-label={t("filter_bar.remove_token")}
                className="mr-0.5 rounded p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
                onClick={(e) => {
                  e.stopPropagation();
                  removeToken(i);
                }}
              >
                <X className="size-3" />
              </button>
            </span>
          );
        })}
        <input
          ref={inputRef}
          value={input}
          placeholder={tokens.length === 0 ? placeholder : undefined}
          autoComplete="off"
          spellCheck={false}
          className="min-w-32 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
          onChange={(e) => {
            setInput(e.target.value);
            setHighlight(0);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        {tokens.length > 0 && (
          <button
            type="button"
            aria-label={t("filter_bar.clear_all")}
            className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
            onClick={(e) => {
              e.stopPropagation();
              clearTokens();
              setInput("");
            }}
          >
            <X className="size-3" />
            {t("filter_bar.clear_all")}
          </button>
        )}
      </div>

      {open && (rows.length > 0 || loadingKey !== null) && (
        <div className="absolute left-0 top-full z-50 mt-1.5 w-72 max-w-[calc(100vw-3rem)] rounded-lg border bg-popover p-1 text-sm shadow-md">
          <div className="px-2 pb-1 pt-1.5 text-[0.65rem] font-medium uppercase tracking-wider text-muted-foreground">
            {loadingKey !== null && draft ? t("filter_bar.loading_options") : heading}
          </div>
          {loadingKey !== null && draft ? (
            <div className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground">
              <Loader2 className="size-3.5 animate-spin" />
              {t("filter_bar.loading_options")}
            </div>
          ) : rows.length === 0 ? (
            <div className="px-2 py-2 text-xs text-muted-foreground">
              {t("filter_bar.no_options")}
            </div>
          ) : (
            rows.map((row, i) => (
              <button
                key={
                  row.kind === "facet"
                    ? `f:${row.facet.key}`
                    : row.kind === "value"
                      ? `v:${row.facet.key}:${row.value}`
                      : `kw:${row.text}`
                }
                type="button"
                role="option"
                aria-selected={i === hl}
                className={cn(
                  "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left hover:bg-accent",
                  i === hl && "bg-accent",
                )}
                onMouseDown={(e) => {
                  e.preventDefault();
                  pick(row);
                }}
                onMouseEnter={() => setHighlight(i)}
              >
                {row.kind === "facet" && (
                  <>
                    <span className="font-medium">{row.facet.label}</span>
                    <span className="text-xs text-muted-foreground">{row.facet.key}:</span>
                  </>
                )}
                {row.kind === "value" && (
                  <span className="truncate">
                    {row.facet.formatValue ? row.facet.formatValue(row.value) : row.value}
                  </span>
                )}
                {row.kind === "keyword" && (
                  <span className="truncate">
                    {t("filter_bar.keyword_hint").replace("{text}", row.text)}
                  </span>
                )}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 验证编译与 lint**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无 error

- [ ] **Step 3: Commit**

```bash
git add web/src/components/filter-bar/filter-bar.tsx
git commit -m "feat(web): add faceted FilterBar component with token editing and suggestions"
```

---

### Task 4: i18n keys（zh/en/ja）

**Files:**
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Produces: Task 3 引用的 `filter_bar.*` keys 与 Task 5 的 `sessions.filter_unscored`。

- [ ] **Step 1: 三个 locale 文件各追加以下 key**（插到相邻 filter 相关 key 附近，保持现有聚类习惯）

`web/src/locales/zh.json`：

```json
  "filter_bar.applied_count": "已应用 {count} 项条件",
  "filter_bar.clear_all": "清除全部",
  "filter_bar.keyword": "关键词",
  "filter_bar.keyword_hint": "关键词：“{text}”",
  "filter_bar.loading_options": "加载选项…",
  "filter_bar.no_options": "无匹配选项",
  "filter_bar.remove_token": "移除此条件",
  "filter_bar.suggest_facets": "筛选维度",
  "filter_bar.suggest_values": "选择「{facet}」的值",
  "sessions.filter_unscored": "未评分",
```

`web/src/locales/en.json`：

```json
  "filter_bar.applied_count": "{count} filters applied",
  "filter_bar.clear_all": "Clear all",
  "filter_bar.keyword": "Keyword",
  "filter_bar.keyword_hint": "Keyword: \"{text}\"",
  "filter_bar.loading_options": "Loading options…",
  "filter_bar.no_options": "No matching options",
  "filter_bar.remove_token": "Remove this filter",
  "filter_bar.suggest_facets": "Filter by",
  "filter_bar.suggest_values": "Pick a {facet} value",
  "sessions.filter_unscored": "Unscored",
```

`web/src/locales/ja.json`：

```json
  "filter_bar.applied_count": "{count} 件の条件を適用中",
  "filter_bar.clear_all": "すべてクリア",
  "filter_bar.keyword": "キーワード",
  "filter_bar.keyword_hint": "キーワード：「{text}」",
  "filter_bar.loading_options": "オプション読み込み中…",
  "filter_bar.no_options": "一致するオプションがありません",
  "filter_bar.remove_token": "この条件を削除",
  "filter_bar.suggest_facets": "フィルター項目",
  "filter_bar.suggest_values": "「{facet}」の値を選択",
  "sessions.filter_unscored": "未評価",
```

- [ ] **Step 2: 验证 JSON 合法与 lint**

Run: `cd web && python3 -c "import json; [json.load(open(f'src/locales/{l}.json')) for l in ('zh','en','ja')]" && npm run lint`
Expected: 无解析错误 / lint 通过

- [ ] **Step 3: Commit**

```bash
git add web/src/locales/
git commit -m "feat(web): add filter_bar i18n keys in zh/en/ja"
```

---

### Task 5: sessions 页迁移（试点模板）

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

**Interfaces:**
- Consumes: Task 2 `useFilterBar` / `FacetDef` / `FilterBarQueryParams`；Task 3 `FilterBar`；现有 `api.listSessionOptions({ field, startTime, endTime })`（field: `"score" | "model" | "messageCount"`）。
- Produces: 后续页面迁移的范式——**fetch 函数改对象参数 + `[queryParams]` effect 驱动查询 + 工具栏 `TimeRangePicker + FilterBar` 布局**。

- [ ] **Step 1: 替换筛选状态声明**

删除以下代码：

```ts
// 删除：dashboard.sessions.keyword / dashboard.sessions.searchInput 两个 usePersistentState
// 删除：filterScore / filterModel / filterMessageCount 三个 usePersistentState
// 删除：scoreOptions / modelOptions / messageCountOptions 三个 useState
// 删除：fetchScoreOptions / fetchModelOptions / fetchMessageCountOptions 与其 useEffect
// 删除：buildSessionFilter 函数
```

替换为：

```ts
const fetchOptionsFor = useCallback(
  (field: "score" | "model" | "messageCount") => async () => {
    const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
    const rsp = await api.listSessionOptions({ field, startTime, endTime });
    return rsp.items ?? [];
  },
  [timeRange, customStart, customEnd],
);

const facets = useMemo<FacetDef[]>(
  () => [
    {
      key: "score",
      label: t("sessions.filter_score"),
      options: fetchOptionsFor("score"),
      formatValue: (v) => (v === "unscored" ? t("sessions.filter_unscored") : `★${v}`),
    },
    { key: "model", label: t("sessions.filter_model"), options: fetchOptionsFor("model") },
    {
      key: "messageCount",
      label: t("sessions.filter_message_count"),
      options: fetchOptionsFor("messageCount"),
    },
  ],
  [t, fetchOptionsFor],
);

const filterBar = useFilterBar({
  persistKey: "dashboard.sessions",
  facets,
  freeTextPlaceholder: t("sessions.search_placeholder"),
  legacyKeys: {
    score: "dashboard.sessions.filterScore",
    model: "dashboard.sessions.filterModel",
    messageCount: "dashboard.sessions.filterMessageCount",
  },
  legacyFreeTextKey: "dashboard.sessions.keyword",
  optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
});
const { queryParams } = filterBar;
```

import 调整：删 `MultiSelectPill`、`SearchInput`；加 `useMemo`、`FilterBar`、`useFilterBar`、`FacetDef`（type import）。

- [ ] **Step 2: fetchSessions 改对象参数**

将现有 10 参数签名收敛为：

```ts
interface SessionsQuery {
  page: number;
  pageSize: number;
  range: TimeRangeKey;
  cs: string;
  ce: string;
  sortState: { field: string; dir: SortDir };
  qp: FilterBarQueryParams;
  silent?: boolean;
}

const fetchSessions = useCallback(
  async (q: SessionsQuery) => {
    if (!q.silent) setLoading(true);
    try {
      const { startTime, endTime } = computeRange(q.range, q.cs, q.ce);
      const rsp = await api.listSessions({
        page: q.page,
        pageSize: q.pageSize,
        sort: q.sortState.dir,
        sortField: q.sortState.field,
        startTime,
        endTime,
        keyword: q.qp.freeText || undefined,
        filter: q.qp.filter,
      });
      setSessions(rsp.sessions ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  },
  [setPersistedPage, setPersistedPageSize],
);
```

当前查询参数快照 helper（替代原 `refresh` 与散落的 9 参数透传）：

```ts
const currentQuery = (): Omit<SessionsQuery, "page" | "pageSize"> => ({
  range: timeRange,
  cs: customStart,
  ce: customEnd,
  sortState: sort,
  qp: queryParams,
});
```

调用点映射：
- 挂载 effect 与 `refresh(page, pageSize)` → `fetchSessions({ page, pageSize: pageSize ?? pageInfo.pageSize, ...currentQuery() })`
- `handleSort` → `fetchSessions({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery(), sortState: newSort })`
- TimeRangePicker `onChange` → `setTimeRange/setCustomStart/setCustomEnd` 后 `fetchSessions({ page: 1, pageSize: pageInfo.pageSize, range: key, cs, ce, sortState: sort, qp: queryParams })`
- 删除 `handleSearch` 与搜索框 `onClear` 回调（由 `[queryParams]` effect 接管）
- 删除/打分回调中的 `fetchSessions(...)` → `fetchSessions({ page: pageInfo.page, pageSize: pageInfo.pageSize, ...currentQuery(), silent: true })`

新增 token 驱动查询 effect（替代原挂载 effect）：

```ts
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- token 变化回到第 1 页查询；挂载时以持久化筛选发起首次查询 */
useEffect(() => {
  setSelected(new Set());
  fetchSessions({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery() });
}, [queryParams]);
/* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
```

- [ ] **Step 3: 工具栏 JSX 替换**

将 `{/* Filters — always visible */}` 整个 `div.mb-4...` 块（含 3 个 `MultiSelectPill`、清除按钮、`SearchInput`）替换为：

```tsx
{/* Filters — faceted bar */}
<div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center">
  <TimeRangePicker
    value={timeRange}
    customStart={customStart}
    customEnd={customEnd}
    onChange={(key, cs, ce) => {
      setTimeRange(key);
      setCustomStart(cs);
      setCustomEnd(ce);
      fetchSessions({
        page: 1,
        pageSize: pageInfo.pageSize,
        range: key,
        cs,
        ce,
        sortState: sort,
        qp: queryParams,
      });
    }}
  />
  <FilterBar {...filterBar} facets={facets} placeholder={t("sessions.search_placeholder")} />
  {selected.size > 0 && (
    <Button
      variant="destructive"
      size="sm"
      disabled={isDemo()}
      onClick={() => setBatchDeleteConfirmOpen(true)}
      className="gap-1.5 md:ml-auto"
    >
      {isDemo() ? <Lock className="size-3.5" /> : <Trash2 className="size-3.5" />}
      {t("common.delete")} {selected.size}
    </Button>
  )}
</div>
{filterBar.tokens.length > 0 && (
  <p className="-mt-2 mb-3 text-xs text-muted-foreground">
    {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
  </p>
)}
```

- [ ] **Step 4: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run test && npm run build`
Expected: 全通过。手工 `npm run dev` 过一遍：token 增删、建议键盘流、清除全部、刷新后筛选恢复、旧 key 迁移（先在 localStorage 手动种 `dashboard.sessions.filterScore=["5"]` 验证）。

- [ ] **Step 5: Commit**

```bash
git add "web/src/app/(dashboard)/sessions/page.tsx"
git commit -m "refactor(web): migrate sessions page filters to faceted filter bar"
```

---

### Task 6: audit/model 页迁移

**Files:**
- Modify: `web/src/app/(dashboard)/audit/model/page.tsx`

**Interfaces:**
- Consumes: Task 5 范式；`api.listAuditOptions({ field, startTime, endTime })`（field: `"user" | "model" | "status" | "ua"`）；`api.listAuditLogs({ page, pageSize, query, sort, sortField, startTime, endTime, filter })`。

- [ ] **Step 1: facet 配置与 hook**

删除 `filterUser`/`filterModel`/`filterStatus`/`filterUA` 四个 state、四个 options state 与对应 options fetch、内联搜索框相关 `searchQuery` state。替换为：

```ts
const fetchOptionsFor = useCallback(
  (field: "user" | "model" | "status" | "ua") => async () => {
    const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
    const rsp = await api.listAuditOptions({ field, startTime, endTime });
    return rsp.items ?? [];
  },
  [timeRange, customStart, customEnd],
);

const facets = useMemo<FacetDef[]>(
  () => [
    { key: "user", label: t("audit.filter_user"), options: fetchOptionsFor("user") },
    { key: "model", label: t("audit.filter_model"), options: fetchOptionsFor("model") },
    { key: "status", label: t("audit.filter_status"), options: fetchOptionsFor("status") },
    { key: "ua", label: t("audit.filter_ua"), options: fetchOptionsFor("ua") },
  ],
  [t, fetchOptionsFor],
);

const filterBar = useFilterBar({
  persistKey: "dashboard.auditModel",
  facets,
  freeTextPlaceholder: t("audit.search_placeholder"),
  optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
});
const { queryParams } = filterBar;
```

注意：该页现状筛选**未持久化**（`useState`），无 legacyKeys——迁移后自动获得持久化（spec 指定的行为改进）。

- [ ] **Step 2: fetchLogs 改对象参数 + queryParams effect**

按 Task 5 Step 2 同范式收敛（对象 `AuditLogsQuery { page, pageSize, range, cs, ce, sortState, qp, silent? }`），`filter` 与 `query` 取自 `qp`：

```ts
const rsp = await api.listAuditLogs({
  page: q.page,
  pageSize: q.pageSize,
  sort: q.sortState.dir,
  sortField: q.sortState.field,
  startTime,
  endTime,
  query: q.qp.freeText || undefined,
  filter: q.qp.filter,
});
```

挂载 effect 替换为 `[queryParams]` effect（同 Task 5，含 eslint-disable 注释）。

- [ ] **Step 3: 工具栏 JSX 替换**

删除 `{/* Filters */}` 块中 4 个 `MultiSelectPill`、清除按钮、以及**内联重复的搜索框 markup**（`div.relative.w-full.md:max-w-sm` 含 `Search` 图标 + `Input`，本任务顺带收编这段重复代码）。替换为 Task 5 Step 3 同构布局（`TimeRangePicker` + `<FilterBar {...filterBar} facets={facets} placeholder={t("audit.search_placeholder")} />` + applied_count 行）。

- [ ] **Step 4: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`

```bash
git add "web/src/app/(dashboard)/audit/model/page.tsx"
git commit -m "refactor(web): migrate audit model page to faceted filter bar"
```

---

### Task 7: audit/cron 页迁移

**Files:**
- Modify: `web/src/app/(dashboard)/audit/cron/page.tsx`

**Interfaces:**
- Consumes: Task 5 范式；`api.listCronCallAuditOptions({ field, startTime, endTime })`（field: `"type" | "status"`）；`api.listCronCallAudits({ page, pageSize, query, sort, sortField, startTime, endTime, filter })`。

- [ ] **Step 1: facet 配置与 hook**

删除 `filterType`/`filterStatus` state、`typeOptions`/`statusOptions` 与其 fetch、`buildCronAuditFilter`、`searchQuery` state。替换为：

```ts
const fetchOptionsFor = useCallback(
  (field: "type" | "status") => async () => {
    const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
    const rsp = await api.listCronCallAuditOptions({ field, startTime, endTime });
    return rsp.items ?? [];
  },
  [timeRange, customStart, customEnd],
);

const facets = useMemo<FacetDef[]>(
  () => [
    { key: "type", label: t("cron_audit.filter_type"), options: fetchOptionsFor("type") },
    { key: "status", label: t("cron_audit.filter_status"), options: fetchOptionsFor("status") },
  ],
  [t, fetchOptionsFor],
);

const filterBar = useFilterBar({
  persistKey: "dashboard.cronAudit",
  facets,
  freeTextPlaceholder: t("cron_audit.search_placeholder"),
  optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
});
```

该页 `timeRange` 现状是 `useState`（默认 `"24h"`，未持久化）——保持现状不动，只迁筛选。

- [ ] **Step 2: fetchLogs 对象参数化 + queryParams effect + 工具栏替换**

同 Task 6 Step 2/3 范式：`buildCronAuditFilter` 删除，`filter`/`query` 取自 `qp`；status 值建议沿用后端原始字符串，无需 formatValue；工具栏替换为 `TimeRangePicker + FilterBar` 布局 + applied_count 行。

- [ ] **Step 3: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`

```bash
git add "web/src/app/(dashboard)/audit/cron/page.tsx"
git commit -m "refactor(web): migrate cron audit page to faceted filter bar"
```

---

### Task 8: users 页迁移（param facet）

**Files:**
- Modify: `web/src/app/(dashboard)/users/page.tsx`

**Interfaces:**
- Consumes: Task 5 范式；`api.listUsers(page, pageSize, { query, permission })`。

- [ ] **Step 1: facet 配置与 hook**

删除 `permission` state、`PERMISSIONS` 的 `Select` 筛选 UI（`Select`/`SelectTrigger`/`SelectContent`/`SelectItem` import 若不再用于别处一并删除）、`searchQuery` state。替换为：

```ts
const facets = useMemo<FacetDef[]>(
  () => [
    {
      key: "permission",
      label: t("users.permission_filter"),
      options: ["pending", "demo", "user", "admin"],
      target: "param",
      single: true,
      formatValue: (v) => t(`permission.${v}`),
    },
  ],
  [t],
);

const filterBar = useFilterBar({
  persistKey: "dashboard.users",
  facets,
  freeTextPlaceholder: t("users.search_placeholder"),
});
const { queryParams } = filterBar;
```

- [ ] **Step 2: fetchUsers 调用点改造**

`permission` 取自 `queryParams.params.permission`（`target:"param"` + `single` 保证至多一个 token）：

```ts
fetchUsers(page, pageSize, {
  query: queryParams.freeText || undefined,
  permission: queryParams.params.permission,
});
```

`listUsers` 签名为 `(page, pageSize, opts)`——页面 `fetchUsers` wrapper 保持原样仅改参数来源；挂载/搜索 effect 合并为 `[queryParams]` effect（同 Task 5）。

- [ ] **Step 3: 工具栏 JSX 替换**

`div.mb-4.flex.flex-wrap.items-center.gap-3` 内的 `Select` 与 `SearchInput` 替换为：

```tsx
<div className="mb-4 flex">
  <FilterBar {...filterBar} facets={facets} placeholder={t("users.search_placeholder")} />
</div>
{filterBar.tokens.length > 0 && (
  <p className="-mt-2 mb-3 text-xs text-muted-foreground">
    {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
  </p>
)}
```

- [ ] **Step 4: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`

```bash
git add "web/src/app/(dashboard)/users/page.tsx"
git commit -m "refactor(web): migrate users page to faceted filter bar"
```

---

### Task 9: 6 个 freeText-only 页迁移

**Files:**
- Modify: `web/src/app/(dashboard)/trace/page.tsx`
- Modify: `web/src/app/(dashboard)/cron/page.tsx`
- Modify: `web/src/app/(dashboard)/models/page.tsx`
- Modify: `web/src/app/(dashboard)/apikeys/page.tsx`
- Modify: `web/src/app/(dashboard)/endpoints/page.tsx`
- Modify: `web/src/app/(dashboard)/trigger/page.tsx`

**Interfaces:**
- Consumes: Task 5 范式；各页 list API 的 `query` 参数（已逐一确认：`api.listTraces(page, pageSize, query)`、`api.listCronJobs({page, pageSize, query})`、`api.listModels(page, pageSize, query)`、`api.listAPIKeys(page, pageSize, query)`、`api.listEndpoints(page, pageSize, query)`、`api.listTrigger(page, pageSize, query)`）。

- [ ] **Step 1: 统一模式（每页相同结构，以下用 trace 作完整示例，其余页替换对照表中的值）**

trace 页：删除 `keyword`/`searchInput` 两个 persistent state 与 `handleSearch`/`onClear`，替换为：

```ts
const filterBar = useFilterBar({
  persistKey: "dashboard.trace",
  facets: [],
  freeTextPlaceholder: t("trace.search_placeholder"),
  legacyFreeTextKey: "dashboard.trace.keyword",
  legacyKeys: { _draft: "dashboard.trace.searchInput" },
});
const { queryParams } = filterBar;
```

`legacyKeys` 的 `_draft` 无对应 facet，不会产生 token，仅用于把草稿 key 纳入挂载清理列表。

挂载/搜索触发合并为：

```ts
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时发起首次查询 */
useEffect(() => {
  fetchTraces(1, pageInfo.pageSize, queryParams.freeText);
}, [queryParams]);
/* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
```

`fetchTraces(page, pageSize, kw)` 内部 `api.listTraces(page, pageSize, kw || undefined)` 调用不变（`kw` 来源由 keyword state 改为 `queryParams.freeText`）。工具栏 JSX：

```tsx
<div className="mb-4 flex">
  <FilterBar {...filterBar} facets={[]} placeholder={t("trace.search_placeholder")} />
</div>
{filterBar.tokens.length > 0 && (
  <p className="-mt-2 mb-3 text-xs text-muted-foreground">
    {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
  </p>
)}
```

各页替换值对照表：

| 页面 | persistKey | placeholder i18n | legacyFreeTextKey | legacyKeys（`_draft` 清理） | fetch 调用 |
|---|---|---|---|---|---|
| trace | `dashboard.trace` | `trace.search_placeholder` | `dashboard.trace.keyword` | `dashboard.trace.searchInput` | `fetchTraces(1, pageSize, freeText)` |
| cron | `dashboard.cron` | `cron.search_placeholder` | 无 | 无 | `refresh(1)`，query 来源改 `freeText` |
| models | `dashboard.models` | `models.search_placeholder` | 无 | 无 | `fetchData(1, pageSize, freeText || undefined)` |
| apikeys | `dashboard.apikeys` | `apikeys.search_keys` | 无 | 无 | `fetchAPIKeys(1, pageSize, freeText || undefined)` |
| endpoints | `dashboard.endpoints` | `endpoints.search_endpoints` | 无 | 无 | `fetchEndpoints(1, pageSize, freeText || undefined)` |
| trigger | `dashboard.trigger` | `trigger.search_placeholder` | 无 | 无 | `fetchTriggers(1, pageSize, freeText || undefined)` |

cron/models/apikeys/endpoints/trigger 五页搜索词现状均未持久化（`useState`），无 legacy 迁移；各页 fetch wrapper 签名不同，仅把 `searchQuery` state 来源替换为 `queryParams.freeText`，删除 `onSearch`/`handleSearch` 回调与 `SearchInput` import。

- [ ] **Step 2: 逐页验证**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`
Expected: 全通过；每页 `SearchInput` import 移除后无未用引用。

- [ ] **Step 3: Commit**

```bash
git add "web/src/app/(dashboard)/trace/page.tsx" "web/src/app/(dashboard)/cron/page.tsx" "web/src/app/(dashboard)/models/page.tsx" "web/src/app/(dashboard)/apikeys/page.tsx" "web/src/app/(dashboard)/endpoints/page.tsx" "web/src/app/(dashboard)/trigger/page.tsx"
git commit -m "refactor(web): migrate six search-only list pages to faceted filter bar"
```

---

### Task 10: 引用清查 + 全量验证

**Files:**
- Modify: `web/src/components/search-input.tsx`（仅当全库无引用时删除）

- [ ] **Step 1: 确认组件引用情况**

Run: `cd web && grep -rn "MultiSelectPill\|SearchInput" src/ --include="*.tsx" --include="*.ts"`
Expected: `MultiSelectPill` 仅剩 `dataset/page.tsx` 引用（dataset 属 spec 排除范围，其导出配置语义继续依赖该组件——**`multi-select-pill.tsx` 与 `multi_select.*` i18n key 保留不删**）；`SearchInput` 应无任何引用，若有（如 session-detail/share 等非列表页）则保留，无引用则删除 `web/src/components/search-input.tsx`。

- [ ] **Step 2: 全量验证**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 全通过。

手工清单（`npm run dev`）：
- 10 页逐页：token 增删、键盘流（输入 → facet 建议 → `:` → 值建议 → Enter）、Backspace 删 token、清除全部、刷新后恢复
- sessions 旧 key 迁移（手动种 `dashboard.sessions.filterModel=["gpt-5.2"]` 验证一次性迁移与旧 key 删除）
- 窄屏（<768px）换行不溢出
- 语言切换 zh/en/ja 建议分组标题与 facet label 跟随

- [ ] **Step 3: Commit（如有删除）**

```bash
git add -A web/
git commit -m "chore(web): retire SearchInput component after filter bar migration"
```

---

## Self-Review 记录

- **Spec 覆盖**：spec §3.1 四模块 → Task 1-3 ✓；§3.5 交互契约 → Task 3 组件实现 ✓；§3.6 持久化与迁移 → Task 2 hook + 各页 legacyKeys ✓；§4 迁移矩阵 10 页 → Task 5-9 ✓；§5 i18n → Task 4 ✓；§6.1 vitest → Task 1 ✓。dataset/shares 排除 → Task 10 Step 1 显式保留 MultiSelectPill 供 dataset 使用（修正 spec §4 退役清单中"退役 MultiSelectPill"的表述——dataset 仍依赖它）。
- **Placeholder 扫描**：无 TBD/TODO；每个 Task 含完整代码或精确替换值。
- **类型一致性**：`FilterToken`（Task 1）→ `types.ts` re-export（Task 2）→ `FilterBarProps`（Task 3）→ 页面 `<FilterBar {...filterBar} facets={facets} />`（Task 5-9）一致；`FilterBarQueryParams { filter?, freeText, params }` 在 Task 5-9 消费一致。
