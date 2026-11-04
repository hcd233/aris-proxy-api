"use client";

import { ListTree } from "lucide-react";
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
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";
import { AssistantMessage } from "./assistant-message";
import type { ToolResultInfo } from "./content-extract";
import type { TranscriptEntry } from "./transcript-groups";

/** 摘要描述里最多列出的工具名个数 */
const MAX_LISTED_TOOLS = 3;

interface ChainOfThoughtProps {
  entry: Extract<TranscriptEntry, { kind: "steps" }>;
  toolResultsByID: Record<string, ToolResultInfo>;
}

/**
 * shadcn AI Elements · Chain of Thought：连续的工具步骤折叠成一行摘要，
 * 展开后按原顺序呈现每一步（含其文本、思考与工具调用）。
 */
export function ChainOfThought({ entry, toolResultsByID }: ChainOfThoughtProps) {
  const { t, locale } = useI18n();
  const sep = locale === "en" ? ", " : "、";
  const listed = entry.toolNames.slice(0, MAX_LISTED_TOOLS).join(sep);
  const description =
    entry.toolNames.length > MAX_LISTED_TOOLS ? `${listed}${t("chat.steps_more")}` : listed;
  const title = t("chat.steps_title").replace("{count}", String(entry.callCount));

  return (
    <Trace>
      <TraceCollapsible>
        <TraceCollapsibleTrigger>
          <TraceLine>
            <TraceIcon>
              <ListTree />
            </TraceIcon>
            <TooltipRoot>
              <TooltipTrigger render={<TraceTitle>{title}</TraceTitle>} />
              <TooltipContent side="top">{title}</TooltipContent>
            </TooltipRoot>
            {description && (
              <TooltipRoot>
                <TooltipTrigger
                  render={
                    <TraceDescription className="min-w-0 shrink truncate">
                      {description}
                    </TraceDescription>
                  }
                />
                <TooltipContent side="top" className="max-w-xs break-all">
                  {entry.toolNames.join(sep)}
                </TooltipContent>
              </TooltipRoot>
            )}
          </TraceLine>
        </TraceCollapsibleTrigger>
        <TraceCollapsibleContent keepMounted className="flex flex-col gap-5 pt-1 text-foreground">
          {entry.messages.map((message, i) => (
            <AssistantMessage
              key={message.id}
              message={message}
              index={i}
              toolResultsByID={toolResultsByID}
            />
          ))}
        </TraceCollapsibleContent>
      </TraceCollapsible>
    </Trace>
  );
}
