"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Key, MessageSquare, Server, Cpu } from "lucide-react";
import { MetricNumber } from "@/components/motion/metric-number";
import { ModelCostBarChart } from "@/components/charts/model-cost-bar-chart";
import { ModelTrendChart } from "@/components/charts/model-trend-chart";
import { HarnessTrendChart } from "@/components/charts/harness-trend-chart";
import { RequestRateChart } from "@/components/charts/request-rate-chart";
import { TokenVolumeChart } from "@/components/charts/token-volume-chart";
import { TokenRateChart } from "@/components/charts/token-rate-chart";
import { FirstTokenLatencyChart } from "@/components/charts/first-token-latency-chart";
import { ModelTokenBarChart } from "@/components/charts/model-token-bar-chart";

interface DashboardStats {
  apiKeys: number;
  sessions: number;
  endpoints: number;
  models: number;
}

function StatCard({
  title,
  value,
  icon,
  loading,
  href,
}: {
  title: string;
  value: number;
  icon: React.ReactNode;
  loading: boolean;
  href: string;
}) {
  return (
    <Link
      href={href}
      className="block rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-2 focus-visible:ring-offset-background"
    >
      <Card data-interactive>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <div className="flex items-center gap-2.5">
            <span className="flex size-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
              {icon}
            </span>
            <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {title}
            </span>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          {loading ? (
            <Skeleton className="h-10 w-20" />
          ) : (
            <div className="font-display text-3xl font-semibold tabular-nums text-foreground">
              <MetricNumber value={value} />
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}

export default function DashboardPage() {
  const t = useT();
  const { isAdmin, isDemo, isModuleOpen } = useAuth();
  const [stats, setStats] = useState<DashboardStats>({
    apiKeys: 0,
    sessions: 0,
    endpoints: 0,
    models: 0,
  });
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    const demoUser = isDemo();
    try {
      // demo 未开放 apikeys 模块，跳过探测（避免 403）
      const keysPromise = demoUser
        ? Promise.resolve(null)
        : api.listAPIKeys(1, 1).catch(() => null);
      const [keysRsp, sessionsRsp] = await Promise.all([
        keysPromise,
        api.listSessions({ page: 1, pageSize: 1 }).catch(() => null),
      ]);

      const canListUpstream = isAdmin() || isModuleOpen("upstream");
      const upstreamRsp = canListUpstream ? await api.listUpstream(1, 1).catch(() => null) : null; // 端点数取 pageInfo.total，模型数取 modelTotal，一次调用双统计

      setStats({
        apiKeys: keysRsp?.pageInfo?.total ?? 0,
        sessions: sessionsRsp?.pageInfo?.total ?? 0,
        endpoints: upstreamRsp?.pageInfo?.total ?? 0,
        models: upstreamRsp?.modelTotal ?? 0,
      });
      // 0 和"加载失败"必须可区分：任一实际发起的请求失败都提示，而不是静默显示 0。
      const results = [sessionsRsp !== null];
      if (!demoUser) results.push(keysRsp !== null);
      if (canListUpstream) results.push(upstreamRsp !== null);
      setLoadError(results.some((ok) => !ok));
    } catch {
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [isAdmin, isDemo, isModuleOpen]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchDashboard();
  }, [fetchDashboard]);
  /* eslint-enable react-hooks/set-state-in-effect */

  return (
    <PermissionGuard module="dashboard">
      <div className="space-y-8">
        <div className="animate-rise">
          <h1 className="motion-title font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
            {t("dashboard.title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("dashboard.overview")}</p>
        </div>

        {loadError && (
          <Alert variant="destructive" className="items-center">
            <AlertCircle />
            <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
              <span>{t("dashboard.load_error")}</span>
              <Button variant="outline" size="sm" onClick={fetchDashboard}>
                {t("common.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        <div className="stagger-rise grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
          {!isDemo() && (
            <StatCard
              title={t("apikeys.title")}
              value={stats.apiKeys}
              icon={<Key className="size-4" />}
              loading={loading}
              href="/apikeys/"
            />
          )}
          <StatCard
            title={t("sessions.title")}
            value={stats.sessions}
            icon={<MessageSquare className="size-4" />}
            loading={loading}
            href="/sessions/"
          />
          {(isAdmin() || isModuleOpen("upstream")) && (
            <StatCard
              title={t("endpoints.title")}
              value={stats.endpoints}
              icon={<Server className="size-4" />}
              loading={loading}
              href="/upstream/"
            />
          )}
          {(isAdmin() || isModuleOpen("upstream")) && (
            <StatCard
              title={t("models.title")}
              value={stats.models}
              icon={<Cpu className="size-4" />}
              loading={loading}
              href="/upstream/"
            />
          )}
        </div>

        <div className="stagger-rise grid gap-4 lg:grid-cols-2 lg:gap-y-8">
          <ModelCostBarChart />
          <ModelTokenBarChart />
          <ModelTrendChart />
          <HarnessTrendChart />
          <RequestRateChart />
          <TokenVolumeChart />
          <TokenRateChart />
          <div className="lg:col-span-2">
            <FirstTokenLatencyChart />
          </div>
        </div>
      </div>
    </PermissionGuard>
  );
}
