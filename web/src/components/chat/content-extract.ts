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
  map: Record<string, ToolResultInfo>,
  id: string,
): ToolResultInfo | undefined {
  if (id in map) return map[id];
  const normalized = normalizeToolCallID(id);
  for (const key of Object.keys(map)) {
    if (normalizeToolCallID(key) === normalized) return map[key];
  }
  return undefined;
}

export interface ToolResultInfo {
  text: string;
  rawContent?: string;
}

export function buildToolResultsByID(
  messages: MessageItem[],
): Record<string, ToolResultInfo> {
  const map: Record<string, ToolResultInfo> = {};
  for (const m of messages) {
    const id = m.message.tool_call_id;
    if (!id) continue;
    if (m.message.role !== "tool" && m.message.role !== "user") continue;
    const { text } = extractContent(m.message.content);
    map[id] = {
      text,
      rawContent: m.message.raw_content,
    };
  }
  return map;
}
