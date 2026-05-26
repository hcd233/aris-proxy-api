"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { SessionSummary, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ChevronLeft, ChevronRight, MessageSquare } from "lucide-react";

export default function SessionsPage() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: 1,
    pageSize: 20,
    total: 0,
  });
  const [loading, setLoading] = useState(true);

  const fetchSessions = useCallback(async (page: number, pageSize: number) => {
    setLoading(true);
    try {
      const rsp = await api.listSessions(page, pageSize);
      setSessions(rsp.sessions ?? []);
      if (rsp.pageInfo) setPageInfo(rsp.pageInfo);
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchSessions(1, 20);
  }, [fetchSessions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const totalPages = Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize));

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-4xl font-bold tracking-tight text-foreground">Sessions</h1>
        <p className="text-sm text-muted-foreground">
          View and browse your conversation sessions
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All Sessions</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <MessageSquare className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">No sessions found</p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Summary</TableHead>
                    <TableHead>Messages</TableHead>
                    <TableHead>Created</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sessions.map((s) => (
                    <TableRow
                      key={s.id}
                      className="cursor-pointer"
                      onClick={() => {
                        window.location.href = `/web/sessions/detail/?id=${s.id}`;
                      }}
                    >
                      <TableCell className="font-mono text-xs">
                        {s.id}
                      </TableCell>
                      <TableCell className="max-w-[300px] truncate">
                        {s.summary || "—"}
                      </TableCell>
                      <TableCell>{s.messageIds?.length ?? 0}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(s.createdAt).toLocaleDateString()}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Pagination */}
              <div className="mt-4 flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  {pageInfo.total} session{pageInfo.total !== 1 ? "s" : ""} total
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page <= 1}
                    onClick={() => fetchSessions(pageInfo.page - 1, pageInfo.pageSize)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="text-sm">
                    {pageInfo.page} / {totalPages}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page >= totalPages}
                    onClick={() => fetchSessions(pageInfo.page + 1, pageInfo.pageSize)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}