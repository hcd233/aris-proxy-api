# Playground 布局与交互重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Playground 从「表单 + 纯文本输出」重构为对话式调试工作台：两栏布局（本地会话列表 + 全宽对话流）、参数抽屉、全自由消息编辑、消息级折叠调试详情。

**Architecture:** 纯前端重构、后端零改动。纯逻辑（SSE 解析、费用估算、会话存储）集中在 `playground-logic.ts` 供 vitest 直测；展示与交互拆到 `components/playground/` 五个组件；`page.tsx` 只做状态装配与运行链路。复用 `components/chat/markdown-lite.tsx` 渲染 markdown。

**Tech Stack:** Next.js 16（静态导出）+ React 19 + Tailwind v4 + shadcn/ui（base-nova）+ vitest。

**Spec:** `docs/superpowers/specs/2026-10-09-playground-redesign-design.md`（决策与语义以此为准）

## Global Constraints

- 分支：`feature/playground-chat-redesign-2026-10-09`，在 `.worktrees/` 下开发；worktree 内软链 `node_modules`（`ln -s ../../../web/node_modules .worktrees/<name>/web/node_modules`），**禁止 `npm ci`**。
- `playground-logic.ts` **运行时禁止项目内 import**（type-only import 允许，编译期擦除后 vitest 无需路径别名直接加载）。
- 无组件测试库（package.json 只有 vitest）：纯逻辑改动必须带 vitest 用例；组件改动以 `npm run lint && npm run build` 通过为验收。
- 样式：Tailwind v4 utility + `globals.css` CSS 变量，禁止内联 `style` 定值与硬编码 hex；图标只用 `lucide-react`；toast 用 `sonner`；HTTP 只走 `api.*`（禁止业务组件 `fetch`）。
- i18n：`web/src/locales/{en,zh,ja}.json` 三份同步维护，扁平 key（如 `playground.send`）。
- lint 强制：`truncate` / `line-clamp-1` 元素必须在 `TooltipTrigger` 子树内；恒定短占位（`—`）不加截断类。
- 每个任务以独立 commit 收尾（`feat(playground): …` / `test(playground): …` / `refactor(playground): …`）；pre-commit hook 自动 prettier，提交前跑 `npm run format:check`。
- 最终验证（Task 8）：`cd web && npm run lint && npm run test && npm run build` 三件套全绿。

## File Structure

| 文件 | 动作 | 职责 |
|---|---|---|
| `web/src/app/(dashboard)/playground/playground-logic.ts` | 修改 | 纯逻辑：SSE 解析、费用估算、curl 构造、会话/参数存储（无 React） |
| `web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts` | 修改 | 纯逻辑 vitest 用例 |
| `web/src/app/(dashboard)/playground/page.tsx` | 重写 | 状态装配 + 运行链路（发送/流式/停止/重跑/编辑截断） |
| `web/src/components/playground/session-sidebar.tsx` | 新建 | 本地会话列表（新建/切换/重命名/删除） |
| `web/src/components/playground/conversation-view.tsx` | 新建 | 对话流容器 + 自动滚底 |
| `web/src/components/playground/chat-turn.tsx` | 新建 | 单条消息：气泡/操作条/编辑态/指标行/错误卡 |
| `web/src/components/playground/composer.tsx` | 新建 | 输入区：多行输入/角色插入/发送/停止 |
| `web/src/components/playground/params-drawer.tsx` | 新建 | 参数抽屉（Sheet） |
| `web/src/locales/{en,zh,ja}.json` | 修改 | 新 i18n key |
| `web/CONTEXT.md` | 修改 | 补 Playground 术语（Playground Session / Turn Meta / 估算费用） |

跨任务接口（Consumes/Produces）在各任务头部声明；类型以 Task 1–3 的定义为准，后续任务不得改名。

---

### Task 1: SSE 解析纯逻辑（parseSSELine + ChatUsage）

**Files:**
- Modify: `web/src/app/(dashboard)/playground/playground-logic.ts`
- Test: `web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts`

**Interfaces:**
- Consumes: 无（首任务）
- Produces（后续任务依赖，签名固定）:

```ts
export interface ChatUsage {
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
}

export type SSEEvent =
  | { type: "delta"; text: string }
  | { type: "usage"; usage: ChatUsage }
  | { type: "done" }
  | { type: "skip" };

export function parseSSELine(line: string): SSEEvent;
```

同时**删除**旧 `extractDeltaContent`（被 `parseSSELine` 取代，不留双份），其既有测试改写进本任务用例。

- [ ] **Step 1: 写失败测试**

在 `playground-logic.test.ts` 中删除 `extractDeltaContent` 的 describe 块（若有）与对应 import，新增：

```ts
import { parseSSELine } from "../playground-logic";

describe("parseSSELine", () => {
  it("提取内容增量", () => {
    const line = 'data: {"choices":[{"delta":{"content":"你好"}}]}';
    expect(parseSSELine(line)).toEqual({ type: "delta", text: "你好" });
  });

  it("识别 usage 尾帧（choices 空 + usage）", () => {
    const line =
      'data: {"choices":[],"usage":{"prompt_tokens":812,"completion_tokens":408,"prompt_tokens_details":{"cached_tokens":512}}}';
    expect(parseSSELine(line)).toEqual({
      type: "usage",
      usage: { promptTokens: 812, completionTokens: 408, cachedTokens: 512 },
    });
  });

  it("usage 缺 cached_tokens 时归零", () => {
    const line = 'data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}';
    expect(parseSSELine(line)).toEqual({
      type: "usage",
      usage: { promptTokens: 10, completionTokens: 5, cachedTokens: 0 },
    });
  });

  it("[DONE] 与空 data 行分别为 done / skip", () => {
    expect(parseSSELine("data: [DONE]")).toEqual({ type: "done" });
    expect(parseSSELine("data:")).toEqual({ type: "skip" });
  });

  it("role 帧、空行、非 data 行、畸形 JSON 均为 skip", () => {
    expect(parseSSELine('data: {"choices":[{"delta":{"role":"assistant"}}]}')).toEqual({
      type: "skip",
    });
    expect(parseSSELine("")).toEqual({ type: "skip" });
    expect(parseSSELine("event: message")).toEqual({ type: "skip" });
    expect(parseSSELine("data: {not-json")).toEqual({ type: "skip" });
  });

  it("delta.content 为空串时为 skip", () => {
    expect(parseSSELine('data: {"choices":[{"delta":{"content":""}}]}')).toEqual({
      type: "skip",
    });
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: FAIL（`parseSSELine` 未导出）

- [ ] **Step 3: 最小实现**

在 `playground-logic.ts` 中：删除 `extractDeltaContent` 整个函数（含 JSDoc），改为：

```ts
/** 一次调用的用量（OpenAI usage 子集） */
export interface ChatUsage {
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
}

/** 一条 SSE 行的解析结果：内容增量 / 用量尾帧 / 结束 / 忽略 */
export type SSEEvent =
  | { type: "delta"; text: string }
  | { type: "usage"; usage: ChatUsage }
  | { type: "done" }
  | { type: "skip" };

/**
 * 解析单条 SSE 行。
 * usage 帧为 OpenAI 流式尾帧（choices 空数组 + usage，需 stream_options.include_usage）。
 */
export function parseSSELine(line: string): SSEEvent {
  const trimmed = line.trim();
  if (!trimmed.startsWith("data:")) return { type: "skip" };
  const payload = trimmed.slice("data:".length).trim();
  if (payload === "") return { type: "skip" };
  if (payload === "[DONE]") return { type: "done" };
  let chunk: {
    choices?: { delta?: { content?: string } }[];
    usage?: {
      prompt_tokens?: number;
      completion_tokens?: number;
      prompt_tokens_details?: { cached_tokens?: number };
    };
  };
  try {
    chunk = JSON.parse(payload) as typeof chunk;
  } catch {
    return { type: "skip" };
  }
  if (chunk.usage) {
    return {
      type: "usage",
      usage: {
        promptTokens: chunk.usage.prompt_tokens ?? 0,
        completionTokens: chunk.usage.completion_tokens ?? 0,
        cachedTokens: chunk.usage.prompt_tokens_details?.cached_tokens ?? 0,
      },
    };
  }
  const text = chunk.choices?.[0]?.delta?.content;
  if (typeof text === "string" && text !== "") return { type: "delta", text };
  return { type: "skip" };
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: PASS（含其余既有用例）

- [ ] **Step 5: Commit**

```bash
git add "web/src/app/(dashboard)/playground/playground-logic.ts" "web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts"
git commit -m "feat(playground): SSE 解析纯逻辑 parseSSELine（delta/usage/done/skip）"
```

---

### Task 2: 费用估算与 curl 构造（estimateCost / buildCurl）

**Files:**
- Modify: `web/src/app/(dashboard)/playground/playground-logic.ts`
- Test: `web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts`

**Interfaces:**
- Consumes: `ChatUsage`（Task 1）
- Produces:

```ts
export function estimateCost(usage: ChatUsage, pricing?: PricingDTO): number | null;
export function buildCurl(body: Record<string, unknown>, origin: string): string;
```

其中 `PricingDTO` 为 type-only import：`import type { PricingDTO } from "@/lib/types";`。

- [ ] **Step 1: 写失败测试**

```ts
import { buildCurl, estimateCost, type ChatUsage } from "../playground-logic";
import type { PricingDTO } from "@/lib/types";

const usage: ChatUsage = { promptTokens: 812, completionTokens: 408, cachedTokens: 512 };

describe("estimateCost", () => {
  it("按无条件默认规则估算：(prompt-cached)×input + cached×cacheRead + completion×output，单位每 1M", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [
        {
          time_windows: [{ start: "09:00", end: "18:00" }],
          context_min: 128000,
          input_price: 99,
          output_price: 99,
          cache_creation_price: 99,
          cache_read_price: 99,
        },
        {
          input_price: 2.5,
          output_price: 10,
          cache_creation_price: 3.75,
          cache_read_price: 0.5,
        },
      ],
    };
    // (812-512)*2.5 + 512*0.5 + 408*10 = 750 + 256 + 4080 = 5086 / 1e6
    expect(estimateCost(usage, pricing)).toBeCloseTo(0.005086, 8);
  });

  it("无条件默认规则缺失时返回 null（不按其他规则猜）", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [{ context_min: 0, context_max: 128000, input_price: 1, output_price: 2, cache_creation_price: 1, cache_read_price: 1 }],
    };
    expect(estimateCost(usage, pricing)).toBeNull();
  });

  it("未计价（无 currency）或无 pricing 返回 null", () => {
    expect(estimateCost(usage, undefined)).toBeNull();
    expect(estimateCost(usage, { rules: [{ input_price: 1, output_price: 1, cache_creation_price: 1, cache_read_price: 1 }] })).toBeNull();
  });

  it("cached 大于 prompt 时负输入按 0 计", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [{ input_price: 1, output_price: 1, cache_creation_price: 1, cache_read_price: 1 }],
    };
    expect(estimateCost({ promptTokens: 10, completionTokens: 0, cachedTokens: 50 }, pricing)).toBeCloseTo(0.00005, 8);
  });
});

describe("buildCurl", () => {
  it("生成指向 OpenAI 兼容端点的 curl，Key 脱敏占位", () => {
    const curl = buildCurl({ model: "gpt-4o", messages: [{ role: "user", content: "hi" }] }, "https://api.example.com");
    expect(curl).toContain("curl https://api.example.com/api/openai/v1/chat/completions");
    expect(curl).toContain('Authorization: Bearer <API_KEY>');
    expect(curl).toContain("-d '");
    expect(curl).toContain('"model": "gpt-4o"');
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: FAIL（`estimateCost` / `buildCurl` 未导出）

- [ ] **Step 3: 最小实现**

`playground-logic.ts` 顶部加 type import：`import type { PricingDTO } from "@/lib/types";`，并新增：

```ts
/**
 * 按模型定价的「无条件默认规则」（代表档）估算费用（展示单位）。
 * 口径：(prompt − cached)×input + cached×cache_read + completion×output，单价为每 1M tokens。
 * 不做时段/上下文区间匹配（防与审计口径漂移）；未计价或缺默认规则返回 null。
 */
export function estimateCost(usage: ChatUsage, pricing?: PricingDTO): number | null {
  const rule = pricing?.rules?.find(
    (r) =>
      (!r.time_windows || r.time_windows.length === 0) &&
      !r.context_min &&
      !r.context_max,
  );
  if (!pricing?.currency || !rule) return null;
  const billableInput = Math.max(0, usage.promptTokens - usage.cachedTokens);
  return (
    (billableInput * rule.input_price +
      usage.cachedTokens * rule.cache_read_price +
      usage.completionTokens * rule.output_price) /
    1_000_000
  );
}

/** 生成可直接执行的 curl 命令（指向网关 OpenAI 兼容端点，Key 用 <API_KEY> 占位） */
export function buildCurl(body: Record<string, unknown>, origin: string): string {
  const json = JSON.stringify(body, null, 2);
  return [
    `curl ${origin}/api/openai/v1/chat/completions \\`,
    '  -H "Authorization: Bearer <API_KEY>" \\',
    '  -H "Content-Type: application/json" \\',
    `  -d '${json.replace(/'/g, "'\\''")}'`,
  ].join("\n");
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add "web/src/app/(dashboard)/playground/playground-logic.ts" "web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts"
git commit -m "feat(playground): 费用代表档估算与 curl 复制构造"
```

---

### Task 3: 会话与参数本地存储（sessions/params 持久化纯逻辑）

**Files:**
- Modify: `web/src/app/(dashboard)/playground/playground-logic.ts`
- Test: `web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts`

**Interfaces:**
- Consumes: 无（`ChatUsage` 仅被 `PlaygroundTurnMeta` 引用）
- Produces（Task 4–7 依赖，签名固定）:

```ts
export interface PlaygroundTurnMeta {
  model: string;
  usage?: ChatUsage;
  firstTokenMs?: number;
  totalMs: number;
  cost?: number | null;
  interrupted?: boolean;
  requestSnapshot?: Record<string, unknown>;
  error?: { status?: number; message: string };
}

export interface PlaygroundMessage {
  id: string;
  role: "system" | "user" | "assistant";
  content: string;
  meta?: PlaygroundTurnMeta;
}

export interface PlaygroundSession {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  messages: PlaygroundMessage[];
}

export interface PlaygroundParams {
  model: string;
  apiKeyID: number | null;
  temperature: string;
  maxTokens: string;
  stream: boolean;
}

export const SESSIONS_STORAGE_KEY = "playground.sessions.v1";
export const PARAMS_STORAGE_KEY = "playground.params.v1";
export const MAX_SESSIONS = 50;

export function loadSessions(): PlaygroundSession[];
export function saveSessions(sessions: PlaygroundSession[]): boolean; // false = 写入失败（配额满）
export function upsertSession(sessions: PlaygroundSession[], session: PlaygroundSession): PlaygroundSession[]; // 更新或插入并置顶，超 MAX_SESSIONS 淘汰 updatedAt 最旧
export function deriveTitle(messages: PlaygroundMessage[]): string; // 首条非空 user 消息前 30 字，无则 ""
export function loadParams(): PlaygroundParams | null;
export function saveParams(params: PlaygroundParams): void;
export function newSession(): PlaygroundSession;
export function newMessage(role: PlaygroundMessage["role"], content: string): PlaygroundMessage;
export function truncateAfter(messages: PlaygroundMessage[], index: number): PlaygroundMessage[]; // 保留 [0..index]，编辑保存用
```

注意：`buildChatBody` 的 messages 参数类型改为 `Pick<PlaygroundMessage, "role" | "content">[]`（兼容既有测试的无 id 对象）；`stream_options: { include_usage: true }` 不进纯函数、由 Task 7 页面层在 `buildChatBody` 结果上 spread 追加（故不在本任务测试范围）。

- [ ] **Step 1: 写失败测试**

测试需 mock localStorage（vitest 无 jsdom 时用简单 stub）：

```ts
import {
  deriveTitle,
  loadParams,
  loadSessions,
  MAX_SESSIONS,
  newMessage,
  newSession,
  PARAMS_STORAGE_KEY,
  SESSIONS_STORAGE_KEY,
  saveParams,
  saveSessions,
  truncateAfter,
  upsertSession,
  type PlaygroundMessage,
  type PlaygroundSession,
} from "../playground-logic";

function stubStorage() {
  const store = new Map<string, string>();
  const storage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  };
  vi.stubGlobal("localStorage", storage);
  return store;
}

function msg(role: PlaygroundMessage["role"], content: string, id = content): PlaygroundMessage {
  return { id, role, content };
}

describe("会话存储", () => {
  it("写读回环；损坏 JSON / 版本不符降级为空", () => {
    const store = stubStorage();
    const sessions = [newSession()];
    expect(saveSessions(sessions)).toBe(true);
    expect(loadSessions()).toEqual(sessions);

    store.set(SESSIONS_STORAGE_KEY, "{broken");
    expect(loadSessions()).toEqual([]);
    store.set(SESSIONS_STORAGE_KEY, JSON.stringify({ version: 99, sessions: [1] }));
    expect(loadSessions()).toEqual([]);
    vi.unstubAllGlobals();
  });

  it("upsertSession 更新置顶、插入新会话并淘汰最旧", () => {
    const base = Array.from({ length: MAX_SESSIONS }, (_, i) => ({
      ...newSession(),
      id: `s${i}`,
      updatedAt: 1000 + i,
    }));
    const inserted = upsertSession(base, { ...newSession(), id: "new" });
    expect(inserted).toHaveLength(MAX_SESSIONS);
    expect(inserted[0].id).toBe("new");
    expect(inserted.map((s) => s.id)).not.toContain("s0"); // 最旧被淘汰

    const updated = upsertSession(inserted, { ...inserted[5], title: "改" });
    expect(updated[0].id).toBe(inserted[5].id);
    expect(updated[0].title).toBe("改");
  });

  it("deriveTitle 取首条非空 user 消息前 30 字", () => {
    expect(deriveTitle([msg("system", "sys"), msg("user", "你好".repeat(20))])).toHaveLength(30);
    expect(deriveTitle([msg("system", "only system")])).toBe("");
    expect(deriveTitle([])).toBe("");
  });

  it("truncateAfter 保留 [0..index]", () => {
    const msgs = [msg("user", "a"), msg("assistant", "b"), msg("user", "c")];
    expect(truncateAfter(msgs, 0)).toEqual([msgs[0]]);
    expect(truncateAfter(msgs, 2)).toEqual(msgs);
  });

  it("newMessage / newSession 生成 id 与时间戳", () => {
    const m = newMessage("user", "hi");
    expect(m.role).toBe("user");
    expect(m.id).not.toBe("");
    const s = newSession();
    expect(s.messages).toEqual([]);
    expect(s.title).toBe("");
    expect(s.createdAt).toBeGreaterThan(0);
  });
});

describe("参数存储", () => {
  it("写读回环；无数据或损坏返回 null", () => {
    const store = stubStorage();
    expect(loadParams()).toBeNull();
    const params = { model: "gpt-4o", apiKeyID: 7, temperature: "0.7", maxTokens: "", stream: true };
    saveParams(params);
    expect(loadParams()).toEqual(params);
    store.set(PARAMS_STORAGE_KEY, "{broken");
    expect(loadParams()).toBeNull();
    vi.unstubAllGlobals();
  });
});
```

测试顶部 import 补 `import { vi } from "vitest";`（若既有 import 行已有 `describe/expect/it` 则合并）。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: FAIL（未导出各存储函数）

- [ ] **Step 3: 最小实现**

`playground-logic.ts` 新增（`PlaygroundMessage` 用本定义**替换**既有同名接口 `{ role, content }`，并把 `buildChatBody` 参数类型改为 `Pick<PlaygroundMessage, "role" | "content">[]`）：

```ts
/** 单轮调试元信息（跟随其所属 assistant/user 消息持久化） */
export interface PlaygroundTurnMeta {
  model: string;
  usage?: ChatUsage;
  firstTokenMs?: number;
  totalMs: number;
  cost?: number | null;
  interrupted?: boolean;
  requestSnapshot?: Record<string, unknown>;
  error?: { status?: number; message: string };
}

/** 调试消息（对话流的一条） */
export interface PlaygroundMessage {
  id: string;
  role: "system" | "user" | "assistant";
  content: string;
  meta?: PlaygroundTurnMeta;
}

/** 本地调试会话（localStorage 持久化） */
export interface PlaygroundSession {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  messages: PlaygroundMessage[];
}

/** 调试参数（跨会话记忆） */
export interface PlaygroundParams {
  model: string;
  apiKeyID: number | null;
  temperature: string;
  maxTokens: string;
  stream: boolean;
}

export const SESSIONS_STORAGE_KEY = "playground.sessions.v1";
export const PARAMS_STORAGE_KEY = "playground.params.v1";
/** 会话数量上限，超出淘汰 updatedAt 最旧者 */
export const MAX_SESSIONS = 50;

function readJSON<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

/** 读取本地会话；损坏或版本不符降级为空数组 */
export function loadSessions(): PlaygroundSession[] {
  const parsed = readJSON<{ version?: number; sessions?: PlaygroundSession[] }>(SESSIONS_STORAGE_KEY);
  if (!parsed || parsed.version !== 1 || !Array.isArray(parsed.sessions)) return [];
  return parsed.sessions;
}

/** 写入本地会话；返回 false 表示写入失败（如配额满） */
export function saveSessions(sessions: PlaygroundSession[]): boolean {
  try {
    localStorage.setItem(SESSIONS_STORAGE_KEY, JSON.stringify({ version: 1, sessions }));
    return true;
  } catch {
    return false;
  }
}

/** 更新或插入会话并置顶（updatedAt=now），超上限淘汰最旧 */
export function upsertSession(
  sessions: PlaygroundSession[],
  session: PlaygroundSession,
): PlaygroundSession[] {
  const rest = sessions.filter((s) => s.id !== session.id);
  return [{ ...session, updatedAt: Date.now() }, ...rest].slice(0, MAX_SESSIONS);
}

/** 会话标题：首条非空 user 消息前 30 字；无则空串（渲染方用 i18n 占位） */
export function deriveTitle(messages: PlaygroundMessage[]): string {
  const first = messages.find((m) => m.role === "user" && m.content.trim() !== "");
  return first ? first.content.trim().slice(0, 30) : "";
}

export function loadParams(): PlaygroundParams | null {
  return readJSON<PlaygroundParams>(PARAMS_STORAGE_KEY);
}

export function saveParams(params: PlaygroundParams): void {
  try {
    localStorage.setItem(PARAMS_STORAGE_KEY, JSON.stringify(params));
  } catch {
    // 参数丢失可容忍，不阻断调试
  }
}

export function newSession(): PlaygroundSession {
  return { id: crypto.randomUUID(), title: "", createdAt: Date.now(), updatedAt: Date.now(), messages: [] };
}

export function newMessage(role: PlaygroundMessage["role"], content: string): PlaygroundMessage {
  return { id: crypto.randomUUID(), role, content };
}

/** 保留 [0..index] 区间的消息（编辑保存后截断其后文用） */
export function truncateAfter(messages: PlaygroundMessage[], index: number): PlaygroundMessage[] {
  return messages.slice(0, index + 1);
}
```

- [ ] **Step 4: 跑测试确认通过（含 Task 1/2 与既有用例回归）**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/playground-logic.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add "web/src/app/(dashboard)/playground/playground-logic.ts" "web/src/app/(dashboard)/playground/__tests__/playground-logic.test.ts"
git commit -m "feat(playground): 会话与参数本地存储纯逻辑（版本降级/淘汰/截断）"
```

---

### Task 4: 参数抽屉与输入区（params-drawer / composer）

**Files:**
- Create: `web/src/components/playground/params-drawer.tsx`
- Create: `web/src/components/playground/composer.tsx`

**Interfaces:**
- Consumes: `PlaygroundParams`（Task 3）；`PlaygroundModelOption`（既有，本任务加 `pricing?: PricingDTO` 字段）
- Produces:

```ts
// params-drawer.tsx
interface ParamsDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: PlaygroundModelOption[];
  keys: APIKeyItem[];
  keysLoaded: boolean;
  params: PlaygroundParams;
  onParamsChange: (patch: Partial<PlaygroundParams>) => void;
}

// composer.tsx
interface ComposerProps {
  model: string;
  stream: boolean;
  sending: boolean;
  disabled: boolean;
  onSend: (content: string) => void;
  onStop: () => void;
  onInsertRole: (role: "system" | "assistant") => void;
  onOpenParams: () => void;
}
```

先在 `playground-logic.ts` 给 `PlaygroundModelOption` 加一行 `pricing?: PricingDTO;`（type-only 引用已有），`selectableModels` 不变（去重保序、保留对象原有字段）。

- [ ] **Step 1: 写 params-drawer.tsx**

```tsx
"use client";

import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { APIKeyItem } from "@/lib/types";
import type { PlaygroundModelOption, PlaygroundParams } from "@/app/(dashboard)/playground/playground-logic";
import { useT } from "@/lib/i18n";
import Link from "next/link";

interface ParamsDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: PlaygroundModelOption[];
  keys: APIKeyItem[];
  keysLoaded: boolean;
  params: PlaygroundParams;
  onParamsChange: (patch: Partial<PlaygroundParams>) => void;
}

export function ParamsDrawer({
  open,
  onOpenChange,
  models,
  keys,
  keysLoaded,
  params,
  onParamsChange,
}: ParamsDrawerProps) {
  const t = useT();
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="flex w-[21rem] flex-col gap-4 sm:max-w-[21rem]">
        <SheetHeader>
          <SheetTitle>{t("playground.params.title")}</SheetTitle>
        </SheetHeader>

        <div className="space-y-1">
          <Label htmlFor="pg-model">{t("playground.model")}</Label>
          <Select value={params.model} onValueChange={(v) => onParamsChange({ model: String(v) })}>
            <SelectTrigger id="pg-model" className="w-full">
              <SelectValue placeholder={t("playground.model.placeholder")} />
            </SelectTrigger>
            <SelectContent>
              {models.map((m) => (
                <SelectItem key={m.alias} value={m.alias}>
                  {m.alias}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1">
          <Label htmlFor="pg-apikey">{t("playground.api_key")}</Label>
          {keysLoaded && keys.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("playground.api_key.empty")}{" "}
              <Link href="/apikeys/" className="underline underline-offset-2">
                {t("playground.api_key.create")}
              </Link>
            </p>
          ) : (
            <Select
              value={params.apiKeyID === null ? "" : String(params.apiKeyID)}
              onValueChange={(v) => onParamsChange({ apiKeyID: Number(v) })}
            >
              <SelectTrigger id="pg-apikey" className="w-full">
                <SelectValue placeholder={t("playground.api_key.placeholder")} />
              </SelectTrigger>
              <SelectContent>
                {keys.map((k) => (
                  <SelectItem key={k.id} value={String(k.id)}>
                    {k.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <p className="text-[11px] text-muted-foreground">{t("playground.api_key.hint")}</p>
        </div>

        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1">
            <Label htmlFor="pg-temperature">{t("playground.temperature")}</Label>
            <Input
              id="pg-temperature"
              type="number"
              min={0}
              max={2}
              step={0.1}
              value={params.temperature}
              onChange={(e) => onParamsChange({ temperature: e.target.value })}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="pg-max-tokens">{t("playground.max_tokens")}</Label>
            <Input
              id="pg-max-tokens"
              type="number"
              min={1}
              value={params.maxTokens}
              onChange={(e) => onParamsChange({ maxTokens: e.target.value })}
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Switch
            checked={params.stream}
            onCheckedChange={(v) => onParamsChange({ stream: v })}
            id="pg-stream"
          />
          <Label htmlFor="pg-stream">{t("playground.stream")}</Label>
        </div>
      </SheetContent>
    </Sheet>
  );
}
```

- [ ] **Step 2: 写 composer.tsx**

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Play, Plus, Settings2, Square } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface ComposerProps {
  model: string;
  stream: boolean;
  sending: boolean;
  disabled: boolean;
  onSend: (content: string) => void;
  onStop: () => void;
  onInsertRole: (role: "system" | "assistant") => void;
  onOpenParams: () => void;
}

export function Composer({
  model,
  stream,
  sending,
  disabled,
  onSend,
  onStop,
  onInsertRole,
  onOpenParams,
}: ComposerProps) {
  const t = useT();
  const [value, setValue] = useState("");

  const submit = () => {
    const content = value.trim();
    if (!content || disabled || sending) return;
    onSend(content);
    setValue("");
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => onInsertRole("system")}>
          <Plus className="size-3.5" />
          {t("playground.composer.insert_system")}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => onInsertRole("assistant")}>
          <Plus className="size-3.5" />
          {t("playground.composer.insert_assistant")}
        </Button>
      </div>

      <div className="rounded-xl border bg-background p-2.5 shadow-sm">
        <Textarea
          value={value}
          placeholder={t("playground.composer.placeholder")}
          className="min-h-[3.5rem] resize-none border-0 p-1 shadow-none focus-visible:ring-0"
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <div className="mt-1.5 flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <Badge variant="secondary" className="font-mono text-[10px]">
              {model || t("playground.model.placeholder")}
            </Badge>
            <Badge variant="secondary" className="text-[10px]">
              {stream ? t("playground.stream") : t("playground.composer.non_stream")}
            </Badge>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t("playground.params.open")}
              onClick={onOpenParams}
            >
              <Settings2 className="size-3.5" />
            </Button>
          </div>
          {sending ? (
            <Button type="button" variant="outline" size="sm" onClick={onStop}>
              <Square className="size-3.5" />
              {t("playground.stop")}
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              disabled={disabled || value.trim() === ""}
              onClick={submit}
              className={cn(value.trim() === "" && "opacity-80")}
            >
              <Play className="size-3.5" />
              {t("playground.send")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
```

注意：若 `web/src/components/ui/textarea.tsx` 不存在，先 `npx shadcn add textarea` 生成（不要手写散件）；`Badge` 已在 `ui/badge.tsx`。

- [ ] **Step 3: Lint + 构建验证**

Run: `cd web && npm run lint && npm run build`
Expected: 0 error（此时 page.tsx 尚未引用新组件，仅编译检查新文件）

- [ ] **Step 4: Commit**

```bash
git add web/src/components/playground/ "web/src/app/(dashboard)/playground/playground-logic.ts"
git commit -m "feat(playground): 参数抽屉与输入区组件"
```

---

### Task 5: 消息气泡组件（chat-turn：操作条/编辑态/指标详情/错误卡）

**Files:**
- Create: `web/src/components/playground/chat-turn.tsx`

**Interfaces:**
- Consumes: `PlaygroundMessage` / `PlaygroundTurnMeta` / `estimateCost` / `buildCurl`（Task 2/3）、`MarkdownLite`（`@/components/chat/markdown-lite`，props `{ text, raw?, className? }`）、`formatCost`（`@/lib/money`，`formatCost(v?: number | null)` 渲染 `—` 空值）
- Produces:

```ts
interface ChatTurnProps {
  message: PlaygroundMessage;
  index: number;
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}
```

- [ ] **Step 1: 写 chat-turn.tsx**

结构（单文件，内部小函数组件分区）：hover 操作条（`opacity-0 group-hover:opacity-100`）、编辑态（textarea + 保存并重跑/取消 + 警示）、指标行（`{tokens} tok · {耗时} · {费用}` 点击展开）、展开详情（输入/输出/缓存、首字延迟、请求体 JSON、复制为 curl）、错误卡（status + message + 重试）。

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { copyTextToClipboard } from "@/lib/clipboard";
import { MarkdownLite } from "@/components/chat/markdown-lite";
import { formatCost } from "@/lib/money";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import {
  buildCurl,
  type PlaygroundMessage,
} from "@/app/(dashboard)/playground/playground-logic";
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Copy,
  Pencil,
  RefreshCw,
  Sparkles,
  Trash2,
} from "lucide-react";

interface ChatTurnProps {
  message: PlaygroundMessage;
  index: number;
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

function formatDuration(ms?: number): string {
  if (ms === undefined) return "—";
  return ms < 1000 ? `${Math.round(ms)}ms` : `${(ms / 1000).toFixed(1)}s`;
}

export function ChatTurn({
  message,
  index,
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ChatTurnProps) {
  const t = useT();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message.content);
  const [detailsOpen, setDetailsOpen] = useState(false);

  const isAssistant = message.role === "assistant";
  const isUser = message.role === "user";
  const meta = message.meta;
  const usage = meta?.usage;
  const totalTokens = usage ? usage.promptTokens + usage.completionTokens : null;

  return (
    <div className="group/turn flex gap-3">
      {/* 头像 */}
      <div
        className={cn(
          "flex size-7 shrink-0 items-center justify-center rounded-full",
          isAssistant ? "bg-primary/15 text-primary" : "bg-muted text-muted-foreground",
        )}
      >
        {isAssistant ? (
          <Sparkles className="size-3.5" />
        ) : (
          <span className="text-[10px] font-medium">
            {message.role === "system" ? "S" : "U"}
          </span>
        )}
      </div>

      <div className="min-w-0 flex-1">
        {/* 角色标签 + hover 操作条 */}
        <div className="flex items-center justify-between">
          <span className="text-[10px] text-muted-foreground">{message.role}</span>
          {!sending && (
            <div className="flex gap-1 opacity-0 transition-opacity group-hover/turn:opacity-100">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.copy")}
                onClick={() => void copyTextToClipboard(message.content)}
              >
                <Copy className="size-3.5" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.edit")}
                onClick={() => {
                  setDraft(message.content);
                  setEditing(true);
                }}
              >
                <Pencil className="size-3.5" />
              </Button>
              {isAssistant && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("playground.turn.rerun")}
                  onClick={() => onRerun(index)}
                >
                  <RefreshCw className="size-3.5" />
                </Button>
              )}
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.delete")}
                onClick={() => onDelete(index)}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          )}
        </div>

        {/* 内容：编辑态 / 展示态 */}
        {editing ? (
          <div className="space-y-2">
            <Textarea
              value={draft}
              autoFocus
              className="min-h-[5rem]"
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Escape") setEditing(false);
              }}
            />
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                size="sm"
                onClick={() => {
                  setEditing(false);
                  onEditSave(index, draft);
                }}
              >
                {t("playground.turn.edit_save")}
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={() => setEditing(false)}>
                {t("playground.turn.edit_cancel")}
              </Button>
              <span className="text-[11px] text-destructive">
                {t("playground.turn.edit_warning")}
              </span>
            </div>
          </div>
        ) : (
          <div
            className={cn(
              "mt-1 text-[15px] leading-[1.6]",
              isUser &&
                "w-fit max-w-[85%] rounded-[20px] rounded-br-[6px] bg-accent px-4 py-2.5 md:max-w-[75%]",
            )}
          >
            {isAssistant ? (
              message.content === "" && !meta?.error && !meta?.interrupted ? (
                <span className="text-muted-foreground/60">…</span>
              ) : (
                <MarkdownLite text={message.content} />
              )
            ) : (
              <span className="whitespace-pre-wrap break-words">{message.content}</span>
            )}
          </div>
        )}

        {/* 错误卡 */}
        {meta?.error && (
          <div className="mt-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            <div className="min-w-0 flex-1">
              <div className="break-words">
                {meta.error.status ? `${meta.error.status} · ` : ""}
                {meta.error.message}
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="mt-1.5"
                onClick={() => onRetry(index)}
              >
                <RefreshCw className="size-3.5" />
                {t("playground.error.retry")}
              </Button>
            </div>
          </div>
        )}

        {/* 指标行 + 展开详情（仅 assistant 且有 meta） */}
        {isAssistant && meta && (
          <div className="mt-1.5">
            <button
              type="button"
              className="flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"
              onClick={() => setDetailsOpen((v) => !v)}
            >
              {detailsOpen ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}
              <span>
                {totalTokens !== null ? `${totalTokens} tok` : "—"} ·{" "}
                {formatDuration(meta.totalMs)} ·{" "}
                {formatCost(meta.cost)}
                {meta.interrupted ? ` · ${t("playground.metrics.interrupted")}` : ""}
              </span>
            </button>
            {detailsOpen && (
              <div className="mt-1.5 rounded-lg border bg-background p-2.5 text-[11px] text-muted-foreground">
                <div>
                  {t("playground.metrics.tokens_in")} <strong>{usage?.promptTokens ?? "—"}</strong>
                  {" · "}
                  {t("playground.metrics.tokens_cached")}{" "}
                  <strong>{usage?.cachedTokens ?? "—"}</strong>
                  {" · "}
                  {t("playground.metrics.tokens_out")}{" "}
                  <strong>{usage?.completionTokens ?? "—"}</strong>
                </div>
                <div>
                  {t("playground.metrics.first_token")}{" "}
                  <strong>{formatDuration(meta.firstTokenMs)}</strong>
                  {" · "}
                  {t("playground.metrics.total_time")} <strong>{formatDuration(meta.totalMs)}</strong>
                </div>
                <div className="mt-1">{t("playground.metrics.estimate_hint")}</div>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      void copyTextToClipboard(JSON.stringify(meta.requestSnapshot ?? {}, null, 2))
                    }
                  >
                    <Copy className="size-3.5" />
                    {t("playground.metrics.request_json")}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      void copyTextToClipboard(
                        buildCurl(meta.requestSnapshot ?? {}, window.location.origin),
                      )
                    }
                  >
                    <Copy className="size-3.5" />
                    {t("playground.metrics.copy_curl")}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Lint + 构建验证**

Run: `cd web && npm run lint && npm run build`
Expected: 0 error。重点自查：`truncate` 未使用（无此风险）；`window.location.origin` 仅在点击时调用（SSR 静态导出安全，组件 `"use client"`）。

- [ ] **Step 3: Commit**

```bash
git add web/src/components/playground/chat-turn.tsx
git commit -m "feat(playground): 消息气泡组件（hover 操作条/编辑态/指标详情/错误卡）"
```

---

### Task 6: 会话侧栏与对话流容器（session-sidebar / conversation-view）

**Files:**
- Create: `web/src/components/playground/session-sidebar.tsx`
- Create: `web/src/components/playground/conversation-view.tsx`

**Interfaces:**
- Consumes: `PlaygroundSession` / `deriveTitle`（Task 3）、`ChatTurn`（Task 5）
- Produces:

```ts
// session-sidebar.tsx
interface SessionSidebarProps {
  sessions: PlaygroundSession[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onRename: (id: string, title: string) => void;
  onDelete: (id: string) => void;
}

// conversation-view.tsx
interface ConversationViewProps {
  messages: PlaygroundMessage[];
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}
```

- [ ] **Step 1: 写 session-sidebar.tsx**

列表项：标题（`deriveTitle` 为空时显示 `t("playground.session.untitled")`）+ hover 操作（重命名进入内联 Input、删除直接删，本地数据无确认弹窗）。标题截断处按 lint 规则配 Tooltip：

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { MessageSquarePlus, Pencil, Trash2 } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { PlaygroundSession } from "@/app/(dashboard)/playground/playground-logic";

interface SessionSidebarProps {
  sessions: PlaygroundSession[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onRename: (id: string, title: string) => void;
  onDelete: (id: string) => void;
}

export function SessionSidebar({
  sessions,
  activeId,
  onSelect,
  onNew,
  onRename,
  onDelete,
}: SessionSidebarProps) {
  const t = useT();
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");

  return (
    <div className="flex h-full flex-col">
      <Button type="button" size="sm" className="mb-3" onClick={onNew}>
        <MessageSquarePlus className="size-4" />
        {t("playground.session.new")}
      </Button>
      <div className="flex-1 space-y-1 overflow-y-auto">
        {sessions.map((s) => {
          const title = s.title || t("playground.session.untitled");
          return (
            <div
              key={s.id}
              className={cn(
                "group/item flex cursor-pointer items-center gap-1 rounded-lg px-2.5 py-2 hover:bg-accent/50",
                s.id === activeId && "bg-accent",
              )}
              onClick={() => onSelect(s.id)}
            >
              {renamingId === s.id ? (
                <Input
                  value={draft}
                  autoFocus
                  className="h-7 flex-1"
                  onChange={(e) => setDraft(e.target.value)}
                  onClick={(e) => e.stopPropagation()}
                  onBlur={() => {
                    onRename(s.id, draft.trim());
                    setRenamingId(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      onRename(s.id, draft.trim());
                      setRenamingId(null);
                    }
                    if (e.key === "Escape") setRenamingId(null);
                  }}
                />
              ) : (
                <>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <span className="min-w-0 flex-1 truncate text-sm">{title}</span>
                      }
                    />
                    <TooltipContent className="max-w-xs break-all">{title}</TooltipContent>
                  </Tooltip>
                  <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover/item:opacity-100">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("playground.session.rename")}
                      onClick={(e) => {
                        e.stopPropagation();
                        setDraft(s.title);
                        setRenamingId(s.id);
                      }}
                    >
                      <Pencil className="size-3" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("playground.session.delete")}
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(s.id);
                      }}
                    >
                      <Trash2 className="size-3" />
                    </Button>
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 写 conversation-view.tsx**

自动滚底 + 用户上滚暂停跟随（距底 < 40px 视为跟随）：

```tsx
"use client";

import { useEffect, useRef } from "react";
import { ChatTurn } from "./chat-turn";
import { useT } from "@/lib/i18n";
import type { PlaygroundMessage } from "@/app/(dashboard)/playground/playground-logic";

interface ConversationViewProps {
  messages: PlaygroundMessage[];
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

export function ConversationView({
  messages,
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ConversationViewProps) {
  const t = useT();
  const scrollRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);

  // 有新内容且处于跟随态时滚到底
  useEffect(() => {
    const el = scrollRef.current;
    if (el && followRef.current) el.scrollTop = el.scrollHeight;
  }, [messages]);

  return (
    <div
      ref={scrollRef}
      className="flex-1 space-y-5 overflow-y-auto px-1 py-4"
      aria-live="polite"
      onScroll={() => {
        const el = scrollRef.current;
        if (!el) return;
        followRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      }}
    >
      {messages.length === 0 ? (
        <p className="pt-16 text-center text-sm text-muted-foreground">
          {t("playground.session.empty_hint")}
        </p>
      ) : (
        messages.map((m, i) => (
          <ChatTurn
            key={m.id}
            message={m}
            index={i}
            sending={sending && i === messages.length - 1}
            onEditSave={onEditSave}
            onDelete={onDelete}
            onRerun={onRerun}
            onRetry={onRetry}
          />
        ))
      )}
    </div>
  );
}
```

- [ ] **Step 3: Lint + 构建验证**

Run: `cd web && npm run lint && npm run build`
Expected: 0 error。重点自查：`truncate` 元素在 `TooltipTrigger` 子树内（lint error 级）。

- [ ] **Step 4: Commit**

```bash
git add web/src/components/playground/
git commit -m "feat(playground): 会话侧栏与对话流容器（自动滚底）"
```

---

### Task 7: 页面装配与运行链路（page.tsx 重写 + i18n 全量）

**Files:**
- Modify: `web/src/app/(dashboard)/playground/page.tsx`（整文件重写）
- Modify: `web/src/locales/en.json`、`web/src/locales/zh.json`、`web/src/locales/ja.json`

**Interfaces:**
- Consumes: 以上全部 Produces + 既有 `fetchAllPages` / `ownedKeys` / `selectableModels` / `buildChatBody` / `parseSSELine` / `api.playgroundChat(Stream)` / `PermissionGuard` / `useAuth` / `PageHeader` / `LocaleFade`（`@/components/locale-fade`）
- Produces: 页面本身（无后续任务依赖）

- [ ] **Step 1: 写 page.tsx 运行链路（核心状态）**

要点（完整文件按此逻辑组装，组件引用 Task 4–6 的导出）：

```tsx
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import { useAuth } from "@/lib/auth-context";
import type { APIKeyItem } from "@/lib/types";
import { LocaleFade } from "@/components/locale-fade";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { PanelLeft } from "lucide-react";
import { useT } from "@/lib/i18n";
import { toast } from "sonner";
import {
  buildChatBody,
  deriveTitle,
  estimateCost,
  fetchAllPages,
  loadParams,
  loadSessions,
  newMessage,
  newSession,
  ownedKeys,
  parseSSELine,
  saveParams,
  saveSessions,
  selectableModels,
  truncateAfter,
  upsertSession,
  type ChatUsage,
  type PlaygroundMessage,
  type PlaygroundModelOption,
  type PlaygroundParams,
  type PlaygroundSession,
  type PlaygroundTurnMeta,
} from "./playground-logic";
import { SessionSidebar } from "@/components/playground/session-sidebar";
import { ConversationView } from "@/components/playground/conversation-view";
import { Composer } from "@/components/playground/composer";
import { ParamsDrawer } from "@/components/playground/params-drawer";

const DEFAULT_PARAMS: PlaygroundParams = {
  model: "",
  apiKeyID: null,
  temperature: "",
  maxTokens: "",
  stream: true,
};

function PlaygroundPage() {
  const t = useT();
  const { user, isAdmin } = useAuth();
  const userId = user?.id;
  // 解析按用户租户隔离：管理员的模型列表含全部用户，只列本人名下的别名才能调通
  const ownerName = isAdmin() ? user?.name : undefined;

  const [sessions, setSessions] = useState<PlaygroundSession[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [params, setParams] = useState<PlaygroundParams>(DEFAULT_PARAMS);
  const [models, setModels] = useState<PlaygroundModelOption[]>([]);
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [keysLoaded, setKeysLoaded] = useState(false);
  const [sending, setSending] = useState(false);
  const [paramsOpen, setParamsOpen] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false); // < lg 会话抽屉态
  const abortRef = useRef<AbortController | null>(null);
  // 其余成员（初始化 effect、persist/patchActive、runGenerate、handlers）见下方各代码块，按序拼入本函数体
}
```

**初始化与模型/Key 拉取**（接在状态声明之后）：

```ts
  // 本地会话/参数初始化（一次）
  useEffect(() => {
    const loaded = loadSessions();
    const next = loaded.length > 0 ? loaded : [newSession()];
    setSessions(next);
    setActiveId(next[0].id);
    setParams({ ...DEFAULT_PARAMS, ...(loadParams() ?? {}) });
  }, []);

  // 模型与 Key 拉取（沿用分页拉全量 + 租户隔离；selectableModels 前带上 pricing）
  useEffect(() => {
    if (userId === undefined) return;
    let cancelled = false;
    (async () => {
      try {
        const [modelItems, keyItems] = await Promise.all([
          fetchAllPages(async (page, pageSize) => {
            const rsp = await api.listModelsPage({
              page,
              pageSize,
              status: "enabled",
              username: ownerName,
            });
            return { items: rsp.items ?? [], total: rsp.pageInfo?.total ?? 0 };
          }),
          fetchAllPages(async (page, pageSize) => {
            const rsp = await api.listAPIKeys(page, pageSize);
            return { items: rsp.keys ?? [], total: rsp.pageInfo?.total ?? 0 };
          }),
        ]);
        if (cancelled) return;
        setModels(
          selectableModels(
            modelItems.map((m) => ({ alias: m.alias, enabled: m.enabled, pricing: m.pricing })),
          ),
        );
        const mine = ownedKeys(keyItems, userId);
        setKeys(mine);
        setParams((prev) => ({ ...prev, apiKeyID: prev.apiKeyID ?? mine[0]?.id ?? null }));
      } catch (err) {
        if (!cancelled) {
          toast.error(t("playground.load_models_error"));
          console.error(err);
        }
      } finally {
        if (!cancelled) setKeysLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [t, userId, ownerName]);

  // 离开页面时中断进行中的流
  useEffect(() => () => abortRef.current?.abort(), []);
```

**持久化策略**：中间态（流式增量）不写 localStorage，**终态**（生成完成/中断/失败、编辑、删除、会话变更）统一 `saveSessions`：

```ts
  const persist = useCallback(
    (next: PlaygroundSession[]) => {
      if (!saveSessions(next)) toast.error(t("playground.session.save_failed"));
    },
    [t],
  );

  const patchActive = useCallback(
    (patch: (s: PlaygroundSession) => PlaygroundSession) => {
      setSessions((prev) => prev.map((s) => (s.id === activeId ? patch(s) : s)));
    },
    [activeId],
  );

  const patchParams = useCallback((patch: Partial<PlaygroundParams>) => {
    setParams((prev) => {
      const next = { ...prev, ...patch };
      saveParams(next);
      return next;
    });
  }, []);
```

**运行链路**（`runGenerate` 是生成内核：接收完整上下文、末尾插空 assistant 占位并流式填充；`handleSend` = 追加 user 消息后调用它）：

```ts
  /** 以给定上下文发起一次生成；终态统一写 meta 并持久化 */
  const runGenerate = useCallback(
    async (context: PlaygroundMessage[]) => {
      if (!params.model || params.apiKeyID === null) return;
      const pricing = models.find((m) => m.alias === params.model)?.pricing;
      const temp = params.temperature.trim() === "" ? undefined : Number(params.temperature);
      const maxTok = params.maxTokens.trim() === "" ? undefined : Number(params.maxTokens);
      const body = {
        ...buildChatBody(context, {
          model: params.model,
          stream: params.stream,
          temperature: temp,
          maxTokens: maxTok,
        }),
        stream_options: { include_usage: true },
      };

      const placeholder = newMessage("assistant", "");
      setSessions((prev) =>
        prev.map((s) => (s.id === activeId ? { ...s, messages: [...context, placeholder] } : s)),
      );
      setSending(true);
      const startAt = Date.now();
      let firstTokenMs: number | undefined;
      let usage: ChatUsage | undefined;
      let pending = "";
      let raf = 0;
      const flush = () => {
        raf = 0;
        const text = pending;
        patchActive((s) => ({
          ...s,
          messages: s.messages.map((m) => (m.id === placeholder.id ? { ...m, content: text } : m)),
        }));
      };
      const finalize = (extra: Partial<PlaygroundTurnMeta>) => {
        if (raf) cancelAnimationFrame(raf);
        const meta: PlaygroundTurnMeta = {
          model: params.model,
          usage,
          firstTokenMs,
          totalMs: Date.now() - startAt,
          cost: usage ? estimateCost(usage, pricing) : null,
          requestSnapshot: body,
          ...extra,
        };
        setSessions((prev) => {
          const next = prev.map((s) =>
            s.id === activeId
              ? {
                  ...s,
                  messages: s.messages.map((m) =>
                    m.id === placeholder.id ? { ...m, content: pending, meta } : m,
                  ),
                }
              : s,
          );
          persist(next);
          return next;
        });
        setSending(false);
        abortRef.current = null;
      };

      try {
        if (params.stream) {
          const controller = new AbortController();
          abortRef.current = controller;
          await api.playgroundChatStream(
            params.apiKeyID,
            body,
            (line) => {
              const ev = parseSSELine(line);
              if (ev.type === "delta") {
                if (firstTokenMs === undefined) firstTokenMs = Date.now() - startAt;
                pending += ev.text;
                if (!raf) raf = requestAnimationFrame(flush);
              } else if (ev.type === "usage") {
                usage = ev.usage;
              }
            },
            controller.signal,
          );
          finalize({});
        } else {
          const rsp = (await api.playgroundChat(params.apiKeyID, body)) as {
            choices?: { message?: { content?: string } }[];
            usage?: {
              prompt_tokens?: number;
              completion_tokens?: number;
              prompt_tokens_details?: { cached_tokens?: number };
            };
          };
          pending = rsp.choices?.[0]?.message?.content ?? "";
          if (rsp.usage) {
            usage = {
              promptTokens: rsp.usage.prompt_tokens ?? 0,
              completionTokens: rsp.usage.completion_tokens ?? 0,
              cachedTokens: rsp.usage.prompt_tokens_details?.cached_tokens ?? 0,
            };
          }
          finalize({});
        }
      } catch (err) {
        // 401 交给 api-client 全局刷新不展示；AbortError 标中断；其余内联错误卡
        const aborted = err instanceof DOMException && err.name === "AbortError";
        if (aborted) {
          finalize({ interrupted: true });
        } else if (err instanceof ApiError && err.status === 401) {
          finalize({});
        } else {
          finalize({
            error: {
              status: err instanceof ApiError ? err.status : undefined,
              message: err instanceof Error ? err.message : String(err),
            },
          });
        }
      }
    },
    [activeId, models, params, patchActive, persist],
  );
```

**消息与会话操作 handlers**（`handleRetry = handleRerun`，错误卡重试即移除该条重新生成）：

```ts
  const handleSend = useCallback(
    (content: string) => {
      if (!activeId || sending) return;
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const messages = [...session.messages, newMessage("user", content)];
      const title = session.title || deriveTitle(messages);
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages, title } : s));
      setSessions(next);
      persist(next);
      void runGenerate(messages);
    },
    [activeId, sending, sessions, persist, runGenerate],
  );

  const handleEditSave = useCallback(
    (index: number, content: string) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const edited = truncateAfter(session.messages, index).map((m, i) =>
        i === index ? { ...m, content, meta: undefined } : m,
      );
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages: edited, title: deriveTitle(edited) } : s));
      setSessions(next);
      persist(next);
      void runGenerate(edited); // 截断后的上下文自动重发（编辑 assistant = 续写）
    },
    [activeId, sessions, persist, runGenerate],
  );

  const handleRerun = useCallback(
    (index: number) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const kept = session.messages.filter((_, i) => i !== index); // 移除该条回复本身
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages: kept } : s));
      setSessions(next);
      persist(next);
      void runGenerate(kept);
    },
    [activeId, sessions, persist, runGenerate],
  );

  const handleDelete = useCallback(
    (index: number) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const kept = session.messages.filter((_, i) => i !== index);
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages: kept } : s));
      setSessions(next);
      persist(next);
    },
    [activeId, sessions, persist],
  );

  const handleRetry = handleRerun;

  const handleInsertRole = useCallback(
    (role: "system" | "assistant") => {
      patchActive((s) => ({ ...s, messages: [...s.messages, newMessage(role, "")] }));
    },
    [patchActive],
  );

  const handleNewSession = useCallback(() => {
    const s = newSession();
    setSessions((prev) => {
      const next = upsertSession(prev, s); // 插入置顶 + 超 MAX_SESSIONS 淘汰最旧
      persist(next);
      return next;
    });
    setActiveId(s.id);
    setSidebarOpen(false);
  }, [persist]);

  const handleSelectSession = useCallback((id: string) => {
    setActiveId(id);
    setSidebarOpen(false);
  }, []);

  const handleRenameSession = useCallback(
    (id: string, title: string) => {
      setSessions((prev) => {
        const next = prev.map((s) => (s.id === id ? { ...s, title } : s));
        persist(next);
        return next;
      });
    },
    [persist],
  );

  const handleDeleteSession = useCallback(
    (id: string) => {
      setSessions((prev) => {
        const rest = prev.filter((s) => s.id !== id);
        const next = rest.length > 0 ? rest : [newSession()];
        persist(next);
        return next;
      });
      if (activeId === id) setActiveId((prev) => prev); // 见下方说明
    },
    [activeId, persist],
  );
```

`handleDeleteSession` 中活动会话被删时的兜底：在 `setSessions` 的 updater 内无法安全调 `setActiveId`，改为渲染层兜底——`const active = sessions.find((s) => s.id === activeId) ?? sessions[0]`，组件全部使用 `active`（`active.id` 为空列表不可能发生，因删除时保底重建 `newSession()`）。

**布局**（JSX 骨架）：

```tsx
  const active = sessions.find((s) => s.id === activeId) ?? sessions[0];
  if (!active) return null;

  return (
    <LocaleFade>
      <div className="space-y-4">
        <PageHeader title={t("playground.title")} description={t("playground.description")} />
        <div className="flex h-[calc(100dvh-13rem)] gap-4">
          {/* 会话侧栏：lg+ 常驻；< lg 走 Sheet 抽屉 */}
          <aside className="hidden w-60 shrink-0 lg:block">
            <SessionSidebar
              sessions={sessions}
              activeId={active.id}
              onSelect={handleSelectSession}
              onNew={handleNewSession}
              onRename={handleRenameSession}
              onDelete={handleDeleteSession}
            />
          </aside>
          {/* 对话主区 */}
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex items-center gap-2 lg:hidden">
              <Button
                variant="outline"
                size="icon-sm"
                aria-label={t("playground.session.open")}
                onClick={() => setSidebarOpen(true)}
              >
                <PanelLeft className="size-4" />
              </Button>
            </div>
            <ConversationView
              messages={active.messages}
              sending={sending}
              onEditSave={handleEditSave}
              onDelete={handleDelete}
              onRerun={handleRerun}
              onRetry={handleRetry}
            />
            <Composer
              model={params.model}
              stream={params.stream}
              sending={sending}
              disabled={!params.model || params.apiKeyID === null}
              onSend={handleSend}
              onStop={() => abortRef.current?.abort()}
              onInsertRole={handleInsertRole}
              onOpenParams={() => setParamsOpen(true)}
            />
          </div>
        </div>
        <ParamsDrawer
          open={paramsOpen}
          onOpenChange={setParamsOpen}
          models={models}
          keys={keys}
          keysLoaded={keysLoaded}
          params={params}
          onParamsChange={patchParams}
        />
        {/* < lg 会话抽屉 */}
        <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
          <SheetContent side="left" className="w-60 sm:max-w-[15rem]">
            <SheetHeader>
              <SheetTitle>{t("playground.session.open")}</SheetTitle>
            </SheetHeader>
            <SessionSidebar
              sessions={sessions}
              activeId={active.id}
              onSelect={handleSelectSession}
              onNew={handleNewSession}
              onRename={handleRenameSession}
              onDelete={handleDeleteSession}
            />
          </SheetContent>
        </Sheet>
      </div>
    </LocaleFade>
  );
```

页面导出沿用既有守卫：

```tsx
export default function PlaygroundPageGuarded() {
  return (
    <PermissionGuard>
      <PlaygroundPage />
    </PermissionGuard>
  );
}
```

- [ ] **Step 2: 补全 i18n key（en / zh / ja 三份同步）**

`web/src/locales/en.json` 新增（追加到 `playground.*` 既有 key 附近）：

```json
"playground.params.title": "Run parameters",
"playground.params.open": "Run parameters",
"playground.composer.placeholder": "Send a message… Enter to send, Shift+Enter for newline",
"playground.composer.insert_system": "system",
"playground.composer.insert_assistant": "assistant",
"playground.composer.non_stream": "Non-stream",
"playground.session.new": "New chat",
"playground.session.rename": "Rename",
"playground.session.delete": "Delete chat",
"playground.session.untitled": "Untitled",
"playground.session.empty_hint": "Start a conversation — messages are saved locally only",
"playground.session.open": "Chats",
"playground.session.save_failed": "Failed to save chats locally (storage full?)",
"playground.turn.copy": "Copy",
"playground.turn.edit": "Edit",
"playground.turn.edit_save": "Save & regenerate",
"playground.turn.edit_cancel": "Cancel",
"playground.turn.edit_warning": "Following messages will be removed and regenerated",
"playground.turn.rerun": "Regenerate",
"playground.turn.delete": "Delete message",
"playground.metrics.interrupted": "interrupted",
"playground.metrics.tokens_in": "input",
"playground.metrics.tokens_out": "output",
"playground.metrics.tokens_cached": "cached",
"playground.metrics.first_token": "first token",
"playground.metrics.total_time": "total",
"playground.metrics.estimate_hint": "Cost is estimated from the model's default pricing tier; exact figures are in Audit.",
"playground.metrics.request_json": "Request JSON",
"playground.metrics.copy_curl": "Copy as curl",
"playground.error.retry": "Retry"
```

`zh.json` 对应：

```json
"playground.params.title": "运行参数",
"playground.params.open": "运行参数",
"playground.composer.placeholder": "输入消息，Enter 发送，Shift+Enter 换行…",
"playground.composer.insert_system": "system",
"playground.composer.insert_assistant": "assistant",
"playground.composer.non_stream": "非流式",
"playground.session.new": "新建对话",
"playground.session.rename": "重命名",
"playground.session.delete": "删除对话",
"playground.session.untitled": "未命名",
"playground.session.empty_hint": "开始调试对话——消息仅保存在本地",
"playground.session.open": "对话列表",
"playground.session.save_failed": "本地保存失败（存储空间已满？）",
"playground.turn.copy": "复制",
"playground.turn.edit": "编辑",
"playground.turn.edit_save": "保存并重跑",
"playground.turn.edit_cancel": "取消",
"playground.turn.edit_warning": "保存后其后的消息将被移除并重新生成",
"playground.turn.rerun": "重跑",
"playground.turn.delete": "删除消息",
"playground.metrics.interrupted": "已中断",
"playground.metrics.tokens_in": "输入",
"playground.metrics.tokens_out": "输出",
"playground.metrics.tokens_cached": "缓存",
"playground.metrics.first_token": "首字延迟",
"playground.metrics.total_time": "总耗时",
"playground.metrics.estimate_hint": "费用按模型默认定价档估算，精确值以审计为准。",
"playground.metrics.request_json": "请求体 JSON",
"playground.metrics.copy_curl": "复制为 curl",
"playground.error.retry": "重试"
```

`ja.json` 对应：

```json
"playground.params.title": "実行パラメータ",
"playground.params.open": "実行パラメータ",
"playground.composer.placeholder": "メッセージを入力… Enter で送信、Shift+Enter で改行",
"playground.composer.insert_system": "system",
"playground.composer.insert_assistant": "assistant",
"playground.composer.non_stream": "非ストリーム",
"playground.session.new": "新規チャット",
"playground.session.rename": "名前を変更",
"playground.session.delete": "チャットを削除",
"playground.session.untitled": "無題",
"playground.session.empty_hint": "デバッグ会話を始めましょう——メッセージはローカルのみ保存",
"playground.session.open": "チャット一覧",
"playground.session.save_failed": "ローカル保存に失敗しました（ストレージ満杯？）",
"playground.turn.copy": "コピー",
"playground.turn.edit": "編集",
"playground.turn.edit_save": "保存して再生成",
"playground.turn.edit_cancel": "キャンセル",
"playground.turn.edit_warning": "保存すると以降のメッセージは削除され再生成されます",
"playground.turn.rerun": "再実行",
"playground.turn.delete": "メッセージを削除",
"playground.metrics.interrupted": "中断済み",
"playground.metrics.tokens_in": "入力",
"playground.metrics.tokens_out": "出力",
"playground.metrics.tokens_cached": "キャッシュ",
"playground.metrics.first_token": "初トークン",
"playground.metrics.total_time": "合計時間",
"playground.metrics.estimate_hint": "費用はモデルのデフォルト価格帯による概算です。正確な値は監査を参照。",
"playground.metrics.request_json": "リクエスト JSON",
"playground.metrics.copy_curl": "curl としてコピー",
"playground.error.retry": "再試行"
```

同时**删除不再使用的旧 key**（`playground.request` / `playground.messages` / `playground.message.*` / `playground.output*` / `playground.sending`——以 lint 与全局搜索确认无引用后删，三份同步）。

- [ ] **Step 3: Lint + 测试 + 构建**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 全绿。lint 重点：i18n key 三份同步由人工 diff 自查（无自动检查）。

- [ ] **Step 4: Commit**

```bash
git add "web/src/app/(dashboard)/playground/page.tsx" web/src/locales/en.json web/src/locales/zh.json web/src/locales/ja.json web/src/components/playground/
git commit -m "feat(playground): 对话式工作台页面装配与运行链路"
```

---

### Task 8: 旧代码清理、术语沉淀与全量验证

**Files:**
- Modify: `web/src/app/(dashboard)/playground/page.tsx`（删除残留死代码）
- Modify: `web/CONTEXT.md`（补术语）
- Modify: 全局（若 `extractDeltaContent` 有残留引用）

**Interfaces:**
- Consumes: Task 1–7 全部产出
- Produces: 可合并的完整改动

- [ ] **Step 1: 死代码与引用清理**

Run: `cd web && grep -rn "extractDeltaContent\|playground.message.add\|playground.output" src/`
Expected: 0 命中（旧函数与旧 i18n key 已无引用；有则删除）。

`web/CONTEXT.md` 「Pricing & Cost」区后追加：

```markdown
## Playground（模型调试台）

**Playground Session（调试会话）**:
Playground 页的本地多会话单元（localStorage `playground.sessions.v1`，上限 50 淘汰最旧）。调试流量走后端 `SkipStore` 不沉淀会话数据集，会话仅存在浏览器本地。
_Avoid_: chat session, local chat

**Turn Meta（轮次元信息）**:
跟随 assistant 消息持久化的调试指标：usage（输入/输出/缓存 tokens）、首字延迟、总耗时、估算费用、请求体快照、中断/错误标记。指标行折叠展示。
_Avoid_: message metadata, debug info

**估算费用（Estimated Cost）**:
按模型 pricing 的「无条件默认规则（代表档）」估算的单轮费用：`(prompt−cached)×input + cached×cacheRead + completion×output`，单价每 1M tokens。不做时段/上下文区间匹配，精确值以审计为准。
_Avoid_: cost, billing amount
```

- [ ] **Step 2: 三件套全量验证**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 全绿（与 Task 7 相同命令，此处为清理后的最终回归）。

- [ ] **Step 3: ponytail-review 审查 diff**

对 `git diff master...HEAD` 跑 `ponytail-review` skill 逐行审过度工程；发现项修复后重跑 Step 2。

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore(playground): 术语沉淀与死代码清理"
```

- [ ] **Step 5: 汇报与合并询问**

向用户汇报任务完成情况与验证输出，并**询问**是否创建 PR / 合并 master（禁止擅自推送）。
