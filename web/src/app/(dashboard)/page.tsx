"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Key, MessageSquare, Server, Cpu } from "lucide-react";
import { ModelTrendChart } from "@/components/charts/model-trend-chart";
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
}: {
  title: string;
  value: number;
  icon: React.ReactNode;
  loading: boolean;
}) {
  return (
    <Card>
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
          <div className="font-display text-3xl font-semibold text-foreground">{value}</div>
        )}
      </CardContent>
    </Card>
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

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
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

      const canListEndpoints = isAdmin() || isModuleOpen("endpoints");
      const canListModels = isAdmin() || isModuleOpen("models");
      const endpointsRsp = canListEndpoints ? await api.listEndpoints(1, 1).catch(() => null) : null; // 仅探测是否存在 endpoint
      const modelsRsp = canListModels ? await api.listModels(1, 1).catch(() => null) : null;

      setStats({
        apiKeys: keysRsp?.pageInfo?.total ?? 0,
        sessions: sessionsRsp?.pageInfo?.total ?? 0,
        endpoints: endpointsRsp?.pageInfo?.total ?? 0,
        models: modelsRsp?.pageInfo?.total ?? 0,
      });
    } catch {
      // Errors handled silently — dashboard shows zeros
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
      <div>
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
          {t("dashboard.title")}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">{t("dashboard.overview")}</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {!isDemo() && (
          <StatCard
            title={t("apikeys.title")}
            value={stats.apiKeys}
            icon={<Key className="size-4" />}
            loading={loading}
          />
        )}
        <StatCard
          title={t("sessions.title")}
          value={stats.sessions}
          icon={<MessageSquare className="size-4" />}
          loading={loading}
        />
        {(isAdmin() || isModuleOpen("endpoints")) && (
          <StatCard
            title={t("endpoints.title")}
            value={stats.endpoints}
            icon={<Server className="size-4" />}
            loading={loading}
          />
        )}
        {(isAdmin() || isModuleOpen("models")) && (
          <StatCard
            title={t("models.title")}
            value={stats.models}
            icon={<Cpu className="size-4" />}
            loading={loading}
          />
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <ModelTrendChart />
        <RequestRateChart />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <TokenVolumeChart />
        <ModelTokenBarChart />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <FirstTokenLatencyChart />
        <TokenRateChart />
      </div>
    </div>
    </PermissionGuard>
  );
}
