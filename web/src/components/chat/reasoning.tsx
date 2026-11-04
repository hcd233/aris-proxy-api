"use client";

import { useState } from "react";
import { Brain } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
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
import { formatCharCount } from "./reasoning-format";

interface ReasoningProps {
  text: string;
  /** 思考仍在流式输出：展开并以 running 态 shimmer；结束后自动收起 */
  streaming: boolean;
}

/**
 * shadcn AI Elements · Reasoning：思考内容挂在 Trace 上。
 * 展开态 `userOpen ?? streaming`——跟随流式自动展开/收起，用户手动切换后以用户为准。
 */
export function Reasoning({ text, streaming }: ReasoningProps) {
  const t = useT();
  const [userOpen, setUserOpen] = useState<boolean>();
  const content = text.trim();

  if (!content && !streaming) return null;

  const title = streaming ? t("playground.thinking.streaming") : t("chat.thought_process");
  const icon = <TraceIcon>{streaming ? <Spinner /> : <Brain />}</TraceIcon>;

  if (!content) {
    return (
      <Trace>
        <TraceLine status="running">
          {icon}
          <TraceTitle>{title}</TraceTitle>
        </TraceLine>
      </Trace>
    );
  }

  return (
    <Trace>
      <TraceCollapsible open={userOpen ?? streaming} onOpenChange={setUserOpen}>
        <TraceCollapsibleTrigger>
          <TraceLine status={streaming ? "running" : undefined}>
            {icon}
            <TraceTitle>{title}</TraceTitle>
            <TraceDescription>
              {t("chat.reasoning_chars").replace("{count}", formatCharCount(content.length))}
            </TraceDescription>
          </TraceLine>
        </TraceCollapsibleTrigger>
        <TraceCollapsibleContent className="text-[13.5px] whitespace-pre-wrap italic">
          {content}
        </TraceCollapsibleContent>
      </TraceCollapsible>
    </Trace>
  );
}
