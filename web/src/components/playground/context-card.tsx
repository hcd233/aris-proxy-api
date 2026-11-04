"use client";

import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import { Progress } from "@/components/ui/progress";
import { Separator } from "@/components/ui/separator";
import { copyTextToClipboard } from "@/lib/clipboard";
import { formatCost } from "@/lib/money";
import { useT } from "@/lib/i18n";
import { formatDurationMs, formatTokenCount } from "@/lib/utils";
import {
  buildCurl,
  contextSummary,
  type PlaygroundTurnMeta,
} from "@/app/(dashboard)/playground/playground-logic";

interface ContextCardProps {
  meta: PlaygroundTurnMeta;
  /** 该轮所用模型的上下文窗口（tokens），<=0 表示未知 */
  contextLength: number;
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium tabular-nums">{value}</span>
    </div>
  );
}

/** shadcn AI Elements · Context：消息底部一行摘要，悬停展开上下文占用、明细与请求复制 */
export function ContextCard({ meta, contextLength }: ContextCardProps) {
  const t = useT();
  const { usage } = meta;
  const { used, percent, percentLabel } = contextSummary(usage, contextLength);

  const head =
    used === null
      ? "—"
      : percentLabel
        ? `${percentLabel} ${t("playground.context")}`
        : `${formatTokenCount(used)} tok`;
  const trigger = [head, formatDurationMs(meta.totalMs), formatCost(meta.cost)];
  if (meta.interrupted) trigger.push(t("playground.metrics.interrupted"));

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={10}
        closeDelay={100}
        render={
          <Button
            variant="ghost"
            size="xs"
            className="h-6 min-w-0 px-1.5 text-[11px] font-normal text-muted-foreground tabular-nums"
          />
        }
      >
        {trigger.join(" · ")}
      </HoverCardTrigger>
      <HoverCardContent align="start" className="flex w-72 flex-col gap-3 text-xs">
        <div className="flex items-center justify-between gap-3">
          <span className="font-medium">{t("playground.context.title")}</span>
          <span className="text-muted-foreground tabular-nums">
            {formatTokenCount(used)}
            {contextLength > 0 && ` / ${formatTokenCount(contextLength)}`}
          </span>
        </div>
        {percent !== null && (
          <Progress value={Math.min(percent, 100)} aria-label={t("playground.context.used")} />
        )}
        <div className="flex flex-col gap-1">
          <Row
            label={t("playground.metrics.tokens_in")}
            value={formatTokenCount(usage?.promptTokens)}
          />
          <Row
            label={t("playground.metrics.tokens_cached")}
            value={formatTokenCount(usage?.cachedTokens)}
          />
          <Row
            label={t("playground.metrics.tokens_out")}
            value={formatTokenCount(usage?.completionTokens)}
          />
        </div>
        <Separator />
        <div className="flex flex-col gap-1">
          <Row
            label={t("playground.metrics.first_token")}
            value={formatDurationMs(meta.firstTokenMs)}
          />
          <Row label={t("playground.metrics.total_time")} value={formatDurationMs(meta.totalMs)} />
          <Row label={t("playground.context.cost")} value={formatCost(meta.cost)} />
          <p className="text-muted-foreground">{t("playground.metrics.estimate_hint")}</p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              void copyTextToClipboard(JSON.stringify(meta.requestSnapshot ?? {}, null, 2))
            }
          >
            <Copy data-icon="inline-start" />
            {t("playground.metrics.request_json")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              void copyTextToClipboard(
                buildCurl(meta.requestSnapshot ?? {}, window.location.origin),
              )
            }
          >
            <Copy data-icon="inline-start" />
            {t("playground.metrics.copy_curl")}
          </Button>
        </div>
      </HoverCardContent>
    </HoverCard>
  );
}
