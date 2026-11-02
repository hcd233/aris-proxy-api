"use client";

import { useState, type CSSProperties, type ReactNode } from "react";
import { ProviderIcon } from "@/components/provider-icon";
import { MetricNumber } from "@/components/motion/metric-number";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { Badge } from "@/components/ui/badge";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Progress } from "@/components/ui/progress";
import { useT } from "@/lib/i18n";
import { formatCost } from "@/lib/money";
import { PORTAL_CLIENTS, PORTAL_VENDORS, type PortalFeatureKey } from "@/lib/portal";
import { cn, formatInteger, formatTokenCount } from "@/lib/utils";
import { usePortalLoop } from "./use-portal-loop";

/** 门户共用的小件：浮起的白卡、迷你终端、状态点。 */
export function Chip({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "flex items-center gap-2 rounded-xl bg-card px-3 py-2 text-xs shadow-sm ring-1 ring-border/60",
        className,
      )}
    >
      {children}
    </span>
  );
}

export function Terminal({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "rounded-xl border border-[var(--code-border)] bg-[var(--code-bg)] px-4 py-3 font-mono text-xs leading-relaxed text-[var(--code-text)]",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function StatusDot({ tone }: { tone: "positive" | "negative" | "warning" }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "size-1.5 shrink-0 rounded-full",
        tone === "positive" && "bg-positive",
        tone === "warning" && "bg-warning",
        tone === "negative" && "portal-pulse bg-negative text-negative",
      )}
    />
  );
}

function SingleEndpointArt() {
  const t = useT();
  return (
    <>
      <div className="flex h-9 items-center gap-2 rounded-full bg-card px-4 font-mono text-xs shadow-sm ring-1 ring-border/60">
        <StatusDot tone="positive" />
        <span className="flex-1">api.your-domain.com</span>
        <span className="font-sans text-muted-foreground">{t("portal.feature.art.connected")}</span>
      </div>
      <div className="flex justify-center gap-2">
        {PORTAL_VENDORS.map((v, i) => (
          <span
            key={v.id}
            className="portal-bob flex size-9 items-center justify-center rounded-xl bg-card text-foreground shadow-sm ring-1 ring-border/60"
            style={{ "--delay": `${i * 300}ms` } as CSSProperties}
          >
            <ProviderIcon decorative protocol={v.icon} size={18} />
          </span>
        ))}
      </div>
    </>
  );
}

function SessionsArt() {
  const t = useT();
  const [score, setScore] = useState<number | undefined>(4);
  return (
    <>
      <Bubble align="end">
        <BubbleContent>{t("portal.feature.art.chat_user")}</BubbleContent>
      </Bubble>
      <Bubble variant="outline">
        <BubbleContent>{t("portal.feature.art.chat_ai")}</BubbleContent>
      </Bubble>
      <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <span>{t("portal.feature.art.session_saved")}</span>
        <ScoreDots
          score={score}
          scoring={false}
          onScore={setScore}
          onClear={() => setScore(undefined)}
          size={12}
        />
      </div>
    </>
  );
}

function ExportCliArt() {
  const t = useT();
  return (
    <>
      <Terminal>
        <span className="text-[var(--chart-3)]">$</span> aris model export
        <br />
        <span className="text-[var(--chart-3)]">✓</span> {t("portal.feature.art.export_done")}
      </Terminal>
      <div className="flex flex-wrap justify-center gap-1.5">
        {PORTAL_CLIENTS.slice(0, 3).map((c) => (
          <Badge key={c.id} variant="outline">
            <ProviderIcon decorative protocol={c.icon} size={12} />
            {c.label}
          </Badge>
        ))}
      </div>
    </>
  );
}

function UsageArt() {
  const t = useT();
  const stats = [
    { label: t("portal.feature.art.tokens"), value: formatTokenCount(126400) },
    { label: t("portal.feature.art.cost"), value: formatCost(1.82, "USD") },
    { label: t("portal.feature.art.success"), value: "99.2%" },
  ];
  return (
    <>
      <div className="grid grid-cols-3 gap-2">
        {stats.map((s) => (
          <div
            key={s.label}
            className="flex flex-col text-[0.6875rem] leading-tight text-muted-foreground"
          >
            <MetricNumber
              value={s.value}
              className="text-lg font-medium tracking-tight text-foreground"
            />
            {s.label}
          </div>
        ))}
      </div>
      <svg
        viewBox="0 0 320 70"
        preserveAspectRatio="none"
        className="h-16 w-full"
        aria-hidden="true"
      >
        <path
          d="M0 56C30 52 44 34 72 38S120 56 150 40 206 10 238 18 290 34 320 8V70H0Z"
          className="fill-primary/15"
        />
        <path
          d="M0 56C30 52 44 34 72 38S120 56 150 40 206 10 238 18 290 34 320 8"
          className="fill-none stroke-primary"
          strokeWidth={2}
          strokeLinecap="round"
        />
      </svg>
    </>
  );
}

function SelfHealArt() {
  const t = useT();
  const rows = [
    {
      icon: "anthropic",
      label: "Anthropic",
      tone: "positive" as const,
      note: t("portal.feature.art.healthy"),
    },
    {
      icon: "deepseek",
      label: "DeepSeek",
      tone: "positive" as const,
      note: t("portal.feature.art.healthy"),
    },
    {
      icon: "minimax",
      label: "MiniMax",
      tone: "negative" as const,
      note: t("portal.feature.art.paused"),
    },
  ];
  return (
    <>
      {rows.map((r) => (
        <Chip key={r.label}>
          <StatusDot tone={r.tone} />
          <ProviderIcon decorative protocol={r.icon} size={15} />
          <span className="flex-1">{r.label}</span>
          <span className={r.tone === "negative" ? "text-negative" : "text-muted-foreground"}>
            {r.note}
          </span>
        </Chip>
      ))}
    </>
  );
}

function DatasetArt() {
  const t = useT();
  const [pct, setPct] = useState(68);
  usePortalLoop(() => setPct((p) => (p >= 100 ? 8 : Math.min(100, p + 12))), 600);
  return (
    <>
      <div className="flex flex-wrap gap-1.5">
        <Badge variant="secondary">{t("portal.cap.curation.filter_score")}</Badge>
        <Badge variant="secondary">claude-opus-5-5</Badge>
        <Badge variant="secondary">{t("portal.cap.curation.filter_range")}</Badge>
      </div>
      <p className="text-2xl font-medium tracking-tight text-foreground">
        {formatInteger(1204)}{" "}
        <span className="text-xs font-normal tracking-normal text-muted-foreground">
          {t("portal.feature.art.filtered")}
        </span>
      </p>
      <Progress value={pct} aria-label={t("portal.cap.curation.export_label")} />
    </>
  );
}

export const FEATURE_ART: Record<PortalFeatureKey, () => ReactNode> = {
  single_endpoint: SingleEndpointArt,
  sessions: SessionsArt,
  export_cli: ExportCliArt,
  usage: UsageArt,
  self_heal: SelfHealArt,
  dataset: DatasetArt,
};
