/**
 * Playground 调试页的纯逻辑（无 React；运行时无项目内 import——type import 会被
 * 编译期擦除，因此 vitest 无需路径别名即可直接加载测）。
 */
import type { ModelCapability } from "@/lib/types";

/** 调试消息（多轮编辑器的行） */
export interface PlaygroundMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

/** 调试参数 */
export interface PlaygroundParams {
  model: string;
  stream: boolean;
  temperature?: number;
  maxTokens?: number;
}

/** 模型列表项（只需要别名与启用态） */
export interface PlaygroundModelOption {
  alias: string;
  enabled: boolean;
  capabilities?: ModelCapability[];
}

/** 组装 OpenAI Chat 请求体（空消息行剔除；可选参数仅在有效时携带） */
export function buildChatBody(
  messages: PlaygroundMessage[],
  params: PlaygroundParams,
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

/**
 * 从 SSE 增量文本中提取 OpenAI chat chunk 的增量内容。
 * 返回空串表示该行不是内容增量（如 [DONE]、role 帧、空行）。
 */
export function extractDeltaContent(sseLine: string): string {
  const line = sseLine.trim();
  if (!line.startsWith("data:")) return "";
  const payload = line.slice("data:".length).trim();
  if (payload === "" || payload === "[DONE]") return "";
  try {
    const chunk = JSON.parse(payload) as {
      choices?: { delta?: { content?: string } }[];
    };
    return chunk.choices?.[0]?.delta?.content ?? "";
  } catch {
    return "";
  }
}
