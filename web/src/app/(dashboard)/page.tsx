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
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">Dashboard</h1>
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
          {/* Recent Sessions */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="font-display">Recent Sessions</CardTitle>
              <Link href="/sessions/">
                <Button variant="ghost" size="sm" className="text-muted-foreground hover:text-foreground">
                  View all <ArrowRight className="ml-1 size-4" />
                </Button>
              </Link>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="space-y-3">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-12 w-full" />
                  ))}
                </div>
              ) : recentSessions.length === 0 ? (
                <p className="py-8 text-center text-sm text-muted-foreground">
                  No sessions yet
                </p>
              ) : (
                <div className="space-y-1">
                  {recentSessions.map((s) => (
                    <Link
                      key={s.id}
                      href={`/sessions/detail/${s.id}`}
                      className="flex items-center justify-between rounded-lg px-3 py-2.5 transition-all duration-150 hover:bg-secondary"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">
                          {s.summary || `Session #${s.id}`}
                        </p>
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {new Date(s.createdAt).toLocaleDateString()}
                        </p>
                      </div>
                      <Badge variant="secondary" className="ml-2 shrink-0 text-xs">
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
              <CardTitle className="font-display">Quick Actions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Link href="/apikeys/" className="block">
                <Button variant="outline" className="w-full justify-start gap-3 h-10">
                  <Plus className="size-4 text-muted-foreground" />
                  Create API Key
                </Button>
              </Link>
              <Link href="/sessions/" className="block">
                <Button variant="outline" className="w-full justify-start gap-3 h-10">
                  <MessageSquare className="size-4 text-muted-foreground" />
                  View Sessions
                </Button>
              </Link>
              {isAdmin() && (
                <>
                  <Link href="/endpoints/" className="block">
                    <Button variant="outline" className="w-full justify-start gap-3 h-10">
                      <Server className="size-4 text-muted-foreground" />
                      Manage Endpoints
                    </Button>
                  </Link>
                  <Link href="/models/" className="block">
                    <Button variant="outline" className="w-full justify-start gap-3 h-10">
                      <Cpu className="size-4 text-muted-foreground" />
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