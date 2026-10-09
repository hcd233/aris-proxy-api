/**
 * Playground 调试页的纯逻辑（无 React；运行时无项目内 import——type import 会被
 * 编译期擦除，因此 vitest 无需路径别名即可直接加载测）。
 */
import type { PricingDTO } from "@/lib/types";

/** 单轮调试元信息（跟随其所属消息持久化） */
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

/** 调试参数（UI 状态，跨会话记忆） */
export interface PlaygroundParams {
  model: string;
  apiKeyID: number | null;
  temperature: string;
  maxTokens: string;
  stream: boolean;
}

/** 组装请求体的数值参数 */
export interface BuildChatParams {
  model: string;
  stream: boolean;
  temperature?: number;
  maxTokens?: number;
}

/** 模型列表项（只需要别名、启用态与定价） */
export interface PlaygroundModelOption {
  alias: string;
  enabled: boolean;
  pricing?: PricingDTO;
}

/** 组装 OpenAI Chat 请求体（空消息行剔除；可选参数仅在有效时携带） */
export function buildChatBody(
  messages: Pick<PlaygroundMessage, "role" | "content">[],
  params: BuildChatParams,
): Record<string, unknown> {
  const kept = messages
    .filter((m) => m.content.trim() !== "")
    .map((m) => ({ role: m.role, content: m.content }));
  const body: Record<string, unknown> = {
    model: params.model,
    messages: kept,
    stream: params.stream,
  };
  if (params.temperature !== undefined && Number.isFinite(params.temperature)) {
    body.temperature = params.temperature;
  }
  if (params.maxTokens !== undefined && params.maxTokens > 0) {
    body.max_tokens = params.maxTokens;
  }
  return body;
}

/** 可选模型：仅 enabled 项，按 alias 去重 */
export function selectableModels(items: PlaygroundModelOption[]): PlaygroundModelOption[] {
  const seen = new Set<string>();
  const out: PlaygroundModelOption[] = [];
  for (const m of items) {
    if (!m.enabled || seen.has(m.alias)) continue;
    seen.add(m.alias);
    out.push(m);
  }
  return out;
}

/** 后端分页上限（PageParam.pageSize maximum） */
export const MAX_PAGE_SIZE = 500;

/** 拉取全部分页（下拉选项不允许静默截断）；某页不足 pageSize 或累计达到 total 即停止 */
export async function fetchAllPages<T>(
  fetchPage: (page: number, pageSize: number) => Promise<{ items: T[]; total: number }>,
  pageSize: number = MAX_PAGE_SIZE,
): Promise<T[]> {
  const all: T[] = [];
  for (let page = 1; ; page++) {
    const { items, total } = await fetchPage(page, pageSize);
    all.push(...items);
    if (items.length < pageSize || all.length >= total) return all;
  }
}

/** API Key 选项（只需要 ID、名称与归属用户） */
export interface PlaygroundKeyOption {
  id: number;
  name: string;
  user?: { id: number };
}

/**
 * 当前用户名下的 Key：Playground 调用按所选 Key 归属审计，后端只接受本人的 Key。
 * 管理员的 Key 列表包含全部用户，须按归属过滤；未带 user 的条目即本人列表（非管理员视角）。
 */
export function ownedKeys<T extends PlaygroundKeyOption>(keys: T[], userId: number): T[] {
  return keys.filter((k) => !k.user || k.user.id === userId);
}

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

/**
 * 按模型定价的「无条件默认规则」（代表档）估算费用（展示单位）。
 * 口径：(prompt − cached)×input + cached×cache_read + completion×output，单价为每 1M tokens。
 * 不做时段/上下文区间匹配（防与审计口径漂移）；未计价或缺默认规则返回 null。
 */
export function estimateCost(usage: ChatUsage, pricing?: PricingDTO): number | null {
  const rule = pricing?.rules?.find(
    (r) => (!r.time_windows || r.time_windows.length === 0) && !r.context_min && !r.context_max,
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
  const parsed = readJSON<{ version?: number; sessions?: PlaygroundSession[] }>(
    SESSIONS_STORAGE_KEY,
  );
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

/** 更新或插入会话并置顶（updatedAt=now），返回按 updatedAt 降序，超上限淘汰最旧 */
export function upsertSession(
  sessions: PlaygroundSession[],
  session: PlaygroundSession,
): PlaygroundSession[] {
  const rest = sessions.filter((s) => s.id !== session.id);
  return [{ ...session, updatedAt: Date.now() }, ...rest]
    .sort((a, b) => b.updatedAt - a.updatedAt)
    .slice(0, MAX_SESSIONS);
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
  return {
    id: crypto.randomUUID(),
    title: "",
    createdAt: Date.now(),
    updatedAt: Date.now(),
    messages: [],
  };
}

export function newMessage(role: PlaygroundMessage["role"], content: string): PlaygroundMessage {
  return { id: crypto.randomUUID(), role, content };
}

/** 保留 [0..index] 区间的消息（编辑保存后截断其后文用） */
export function truncateAfter(messages: PlaygroundMessage[], index: number): PlaygroundMessage[] {
  return messages.slice(0, index + 1);
}
