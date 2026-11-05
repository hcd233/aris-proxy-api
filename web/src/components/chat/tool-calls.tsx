"use client";

import { Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
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
import { MultimodalParts } from "./multimodal-parts";
import { LONG_OUTPUT_LINES, countLines, formatToolPayload, previewFirstArg } from "./tool-payload";

/**
 * 输入 / 输出合成一块面板，中间一道分隔线，不再是两个各自描边的代码框。
 * Moonshot 下用近黑代码底 + 9% 白发丝边 + 顶部 5% 白月光内高光，
 * 取代默认 Code Block 的 40% 白粗边与灰色填充；段标题降为次要色无衬线。
 * （shadow-xs 工具类在构建期烘焙了 anthropic 的值，不会随主题变量切换，故写字面量。）
 */
const TOOL_IO_PANEL =
  "flex min-w-0 flex-col overflow-hidden rounded-lg border border-input moonshot:border-border moonshot:bg-(--code-bg) moonshot:shadow-[inset_0_1px_0_0_rgb(255_255_255/0.05)] [&_[data-slot=code-block-title]]:font-sans [&_[data-slot=code-block-title]]:text-muted-foreground";

/** 段落本身不再描边、不铺底，交给外层面板 */
const TOOL_IO_SECTION = "rounded-none border-0 moonshot:bg-transparent";

interface ToolCallsProps {
  calls: UnifiedToolCall[];
  resultsByID: Record<string, ToolResultInfo>;
}

/**
 * 单行内容：图标 + 工具名 + 首参数预览。
 * 工具名不参与收缩（shrink-0），只在比整行还长时按 max-w 截断（扣掉图标 size-4 + gap-2）；
 * 剩余空间全部给预览截断。不依赖 flex-shrink 权重分配，各浏览器表现一致。
 */
function ToolLineBody({ call }: { call: UnifiedToolCall }) {
  const name = call.name || "tool";
  const preview = previewFirstArg(call.arguments);
  return (
    <>
      <TraceIcon>
        <Wrench />
      </TraceIcon>
      <TraceTitle className="max-w-[calc(100%-1.5rem)] shrink-0 font-mono">
        <TooltipRoot>
          <TooltipTrigger render={<span />}>{name}</TooltipTrigger>
          <TooltipContent side="top" className="max-w-xs break-all">
            {name}
          </TooltipContent>
        </TooltipRoot>
      </TraceTitle>
      {preview && (
        <TruncatedText
          render={<TraceDescription />}
          side="top"
          className="min-w-0 shrink font-mono text-xs"
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
        <div data-slot="tool-io" className={TOOL_IO_PANEL}>
          <HighlightedCode
            code={input.code}
            language={input.language}
            title={t("tool_call.input")}
            size="sm"
            wrap
            className="max-h-64"
            containerClassName={TOOL_IO_SECTION}
          />
          <Separator className="bg-input moonshot:bg-border" />
          {result.parts && (
            <div className={TOOL_IO_SECTION}>
              <MultimodalParts parts={result.parts} />
            </div>
          )}
          {output.code ? (
            <HighlightedCode
              code={output.code}
              language={output.language}
              title={t("tool_call.output")}
              size="sm"
              wrap
              className="max-h-64"
              containerClassName={TOOL_IO_SECTION}
            />
          ) : result.parts ? null : (
            <p className="px-2.5 py-2 text-xs text-muted-foreground">
              {t("tool_call.empty_output")}
            </p>
          )}
        </div>
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
