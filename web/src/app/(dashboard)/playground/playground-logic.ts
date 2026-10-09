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
