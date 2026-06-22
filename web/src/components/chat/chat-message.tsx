"use client";

/**
 * Chat message entry point — role dispatch + re-exports.
 *
 * Rendering is delegated to per-role components:
 *  - user-message.tsx      (warm rounded bubble, no text label)
 *  - assistant-message.tsx (avatar + time below, no text label)
 *  - system-message.tsx    (retains "System" label, no avatar)
 *
 * Tool-result messages (role="tool" or role="user" with tool_call_id)
 * are skipped here — they're inlined into the matched ToolCallCard.
 */

import type { MessageItem } from "@/lib/types";
import { extractContent, type ToolResultInfo } from "./content-extract";
import { UserMessage } from "./user-message";
import { AssistantMessage } from "./assistant-message";
import { SystemMessage } from "./system-message";

export { buildToolResultsByID } from "./content-extract";

interface ChatMessageProps {
  message: MessageItem;
  index: number;
  toolResultsByID: Record<string, ToolResultInfo>;
}

export function ChatMessage({
  message,
  index,
  toolResultsByID,
}: ChatMessageProps) {
  const { role } = message.message;

  const isToolResult =
    role === "tool" ||
    (role === "user" && !!message.message.tool_call_id);
  if (isToolResult) return null;

  if (role === "user") {
    return <UserMessage message={message} index={index} />;
  }

  if (role === "system") {
    const { text } = extractContent(message.message.content);
    const time = message.createdAt
      ? new Date(message.createdAt).toLocaleTimeString([], {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })
      : undefined;
    return <SystemMessage text={text} time={time} index={index} />;
  }

  return (
    <AssistantMessage
      message={message}
      index={index}
      toolResultsByID={toolResultsByID}
    />
  );
}
