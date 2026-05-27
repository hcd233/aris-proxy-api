"use client";

/**
 * Chat message rendering, modelled after the claude.ai conversation view.
 *
 * Layout decisions:
 *  - Single centred reading column (~720px), not bubble pairs.
 *  - User messages: subtle warm card on the right side of the column.
 *  - Assistant messages: bubble-less prose with a small Claude-orange dot.
 *  - Reasoning (thinking): collapsible muted block, italic.
 *  - Tool calls: compact card under the assistant message; matched tool
 *    results (next message with role=tool/tool_call_id) are inlined as the
 *    "Output" section so the user can see the full request/response together.
 *  - Multimodal parts (image/audio/file/refusal) get dedicated renderers.
 */

import { Fragment, useState } from "react";
import {
  ArrowDownToLine,
  Brain,
  ChevronDown,
  ChevronRight,
  FileText,
  Music2,
  ShieldAlert,
  Sparkles,
  Wrench,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type {
  MessageItem,
  UnifiedToolCall,
} from "@/lib/types";

import { MarkdownLite } from "./markdown-lite";

// ─── Helpers: extract text + multimodal parts ────────────────────────────────

interface ContentPart {
  type?: string;
  text?: string;
  image_url?: string | { url?: string; detail?: string };
  input_audio?: { data?: string; format?: string };
  file?: { filename?: string; file_id?: string; file_data?: string };
  refusal?: string;
  // Anthropic / unified shapes
  audio_data?: string;
  audio_format?: string;
  file_data?: string;
  file_id?: string;
  filename?: string;
  image_detail?: string;
}

interface ExtractedContent {
  text: string;
  parts: ContentPart[];
}

function extractContent(content: unknown): ExtractedContent {
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

function imageURLOf(part: ContentPart): string | undefined {
  if (typeof part.image_url === "string") return part.image_url;
  if (part.image_url && typeof part.image_url === "object") {
    return part.image_url.url;
  }
  return undefined;
}

// ─── Multimodal part renderers ───────────────────────────────────────────────

function PartImage({ url }: { url: string }) {
  return (
    <div className="my-2 inline-block max-w-sm overflow-hidden rounded-lg border border-border/60 bg-muted/40">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={url} alt="" className="block h-auto max-h-80 w-full object-contain" />
    </div>
  );
}

function PartIconCard({
  icon,
  label,
  meta,
}: {
  icon: React.ReactNode;
  label: string;
  meta?: string;
}) {
  return (
    <div className="my-2 inline-flex items-center gap-2.5 rounded-lg border border-border/60 bg-muted/40 px-3 py-2">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground">
        {icon}
      </div>
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-foreground">{label}</p>
        {meta && <p className="truncate text-[11px] text-muted-foreground">{meta}</p>}
      </div>
    </div>
  );
}

function PartRefusal({ text }: { text: string }) {
  return (
    <div className="my-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
      <ShieldAlert className="mt-0.5 size-4 shrink-0" />
      <span className="whitespace-pre-wrap break-words">{text}</span>
    </div>
  );
}

function MultimodalParts({ parts }: { parts: ContentPart[] }) {
  if (parts.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {parts.map((part, i) => {
        switch (part.type) {
          case "image_url": {
            const url = imageURLOf(part);
            return url ? <PartImage key={i} url={url} /> : null;
          }
          case "input_audio": {
            const fmt = part.input_audio?.format ?? part.audio_format ?? "audio";
            return (
              <PartIconCard
                key={i}
                icon={<Music2 className="size-4" />}
                label="Audio attachment"
                meta={String(fmt).toUpperCase()}
              />
            );
          }
          case "file": {
            const filename = part.file?.filename ?? part.filename ?? "file";
            const fileID = part.file?.file_id ?? part.file_id;
            return (
              <PartIconCard
                key={i}
                icon={<FileText className="size-4" />}
                label={filename}
                meta={fileID ? `id: ${fileID}` : undefined}
              />
            );
          }
          case "refusal":
            return part.refusal || part.text ? (
              <PartRefusal key={i} text={(part.refusal ?? part.text) as string} />
            ) : null;
          default:
            return null;
        }
      })}
    </div>
  );
}

// ─── Reasoning (thinking) block ──────────────────────────────────────────────

function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  if (!text.trim()) return null;

  return (
    <div className="mb-3">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="group/thinking inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[12px] text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
      >
        <Brain className="size-3.5 text-primary/70" />
        <span className="font-medium tracking-wide">Thought process</span>
        {open ? (
          <ChevronDown className="size-3 opacity-60" />
        ) : (
          <ChevronRight className="size-3 opacity-60" />
        )}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border-l-2 border-primary/30 bg-muted/30 px-4 py-3">
          <p className="whitespace-pre-wrap break-words text-[13.5px] leading-relaxed text-muted-foreground italic">
            {text}
          </p>
        </div>
      )}
    </div>
  );
}

// ─── Tool call card (with optional matched result) ──────────────────────────

function prettyJSON(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

interface ToolCallCardProps {
  call: UnifiedToolCall;
  /** Tool result text matched by tool_call_id, if any. */
  result?: string;
}

function ToolCallCard({ call, result }: ToolCallCardProps) {
  const [open, setOpen] = useState(false);
  const args = prettyJSON(call.arguments);
  const out = result ? prettyJSON(result) : undefined;

  return (
    <div
      className={cn(
        "mt-3 overflow-hidden rounded-xl border border-primary/25 bg-primary/[0.04]",
        "shadow-[0_1px_0_rgba(217,119,87,0.05)]",
      )}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left transition-colors hover:bg-primary/[0.07]"
      >
        <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/15 text-primary">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-[13px] font-medium text-foreground">
              {call.name || "tool"}
            </span>
            {result !== undefined && (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-1.5 py-[1px] font-mono text-[10px] text-emerald-700 dark:text-emerald-400">
                <ArrowDownToLine className="size-2.5" />
                result
              </span>
            )}
          </div>
          {call.id && (
            <span className="font-mono text-[10px] text-muted-foreground/60">
              {call.id}
            </span>
          )}
        </div>
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {open && (
        <div className="border-t border-primary/15 bg-background/40">
          <div className="px-3.5 py-3">
            <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Input
            </p>
            <pre className="overflow-x-auto rounded-md bg-muted/60 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90">
              {args || "{}"}
            </pre>
          </div>
          {out !== undefined && (
            <div className="border-t border-primary/15 px-3.5 py-3">
              <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                Output
              </p>
              <pre className="overflow-x-auto rounded-md bg-muted/60 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90">
                {out}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Message renderers (per role) ────────────────────────────────────────────

function Avatar({ kind }: { kind: "user" | "assistant" | "system" }) {
  if (kind === "user") return null; // user messages use card background, no avatar
  if (kind === "system") {
    return (
      <div className="mt-1 flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
        <ShieldAlert className="size-3.5" />
      </div>
    );
  }
  // assistant
  return (
    <div className="mt-1 flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/15 text-primary">
      <Sparkles className="size-3.5" />
    </div>
  );
}

function MetaLine({
  label,
  model,
  time,
}: {
  label: string;
  model?: string;
  time?: string;
}) {
  return (
    <div className="mb-1.5 flex items-center gap-2 text-[11px] text-muted-foreground/70">
      <span className="font-medium uppercase tracking-[0.14em]">{label}</span>
      {model && (
        <>
          <span className="text-muted-foreground/30">·</span>
          <span className="font-mono">{model}</span>
        </>
      )}
      {time && (
        <>
          <span className="text-muted-foreground/30">·</span>
          <span>{time}</span>
        </>
      )}
    </div>
  );
}

interface RenderedMessageProps {
  message: MessageItem;
  /** map: tool_call_id -> result text, harvested from sibling tool messages */
  toolResultsByID: Record<string, string>;
  index: number;
}

export function ChatMessage({
  message,
  toolResultsByID,
  index,
}: RenderedMessageProps) {
  const { role, content, tool_calls, reasoning_content, refusal } =
    message.message;
  const { text, parts } = extractContent(content);
  const time = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : undefined;

  // Stagger fade-in
  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  // Tool results may arrive either as role="tool" (Anthropic / unified) or as
  // role="user" with a tool_call_id (OpenAI chat completion convention).
  // Both cases are inlined into the matched tool-call card above, so skip them here.
  const isToolResult =
    role === "tool" ||
    (role === "user" && !!message.message.tool_call_id);
  if (isToolResult) return null;

  if (role === "user") {
    return (
      <div
        style={style}
        className="animate-in fade-in slide-in-from-bottom-1 flex justify-end duration-300"
      >
        <div className="w-full max-w-[88%]">
          <MetaLine label="You" time={time} />
          <div className="rounded-2xl rounded-tr-md border border-border/60 bg-secondary/70 px-5 py-3.5">
            {parts.length > 0 && <MultimodalParts parts={parts} />}
            {text && <MarkdownLite text={text} />}
            {!text && parts.length === 0 && (
              <span className="text-muted-foreground/60">—</span>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (role === "system") {
    return (
      <div
        style={style}
        className="animate-in fade-in slide-in-from-bottom-1 duration-300"
      >
        <MetaLine label="System" time={time} />
        <div className="rounded-xl border border-dashed border-border bg-muted/40 px-4 py-3 text-[13.5px] leading-relaxed text-muted-foreground">
          <MarkdownLite text={text} />
        </div>
      </div>
    );
  }

  // assistant (and any other unhandled role via fallback)
  const isAssistant = role === "assistant";
  return (
    <div
      style={style}
      className="animate-in fade-in slide-in-from-bottom-1 flex gap-3 duration-300"
    >
      <Avatar kind={isAssistant ? "assistant" : "system"} />
      <div className="min-w-0 flex-1">
        <MetaLine
          label={isAssistant ? "AI" : role}
          model={message.model}
          time={time}
        />

        {reasoning_content && <ReasoningBlock text={reasoning_content} />}

        <div className="text-foreground">
          {parts.length > 0 && <MultimodalParts parts={parts} />}
          {text && <MarkdownLite text={text} />}
          {!text && parts.length === 0 && !tool_calls?.length && !refusal && (
            <span className="text-muted-foreground/60">—</span>
          )}
        </div>

        {refusal && <PartRefusal text={refusal} />}

        {tool_calls && tool_calls.length > 0 && (
          <div>
            {tool_calls.map((call, i) => (
              <Fragment key={call.id ?? i}>
                <ToolCallCard
                  call={call}
                  result={call.id ? toolResultsByID[call.id] : undefined}
                />
              </Fragment>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Build tool-result map from a flat message list ──────────────────────────

/**
 * Tool-result messages may appear in two shapes depending on the upstream
 * provider:
 *   1. Anthropic / unified: role="tool" with tool_call_id
 *   2. OpenAI chat completion: role="user" with tool_call_id
 *
 * Both are matched here — the presence of tool_call_id is the authoritative
 * signal, regardless of role.
 */
export function buildToolResultsByID(
  messages: MessageItem[],
): Record<string, string> {
  const map: Record<string, string> = {};
  for (const m of messages) {
    const id = m.message.tool_call_id;
    if (!id) continue;
    if (m.message.role !== "tool" && m.message.role !== "user") continue;
    const { text } = extractContent(m.message.content);
    map[id] = text;
  }
  return map;
}
