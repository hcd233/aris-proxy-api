import type { MessageItem } from "@/lib/types";

/** 工具结果消息（role=tool，或带 tool_call_id 的 user）：内联进对应工具调用行，不单独渲染 */
export function isToolResultMessage(m: MessageItem): boolean {
  const { role, tool_call_id } = m.message;
  return role === "tool" || (role === "user" && !!tool_call_id);
}

export type TranscriptEntry =
  | { kind: "message"; message: MessageItem; index: number }
  | {
      kind: "steps";
      /** 组内首条消息 id */
      id: number;
      messages: MessageItem[];
      callCount: number;
      /** 去重保序的工具名 */
      toolNames: string[];
    };

/** 折叠为 Chain of Thought 的最少工具调用数（只有一次调用时折叠没有意义） */
const MIN_STEP_CALLS = 2;

function hasToolCalls(m: MessageItem): boolean {
  return m.message.role === "assistant" && (m.message.tool_calls?.length ?? 0) > 0;
}

/**
 * 会话转录分组：连续的「带 tool_calls 的 assistant 消息」组成步骤段（中间的工具结果消息
 * 被跳过、不打断），段内调用数 ≥ MIN_STEP_CALLS 时折叠为一个 steps 条目；其余逐条平铺。
 * `index` 保留原数组下标（入场动画 delay 用）。
 */
export function groupTranscript(messages: MessageItem[]): TranscriptEntry[] {
  const out: TranscriptEntry[] = [];
  let run: { message: MessageItem; index: number }[] = [];

  const flush = () => {
    const callCount = run.reduce((n, r) => n + (r.message.message.tool_calls?.length ?? 0), 0);
    if (callCount >= MIN_STEP_CALLS) {
      const names = run.flatMap((r) => (r.message.message.tool_calls ?? []).map((c) => c.name));
      out.push({
        kind: "steps",
        id: run[0].message.id,
        messages: run.map((r) => r.message),
        callCount,
        toolNames: [...new Set(names)],
      });
    } else {
      for (const r of run) out.push({ kind: "message", ...r });
    }
    run = [];
  };

  messages.forEach((message, index) => {
    if (isToolResultMessage(message)) return;
    if (hasToolCalls(message)) {
      run.push({ message, index });
      return;
    }
    flush();
    out.push({ kind: "message", message, index });
  });
  flush();

  return out;
}
