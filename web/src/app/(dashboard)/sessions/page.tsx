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
import { ChevronLeft, ChevronRight, MessageSquare, ListFilter, Check } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export default function SessionsPage() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: 1,
    pageSize: 20,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [pageInputValue, setPageInputValue] = useState("1");

  useEffect(() => {
    setPageInputValue(String(pageInfo.page));
  }, [pageInfo.page]);

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
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">Sessions</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          View and browse your conversation sessions
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">All Sessions</CardTitle>
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
                        window.location.href = `/web/sessions/detail/${s.id}`;
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
              <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                  <DropdownMenu>
                    <DropdownMenuTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
                      <ListFilter className="size-3.5" />
                      {pageInfo.pageSize} / page
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start">
                      {[20, 50, 100, 200].map((size) => (
                        <DropdownMenuItem
                          key={size}
                          onClick={() => fetchSessions(1, size)}
                        >
                          {size === pageInfo.pageSize && (
                            <Check className="size-4" />
                          )}
                          <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>
                            {size} per page
                          </span>
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <p className="text-sm text-muted-foreground">
                    {pageInfo.total} session{pageInfo.total !== 1 ? "s" : ""} total
                  </p>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page <= 1}
                    onClick={() => fetchSessions(pageInfo.page - 1, pageInfo.pageSize)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <div className="flex items-center gap-1.5 text-sm">
                    <span className="text-muted-foreground">Page</span>
                    <input
                      type="number"
                      min={1}
                      max={totalPages}
                      value={pageInputValue}
                      onChange={(e) => setPageInputValue(e.target.value)}
                      className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          let page = parseInt(pageInputValue, 10);
                          if (Number.isNaN(page)) page = 1;
                          page = Math.max(1, Math.min(page, totalPages));
                          fetchSessions(page, pageInfo.pageSize);
                        }
                      }}
                      onBlur={() => {
                        let page = parseInt(pageInputValue, 10);
                        if (Number.isNaN(page)) page = 1;
                        page = Math.max(1, Math.min(page, totalPages));
                        fetchSessions(page, pageInfo.pageSize);
                      }}
                    />
                    <span className="text-muted-foreground">/ {totalPages}</span>
                  </div>
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