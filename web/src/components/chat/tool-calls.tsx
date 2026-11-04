"use client";

import { Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { TruncatedText } from "@/components/truncated-text";
import {
  Trace,
  TraceCollapsible,
  TraceCollapsibleContent,
  TraceCollapsibleTrigger,
  TraceDescription,
  TraceIcon,
  TraceLine,
  TraceTitle,
} from "@/components/ui/trace";
import { useT } from "@/lib/i18n";
import type { UnifiedToolCall } from "@/lib/types";
import { lookupToolResult, type ToolResultInfo } from "./content-extract";
import { HighlightedCode } from "./highlighted-code";
import { LONG_OUTPUT_LINES, countLines, formatToolPayload, previewFirstArg } from "./tool-payload";

interface ToolCallsProps {
  calls: UnifiedToolCall[];
  resultsByID: Record<string, ToolResultInfo>;
}

/** 单行内容：图标 + 工具名 + 首参数预览（两者截断均可悬停看全） */
function ToolLineBody({ call }: { call: UnifiedToolCall }) {
  const name = call.name || "tool";
  const preview = previewFirstArg(call.arguments);
  return (
    <>
      <TraceIcon>
        <Wrench />
      </TraceIcon>
      <TraceTitle className="font-mono">
        <TooltipRoot>
          <TooltipTrigger render={<span />}>{name}</TooltipTrigger>
          <TooltipContent side="top" className="max-w-xs break-all">
            {name}
          </TooltipContent>
        </TooltipRoot>
      </TraceTitle>
      {preview && (
        // shrink 权重远大于标题：空间不足时先截断预览，工具名保持完整
        <TruncatedText
          render={<TraceDescription />}
          side="top"
          className="min-w-0 shrink-[1000] font-mono text-xs"
        >
          {preview}
        </TruncatedText>
      )}
    </>
  );
}

function ToolCallRow({ call, result }: { call: UnifiedToolCall; result?: ToolResultInfo }) {
  const t = useT();

  // 无结果（调用 id 缺失或结果消息未加载到）：只有一行，不可展开
  if (!result) {
    return (
      <TraceLine status="pending">
        <ToolLineBody call={call} />
      </TraceLine>
    );
  }

  const input = formatToolPayload(call.arguments || "{}");
  const output = formatToolPayload(result.text);
  const long = countLines(output.code) > LONG_OUTPUT_LINES;

  return (
    <TraceCollapsible>
      <TraceCollapsibleTrigger>
        <TraceLine>
          <ToolLineBody call={call} />
        </TraceLine>
      </TraceCollapsibleTrigger>
      <TraceCollapsibleContent className="flex flex-col gap-2 text-foreground">
        <HighlightedCode
          code={input.code}
          language={input.language}
          title={t("tool_call.input")}
          size="sm"
          className="max-h-64"
        />
        {output.code ? (
          <HighlightedCode
            code={output.code}
            language={output.language}
            title={t("tool_call.output")}
            size="sm"
            className="max-h-64"
          />
        ) : (
          <p className="text-xs text-muted-foreground">{t("tool_call.empty_output")}</p>
        )}
        {long && (
          <Sheet>
            <SheetTrigger
              render={<Button variant="link" size="xs" className="h-auto w-fit min-w-0 px-0" />}
            >
              {t("tool_call.view_full")}
            </SheetTrigger>
            <SheetContent side="right" className="sm:max-w-2xl">
              <SheetHeader>
                <SheetTitle className="font-mono">{call.name || "tool"}</SheetTitle>
              </SheetHeader>
              <div className="flex min-h-0 flex-1 flex-col px-4 pb-4">
                <HighlightedCode
                  code={output.code}
                  language={output.language}
                  title={t("tool_call.output")}
                  size="sm"
                  lineNumbers
                  containerClassName="min-h-0 flex-1"
                />
              </div>
            </SheetContent>
          </Sheet>
        )}
        {call.id && (
          <span className="font-mono text-[11px] text-muted-foreground/70">{call.id}</span>
        )}
      </TraceCollapsibleContent>
    </TraceCollapsible>
  );
}

/** shadcn AI Elements · Tool：一条 assistant 消息的全部工具调用放进同一个 Trace，一调用一行 */
export function ToolCalls({ calls, resultsByID }: ToolCallsProps) {
  if (calls.length === 0) return null;
  return (
    <Trace>
      {calls.map((call, i) => (
        <ToolCallRow
          key={call.id ?? i}
          call={call}
          result={call.id ? lookupToolResult(resultsByID, call.id) : undefined}
        />
      ))}
    </Trace>
  );
}
