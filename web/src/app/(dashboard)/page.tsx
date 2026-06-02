"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Key,
  MessageSquare,
  Server,
  Cpu,
} from "lucide-react";
import { ModelTrendChart } from "@/components/charts/model-trend-chart";
import { RequestRateChart } from "@/components/charts/request-rate-chart";
import { TokenVolumeChart } from "@/components/charts/token-volume-chart";
import { TokenRateChart } from "@/components/charts/token-rate-chart";
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
    <Card className="hover:border-border/60">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <div className="flex items-center gap-2 text-muted-foreground">
          {icon}
          <span className="text-xs font-medium uppercase tracking-wider">{title}</span>
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
  const { isAdmin } = useAuth();
  const [stats, setStats] = useState<DashboardStats>({
    apiKeys: 0,
    sessions: 0,
    endpoints: 0,
    models: 0,
  });
  const [loading, setLoading] = useState(true);

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const [keysRsp, sessionsRsp] = await Promise.all([
        api.listAPIKeys(),
        api.listSessions({ page: 1, pageSize: 5 }),
      ]);

      const endpointsCount = isAdmin() ? (await api.listEndpoints().catch(() => ({ endpoints: [] }))).endpoints?.length ?? 0 : 0;
      const modelsCount = isAdmin() ? (await api.listModels().catch(() => ({ models: [] }))).models?.length ?? 0 : 0;

      setStats({
        apiKeys: keysRsp.keys?.length ?? 0,
        sessions: sessionsRsp.pageInfo?.total ?? 0,
        endpoints: endpointsCount,
        models: modelsCount,
      });
    } catch {
      // Errors handled silently — dashboard shows zeros
    } finally {
      setLoading(false);
    }
  }, [isAdmin]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchDashboard();
  }, [fetchDashboard]);
  /* eslint-enable react-hooks/set-state-in-effect */

  return (
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">Dashboard</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            Overview of your Aris Proxy API resources
          </p>
        </div>

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            title="API Keys"
            value={stats.apiKeys}
            icon={<Key className="size-4" />}
            loading={loading}
          />
          <StatCard
            title="Sessions"
            value={stats.sessions}
            icon={<MessageSquare className="size-4" />}
            loading={loading}
          />
          {isAdmin() && (
            <StatCard
              title="Endpoints"
              value={stats.endpoints}
              icon={<Server className="size-4" />}
              loading={loading}
            />
          )}
          {isAdmin() && (
            <StatCard
              title="Models"
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
          <TokenRateChart />
        </div>

        <div className="grid gap-4">
          <ModelTokenBarChart />
        </div>
      </div>
  );
}