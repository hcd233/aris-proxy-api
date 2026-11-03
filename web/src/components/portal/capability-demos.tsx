"use client";

import { useState, type ReactNode } from "react";
import { ProviderIcon } from "@/components/provider-icon";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { useT } from "@/lib/i18n";
import { useMotionTier } from "@/lib/motion/tier-store";
import { PORTAL_AUDIT_ROWS, PORTAL_MODELS, PORTAL_ROUTE } from "@/lib/portal";
import { breakerFrames, type BreakerCell } from "@/lib/portal-motion";
import { cn, formatDurationMs, formatInteger, formatTokenCount } from "@/lib/utils";
import { usePortalLoop } from "./use-portal-loop";

/** 示例面板外壳：小标题 + 内容。 */
export function DemoPane({
  label,
  children,
  className,
}: {
  label: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    // 不再给每块演示再套一层淡底面板：卡片 > 面板 > 内框 的三层嵌套会让内框失去层次，
    // 改用发丝线分段，内框（Code / 行 / 卡）成为卡片里唯一的二级容器。
    <figure className={cn("min-w-0 border-t border-border/60 pt-4", className)}>
      <figcaption className="mb-3 text-[0.6875rem] tracking-wide text-muted-foreground">
        {label}
      </figcaption>
      {children}
    </figure>
  );
}

function Code({ children }: { children: ReactNode }) {
  return (
    <pre className="overflow-hidden rounded-xl bg-card p-3 font-mono text-[0.6875rem] leading-relaxed whitespace-pre text-foreground shadow-sm ring-1 ring-border/60">
      {children}
    </pre>
  );
}

const ROUTE_SEQUENCE = [0, 0, 1, 0, 0, 1, 0] as const;

export function RouteDemo() {
  const t = useT();
  const [step, setStep] = useState(0);
  usePortalLoop(() => setStep((s) => (s + 1) % ROUTE_SEQUENCE.length), 1700);
  const selected = ROUTE_SEQUENCE[step];
  return (
    <div className="grid grid-cols-[5.5rem_1fr] items-center gap-5">
      <div className="relative rounded-xl bg-foreground px-2 py-3 text-center font-mono text-sm font-medium text-background">
        {PORTAL_ROUTE.alias}
        <span className="block font-sans text-[0.6875rem] font-normal opacity-70">
          {t("portal.cap.access.alias_caption")}
        </span>
        <span
          aria-hidden="true"
          className="absolute top-1/2 left-full w-5 border-t-[1.5px] border-dashed border-muted-foreground"
        />
      </div>
      <ol className="flex min-w-0 flex-col gap-2 border-l-[1.5px] border-dashed border-muted-foreground pl-3.5">
        {PORTAL_ROUTE.tiers.map((tier, ti) => (
          <li key={tier.priority} className="flex min-w-0 items-center gap-2">
            <span className="w-14 shrink-0 text-[0.6875rem] text-muted-foreground">
              {t("portal.cap.access.priority").replace("{n}", String(tier.priority))}
            </span>
            {tier.endpoints.map((ep, ei) => (
              <span
                key={ep.label}
                className={cn(
                  "flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-lg bg-card px-2.5 py-1.5 text-xs shadow-sm ring-1 ring-border/60 transition-shadow duration-300",
                  ti === 0 && ei === selected && "ring-[1.5px] ring-primary",
                )}
              >
                <ProviderIcon decorative protocol={ep.icon} size={15} />
                <span className="flex min-w-0 flex-col leading-tight">
                  <span className="text-foreground">{ep.labelKey ? t(ep.labelKey) : ep.label}</span>
                  <span className="text-[0.6875rem] text-muted-foreground">
                    {t("portal.cap.access.weight").replace("{n}", String(ep.weight))}
                  </span>
                </span>
              </span>
            ))}
          </li>
        ))}
      </ol>
    </div>
  );
}

export function ProtocolDemo() {
  const t = useT();
  return (
    <div className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[1fr_auto_1fr]">
      <Code>
        <span className="text-muted-foreground">{t("portal.cap.access.protocol_client")}</span>
        {"\n"}
        <b className="font-medium text-primary">POST</b> …/chat/completions
        {`\n{ "model": "${PORTAL_ROUTE.alias}",\n  "stream": true }`}
      </Code>
      <span
        aria-hidden="true"
        className="portal-bob justify-self-center text-lg text-primary max-sm:rotate-90"
      >
        ⇄
      </span>
      <Code>
        <span className="text-muted-foreground">{t("portal.cap.access.protocol_upstream")}</span>
        {"\n"}
        <b className="font-medium text-primary">POST</b> /v1/messages
        {`\n{ "model": "${PORTAL_MODELS.sonnet}",\n  "stream": true }`}
      </Code>
    </div>
  );
}

export function SessionDemo() {
  const t = useT();
  const [score, setScore] = useState<number | undefined>(4);
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-foreground">
          {t("portal.cap.curation.session_title")}
        </span>
        <ScoreDots
          score={score}
          scoring={false}
          onScore={setScore}
          onClear={() => setScore(undefined)}
          size={13}
        />
      </div>
      <div className="flex flex-wrap gap-1.5">
        <Badge variant="outline">{t("portal.cap.curation.messages").replace("{n}", "38")}</Badge>
        <Badge variant="outline">{t("portal.cap.curation.tools").replace("{n}", "6")}</Badge>
        <Badge variant="outline">{PORTAL_MODELS.sonnet}</Badge>
        <Badge variant="secondary">{t("portal.cap.curation.share")}</Badge>
      </div>
    </div>
  );
}

export function DatasetDemo() {
  const t = useT();
  const [pct, setPct] = useState(72);
  usePortalLoop(() => setPct((p) => (p >= 100 ? 6 : Math.min(100, p + 9))), 500);
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-1.5">
        <Badge variant="secondary">{t("portal.cap.curation.filter_score")}</Badge>
        <Badge variant="secondary">{PORTAL_MODELS.opus}</Badge>
        <Badge variant="secondary">{t("portal.cap.curation.filter_range")}</Badge>
      </div>
      <Code>
        {`{"conversations":[\n  {"from":"human","value":"…"},\n  {"from":"gpt","value":"<think>…"}]}`}
      </Code>
      <Progress value={pct} aria-label={t("portal.cap.curation.export_label")} />
      <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <span>{t("portal.cap.curation.export_note")}</span>
        <span className="font-medium text-foreground">
          {t("portal.cap.curation.export_count").replace("{n}", formatInteger(1204))}
        </span>
      </div>
    </div>
  );
}

const AUDIT_VISIBLE = 4;

export function AuditStreamDemo() {
  const [head, setHead] = useState(0);
  usePortalLoop(() => setHead((h) => (h + 1) % PORTAL_AUDIT_ROWS.length), 1900);
  const rows = Array.from({ length: AUDIT_VISIBLE }, (_, i) => {
    const idx = (head - i + PORTAL_AUDIT_ROWS.length * 2) % PORTAL_AUDIT_ROWS.length;
    return PORTAL_AUDIT_ROWS[idx];
  });
  return (
    <ol className="flex flex-col gap-1.5" aria-live="off">
      {rows.map((row, i) => (
        <li
          key={head - i}
          className={cn(
            "grid h-8 grid-cols-[1.125rem_minmax(0,1fr)_2.25rem] items-center gap-2 rounded-lg bg-card px-2.5 text-xs shadow-sm ring-1 ring-border/60 sm:grid-cols-[1.125rem_minmax(0,1fr)_3.25rem_3rem_2.25rem]",
            i === 0 && "portal-slide-in",
          )}
        >
          <ProviderIcon decorative protocol={row.icon} size={14} />
          <span className="overflow-hidden font-mono whitespace-nowrap text-foreground">
            {row.model}
          </span>
          <span className="text-right text-muted-foreground tabular-nums max-sm:hidden">
            {formatTokenCount(row.tokens)}
          </span>
          <span className="text-right text-muted-foreground tabular-nums max-sm:hidden">
            {formatDurationMs(row.firstTokenMs)}
          </span>
          <span
            className={cn(
              "text-right tabular-nums",
              row.status >= 500 ? "text-negative" : "text-positive",
            )}
          >
            {row.status}
          </span>
        </li>
      ))}
    </ol>
  );
}

const TIERS = ["pending", "demo", "user", "admin"] as const;
const TIER_HEIGHT = ["h-[36%]", "h-[54%]", "h-[76%]", "h-full"] as const;

export function PermissionLadder() {
  return (
    <ol className="grid h-16 grid-cols-4 items-end gap-1.5">
      {TIERS.map((tier, i) => (
        <li
          key={tier}
          className={cn(
            "flex items-end justify-center rounded-t-lg rounded-b-sm pb-1 font-mono text-[0.6875rem]",
            TIER_HEIGHT[i],
            i === TIERS.length - 1
              ? "bg-primary text-primary-foreground"
              : "bg-primary/15 text-foreground",
          )}
        >
          {tier}
        </li>
      ))}
    </ol>
  );
}

export function TriggerActions() {
  const t = useT();
  const items = ["deny", "omit", "capture"] as const;
  return (
    <dl className="flex flex-col gap-1.5 text-xs">
      {items.map((k) => (
        <div key={k} className="flex items-baseline gap-2">
          <dt className="w-10 shrink-0 font-medium text-primary">
            {t(`portal.cap.governance.trigger_${k}`)}
          </dt>
          <dd className="text-muted-foreground">{t(`portal.cap.governance.trigger_${k}_desc`)}</dd>
        </div>
      ))}
    </dl>
  );
}

const FRAMES = breakerFrames();
const OPEN_FRAME = FRAMES.findIndex((f) => f.state === "open");
const STATES = [
  {
    key: "closed",
    tone: "bg-positive",
    soft: "bg-positive/10 ring-positive/30",
    label: "CLOSED",
    desc: "portal.cap.resilience.state_closed",
  },
  {
    key: "open",
    tone: "bg-negative",
    soft: "bg-negative/10 ring-negative/30",
    label: "OPEN",
    desc: "portal.cap.resilience.state_open",
  },
  {
    key: "half_open",
    tone: "bg-warning",
    soft: "bg-warning/10 ring-warning/30",
    label: "HALF-OPEN",
    desc: "portal.cap.resilience.state_half_open",
  },
] as const;
const CELL_TONE: Record<BreakerCell, string> = {
  idle: "bg-border",
  ok: "bg-positive",
  err: "bg-negative",
  rejected: "bg-transparent ring-1 ring-muted-foreground/50",
  probe: "bg-warning",
};

export function BreakerDemo() {
  const t = useT();
  const tier = useMotionTier();
  const [i, setI] = useState(0);
  usePortalLoop(
    () => setI((x) => (x + 1) % FRAMES.length),
    () => FRAMES[i].holdMs,
  );
  const f = FRAMES[tier === "still" ? OPEN_FRAME : i];
  return (
    <div className="flex flex-col gap-3">
      <ol className="grid grid-cols-[1fr_1.5rem_1fr_1.5rem_1fr] items-center gap-1">
        {STATES.map((s, si) => (
          <li key={s.key} className="contents">
            {si > 0 && (
              <span
                aria-hidden="true"
                className="border-t-[1.5px] border-dashed border-muted-foreground"
              />
            )}
            <span
              className={cn(
                "rounded-xl px-1.5 py-3 text-center text-xs font-medium transition-all duration-300",
                /* 非当前态也带该状态的语义色（淡化到 10%）：否则静止看这张图时
                 * 三个状态长得一模一样，读者要等动画转到 OPEN 才知道哪个是“熔断” */
                f.state === s.key
                  ? cn(s.tone, "text-background shadow-md")
                  : cn(s.soft, "text-foreground shadow-sm ring-1"),
              )}
            >
              {s.label}
              <span
                className={cn(
                  "mt-0.5 block text-[0.6875rem] font-normal",
                  f.state === s.key ? "opacity-85" : "text-muted-foreground",
                )}
              >
                {t(s.desc)}
              </span>
            </span>
          </li>
        ))}
      </ol>
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{t("portal.cap.resilience.recent")}</span>
        <span>
          {t("portal.cap.resilience.error_rate")}{" "}
          <b className="font-medium text-foreground tabular-nums">
            {f.errorPct === null ? "—" : `${f.errorPct}%`}
          </b>
        </span>
      </div>
      <div className="grid grid-cols-10 gap-1.5" aria-hidden="true">
        {f.cells.map((c, ci) => (
          <span
            key={ci}
            className={cn("h-7 rounded-md transition-colors duration-200", CELL_TONE[c])}
          />
        ))}
      </div>
      <p className="min-h-[2.5rem] text-sm text-foreground" aria-live="off">
        {t(f.captionKey)}
      </p>
    </div>
  );
}

export function BulkheadDemo() {
  const t = useT();
  const [used, setUsed] = useState(9);
  usePortalLoop(
    () => setUsed((u) => Math.max(3, Math.min(32, u + Math.round((Math.random() - 0.42) * 7)))),
    900,
  );
  return (
    <div className="flex flex-col gap-2.5">
      <div className="grid grid-cols-16 gap-[3px]" aria-hidden="true">
        {Array.from({ length: 32 }, (_, i) => (
          <span
            key={i}
            className={cn(
              "h-3.5 rounded-[3px] transition-colors duration-200",
              i < used ? "bg-chart-4" : "bg-border",
            )}
          />
        ))}
      </div>
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{t("portal.cap.resilience.bulkhead_usage")}</span>
        <b className="font-medium text-foreground tabular-nums">{used} / 32</b>
      </div>
    </div>
  );
}
