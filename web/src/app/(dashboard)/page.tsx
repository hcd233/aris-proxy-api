"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import type { SessionSummary } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Key,
  MessageSquare,
  Server,
  Cpu,
  Plus,
  ArrowRight,
} from "lucide-react";

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
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        {icon}
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-8 w-16" />
        ) : (
          <div className="text-2xl font-bold">{value}</div>
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
  const [recentSessions, setRecentSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const [keysRsp, sessionsRsp] = await Promise.all([
        api.listAPIKeys(),
        api.listSessions(1, 5),
      ]);

      const endpointsCount = isAdmin() ? (await api.listEndpoints().catch(() => ({ endpoints: [] }))).endpoints?.length ?? 0 : 0;
      const modelsCount = isAdmin() ? (await api.listModels().catch(() => ({ models: [] }))).models?.length ?? 0 : 0;

      setStats({
        apiKeys: keysRsp.keys?.length ?? 0,
        sessions: sessionsRsp.pageInfo?.total ?? 0,
        endpoints: endpointsCount,
        models: modelsCount,
      });
      setRecentSessions(sessionsRsp.sessions ?? []);
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
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-4xl font-bold tracking-tight text-foreground">Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          Overview of your Aris Proxy API resources
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="API Keys"
          value={stats.apiKeys}
          icon={<Key className="size-4 text-muted-foreground" />}
          loading={loading}
        />
        <StatCard
          title="Sessions"
          value={stats.sessions}
          icon={<MessageSquare className="size-4 text-muted-foreground" />}
          loading={loading}
        />
        {isAdmin() && (
          <StatCard
            title="Endpoints"
            value={stats.endpoints}
            icon={<Server className="size-4 text-muted-foreground" />}
            loading={loading}
          />
        )}
        {isAdmin() && (
          <StatCard
            title="Models"
            value={stats.models}
            icon={<Cpu className="size-4 text-muted-foreground" />}
            loading={loading}
          />
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {/* Recent Sessions */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Recent Sessions</CardTitle>
            <Link href="/sessions/">
              <Button variant="ghost" size="sm">
                View all <ArrowRight className="ml-1 size-3" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : recentSessions.length === 0 ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                No sessions yet
              </p>
            ) : (
              <div className="space-y-2">
                {recentSessions.map((s) => (
                  <Link
                    key={s.id}
                    href={`/sessions/detail/?id=${s.id}`}
                    className="flex items-center justify-between rounded-lg px-3 py-2 transition-colors hover:bg-accent"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">
                        {s.summary || `Session #${s.id}`}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(s.createdAt).toLocaleDateString()}
                      </p>
                    </div>
                    <Badge variant="outline" className="ml-2 shrink-0 text-xs">
                      {s.messageIds?.length ?? 0} msgs
                    </Badge>
                  </Link>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Quick Actions */}
        <Card>
          <CardHeader>
            <CardTitle>Quick Actions</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Link href="/apikeys/" className="block">
              <Button variant="outline" className="w-full justify-start gap-2">
                <Plus className="size-4" />
                Create API Key
              </Button>
            </Link>
            <Link href="/sessions/" className="block">
              <Button variant="outline" className="w-full justify-start gap-2">
                <MessageSquare className="size-4" />
                View Sessions
              </Button>
            </Link>
            {isAdmin() && (
              <>
                <Link href="/endpoints/" className="block">
                  <Button variant="outline" className="w-full justify-start gap-2">
                    <Server className="size-4" />
                    Manage Endpoints
                  </Button>
                </Link>
                <Link href="/models/" className="block">
                  <Button variant="outline" className="w-full justify-start gap-2">
                    <Cpu className="size-4" />
                    Manage Models
                  </Button>
                </Link>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}