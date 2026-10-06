"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { formatCost } from "@/lib/money";
import { showErrorToast } from "@/lib/api-error-handler";
import type {
  AuditCostSummaryRsp,
  AuditCostDistributionRsp,
  AuditCostTotalItem,
  AuditCostSeriesPoint,
  CostGroupBy,
} from "@/lib/types";

export interface CostPanelProps {
  startTime: string;
  endTime: string;
}

const GROUP_BY_OPTIONS: CostGroupBy[] = ["model", "api_key", "user"];

/**
 * CostPanel 成本面板：成本合计卡（按币种）+ 费用趋势（文本序列）+ 成本分布（分组切换）。
 * 图表以清单形式呈现（币种 × 时间桶），避免为成本单独立图表组件。
 */
export function CostPanel({ startTime, endTime }: CostPanelProps) {
  const t = useT();
  const [summary, setSummary] = useState<AuditCostSummaryRsp>();
  const [dist, setDist] = useState<AuditCostDistributionRsp>();
  const [groupBy, setGroupBy] = useState<CostGroupBy>("model");
  const [isAdmin, setIsAdmin] = useState(false);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const [s, d] = await Promise.all([
          api.getAuditCostSummary({ startTime, endTime, granularity: "day" }),
          api.getAuditCostDistribution({ groupBy, startTime, endTime, limit: 10 }),
        ]);
        if (!alive) return;
        setSummary(s);
        setDist(d);
        setIsAdmin(true);
      } catch (err) {
        if (!alive) return;
        // group_by=user 对普通用户 403：回退模型维度并隐藏用户分组
        if (groupBy === "user") {
          setGroupBy("model");
          setIsAdmin(false);
        } else {
          showErrorToast(err, { title: t("common.error") });
        }
      }
    };
    void load();
    return () => {
      alive = false;
    };
  }, [startTime, endTime, groupBy, t]);

  const totals: AuditCostTotalItem[] = summary?.totals ?? [];
  const series: AuditCostSeriesPoint[] = summary?.series ?? [];
  const items = dist?.items ?? [];

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="font-display text-base">{t("audit.cost.summary")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-4">
            {totals.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("audit.cost.unpriced")}</p>
            ) : (
              totals.map((item) => (
                <div key={item.currency} className="rounded-md border px-3 py-2">
                  <div className="text-xs text-muted-foreground">{item.currency}</div>
                  <div className="text-lg font-semibold">
                    {formatCost(item.cost, item.currency)}
                  </div>
                </div>
              ))
            )}
          </div>
          <div className="space-y-1">
            <div className="text-xs font-medium text-muted-foreground">{t("audit.cost.trend")}</div>
            <div className="max-h-40 space-y-0.5 overflow-y-auto text-xs">
              {series.map((p, i) => (
                <div key={i} className="flex items-center justify-between gap-2">
                  <span className="text-muted-foreground">
                    {p.bucketTime.slice(0, 10)} · {p.currency}
                  </span>
                  <span>{formatCost(p.cost, p.currency)}</span>
                </div>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="font-display text-base">{t("audit.cost.distribution")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-1.5">
            {GROUP_BY_OPTIONS.filter((g) => g !== "user" || isAdmin).map((g) => (
              <Button
                key={g}
                type="button"
                size="sm"
                variant={groupBy === g ? "default" : "outline"}
                onClick={() => setGroupBy(g)}
              >
                {t(`audit.cost.groupBy.${g === "api_key" ? "apiKey" : g}`)}
              </Button>
            ))}
          </div>
          <div className="space-y-1">
            {items.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("audit.cost.unpriced")}</p>
            ) : (
              items.map((item) => (
                <div
                  key={`${item.id}|${item.currency}`}
                  className="flex items-center justify-between gap-2 text-sm"
                >
                  <span className="min-w-0 break-words">{item.name || item.id}</span>
                  <span className="shrink-0 text-muted-foreground">
                    {item.currency} · {formatCost(item.cost, item.currency)}
                  </span>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
