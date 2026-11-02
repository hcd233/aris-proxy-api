"use client";

import { useEffect, useState } from "react";
import { ProviderIcon } from "@/components/provider-icon";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { Badge } from "@/components/ui/badge";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Progress } from "@/components/ui/progress";
import { useT } from "@/lib/i18n";
import { useMotionTier } from "@/lib/motion/tier-store";
import { formatCost } from "@/lib/money";
import { PORTAL_MODELS, PORTAL_ROUTE, PORTAL_TTY_LINES, type PortalTtyLine } from "@/lib/portal";
import { cn, formatDurationMs, formatInteger, formatTokenCount } from "@/lib/utils";
import { StatusDot, Terminal } from "./feature-art";

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between border-b border-dashed border-border py-2 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-foreground">{value}</span>
    </div>
  );
}

function Caption({ children }: { children: string }) {
  return <p className="text-[0.6875rem] tracking-wide text-muted-foreground">{children}</p>;
}

export function ApiKeyScene() {
  const t = useT();
  return (
    <>
      <Row label={t("portal.step.scene.key_name")} value="macbook-claude-code" />
      <div className="my-1 flex items-center justify-between gap-3 rounded-xl bg-muted/60 px-4 py-3.5 font-mono text-sm text-foreground">
        ak-7Qx2•••••••••••••3f9a
        <Badge variant="secondary" className="font-sans">
          {t("portal.step.scene.copied")}
        </Badge>
      </div>
      <Row label={t("portal.step.scene.key_perm")} value="user" />
      <Row label={t("portal.step.scene.key_created")} value={t("portal.step.scene.key_now")} />
      <p className="mt-1 text-xs text-muted-foreground">{t("portal.step.scene.key_hint")}</p>
    </>
  );
}

const ENDPOINTS = [
  { icon: "anthropic", name: "anthropic-main", kind: "API Key" },
  { icon: "claudecode", name: "portal.upstream.claudecode_sub", kind: "OAuth" },
  { icon: "deepseek", name: "deepseek-official", kind: "API Key" },
] as const;

export function UpstreamScene() {
  const t = useT();
  const bindings = PORTAL_ROUTE.tiers.flatMap((tier) =>
    tier.endpoints.slice(0, tier.priority === 1 ? 2 : 1).map((ep) => ({
      model: tier.priority === 1 ? PORTAL_MODELS.sonnet : PORTAL_MODELS.deepseek,
      at: ep.labelKey ? t(ep.labelKey) : ep.label,
      tag: `P${tier.priority} · ${ep.weight}`,
    })),
  );
  return (
    <>
      <Caption>{t("portal.step.scene.endpoints")}</Caption>
      {ENDPOINTS.map((e) => (
        <div
          key={e.name}
          className="flex h-10 items-center gap-2.5 rounded-xl bg-muted/60 px-3 text-sm"
        >
          <ProviderIcon decorative protocol={e.icon} size={16} />
          <span className="flex-1 text-foreground">
            {e.name.startsWith("portal.") ? t(e.name) : e.name}
          </span>
          <span className="text-xs text-muted-foreground">{e.kind}</span>
          <StatusDot tone="positive" />
        </div>
      ))}
      <div className="mt-2">
        <Caption>{t("portal.step.scene.alias")}</Caption>
      </div>
      {bindings.map((b) => (
        <div
          key={b.tag + b.at}
          className="flex items-center justify-between gap-3 rounded-lg px-3 py-1.5 text-xs ring-1 ring-border"
        >
          <span className="text-foreground">
            <span className="font-mono">{b.model}</span> @ {b.at}
          </span>
          <span className="shrink-0 text-primary">{b.tag}</span>
        </div>
      ))}
    </>
  );
}

const TTY_TONE: Record<PortalTtyLine["kind"], string> = {
  cmd: "",
  out: "opacity-70",
  ask: "",
  ok: "",
  blank: "",
};
const TTY_PREFIX: Record<PortalTtyLine["kind"], string> = {
  cmd: "$",
  out: "↓",
  ask: "?",
  ok: "✓",
  blank: "",
};

/** 终端回放：进入该步时逐行打出；still 档或卸载时直接/停止。 */
export function TerminalScene() {
  const tier = useMotionTier();
  const [shown, setShown] = useState(tier === "still" ? PORTAL_TTY_LINES.length : 0);

  useEffect(() => {
    if (tier === "still") return;
    let i = 0;
    let timer: ReturnType<typeof setTimeout>;
    const next = () => {
      i += 1;
      setShown(i);
      if (i < PORTAL_TTY_LINES.length) timer = setTimeout(next, PORTAL_TTY_LINES[i].ms);
    };
    timer = setTimeout(next, PORTAL_TTY_LINES[0].ms);
    return () => clearTimeout(timer);
  }, [tier]);

  const lines = tier === "still" ? PORTAL_TTY_LINES : PORTAL_TTY_LINES.slice(0, shown);
  return (
    <Terminal className="min-h-[21rem] text-[0.78rem] leading-7">
      <span className="sr-only">{PORTAL_TTY_LINES.map((l) => l.text).join("\n")}</span>
      <span aria-hidden="true">
        {lines.map((l, i) => (
          <span key={i} className={cn("block whitespace-pre-wrap", TTY_TONE[l.kind])}>
            {l.kind !== "blank" && (
              <span className="mr-2 text-[var(--chart-3)]">{TTY_PREFIX[l.kind]}</span>
            )}
            {l.text || " "}
          </span>
        ))}
        <span className="portal-caret inline-block h-4 w-2 translate-y-0.5 bg-current" />
      </span>
    </Terminal>
  );
}

export function SessionScene() {
  const t = useT();
  const [score, setScore] = useState<number | undefined>(4);
  return (
    <>
      <div className="flex flex-col gap-2.5 rounded-2xl bg-muted/60 p-4">
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
        <Bubble align="end">
          <BubbleContent>{t("portal.feature.art.chat_user")}</BubbleContent>
        </Bubble>
        <Bubble variant="outline">
          <BubbleContent>{t("portal.feature.art.chat_ai")}</BubbleContent>
        </Bubble>
        <div className="flex flex-wrap gap-1.5">
          <Badge variant="outline">{PORTAL_MODELS.sonnet}</Badge>
          <Badge variant="outline">{t("portal.cap.curation.messages").replace("{n}", "38")}</Badge>
          <Badge variant="outline">{formatTokenCount(126400)} Token</Badge>
          <Badge variant="outline">{formatCost(1.82, "USD")}</Badge>
        </div>
      </div>
      <div className="mt-1">
        <Caption>{t("portal.step.scene.audit")}</Caption>
      </div>
      <div className="flex items-center justify-between gap-3 rounded-lg px-3 py-2 text-xs ring-1 ring-border">
        <span className="flex items-center gap-2 text-foreground">
          <ProviderIcon decorative protocol="anthropic" size={14} />
          <span className="font-mono">{PORTAL_MODELS.sonnet}</span>
          <span className="text-muted-foreground">
            {t("portal.step.scene.first_token").replace("{ms}", formatDurationMs(410))}
          </span>
        </span>
        <span className="text-positive">200</span>
      </div>
      <div className="flex items-center gap-3 rounded-xl px-4 py-3 text-sm ring-1 ring-border">
        <span className="shrink-0 text-foreground">{t("portal.step.scene.export")}</span>
        <Progress value={64} className="flex-1" aria-label={t("portal.step.scene.export")} />
        <span className="shrink-0 font-medium text-foreground">
          {t("portal.cap.curation.export_count").replace("{n}", formatInteger(1204))}
        </span>
      </div>
    </>
  );
}
