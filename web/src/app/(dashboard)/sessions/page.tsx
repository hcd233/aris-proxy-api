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
import { Badge } from "@/components/ui/badge";
import {
  ChevronLeft,
  ChevronRight,
  MessageSquare,
  ListFilter,
  Check,
  ArrowUp,
  ArrowDown,
} from "lucide-react";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortDir = "asc" | "desc";

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  messageCount: "message_count",
  toolCount: "tool_count",
};

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  const year = d.getFullYear();
  const month = d.getMonth() + 1;
  const day = d.getDate();
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  const seconds = String(d.getSeconds()).padStart(2, "0");
  return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
}

export default function SessionsPage() {
  const isMobile = useIsMobile();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: 1,
    pageSize: 1,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [pageInputValue, setPageInputValue] = useState("1");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("30d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [sort, setSort] = useState<{ field: string; dir: SortDir }>({ field: "created_at", dir: "desc" });

  const fetchSessions = useCallback(
    async (
      page: number,
      pageSize: number,
      range: TimeRangeKey,
      cs: string,
      ce: string,
      sortState: { field: string; dir: SortDir },
    ) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const rsp = await api.listSessions({
          page,
          pageSize,
          sort: sortState.dir,
          sortField: sortState.field,
          startTime,
          endTime,
        });
        setSessions(rsp.sessions ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPageInputValue(String(rsp.pageInfo.page));
        }
      } catch {
        // handled silently
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Initial data fetch on mount */
  useEffect(() => {
    fetchSessions(1, 1, "30d", "", "", { field: "created_at", dir: "desc" });
  }, [fetchSessions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const totalPages = Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize));

  const refresh = (page: number, pageSize?: number) =>
    fetchSessions(page, pageSize ?? pageInfo.pageSize, timeRange, customStart, customEnd, sort);

  const handleSort = (field: string) => {
    const newSort: { field: string; dir: SortDir } =
      sort.field === field
        ? { field, dir: sort.dir === "asc" ? "desc" : "asc" }
        : { field, dir: "desc" };
    setSort(newSort);
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, newSort);
  };

  const renderSortIcon = (field: string) => {
    if (sort.field !== field) return null;
    return sort.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">Sessions</h1>
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
              {/* Filters */}
              <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="flex flex-wrap items-center gap-2">
                  <TimeRangePicker
                    value={timeRange}
                    customStart={customStart}
                    customEnd={customEnd}
                    onChange={(key, cs, ce) => {
                      setTimeRange(key);
                      setCustomStart(cs);
                      setCustomEnd(ce);
                      if (key !== "custom") {
                        fetchSessions(1, pageInfo.pageSize, key, cs, ce, sort);
                      }
                    }}
                  />
                </div>
              </div>

              {isMobile ? (
              <div className="space-y-3">
                {sessions.map((s) => (
                  <div
                    key={s.id}
                    className="cursor-pointer rounded-lg border border-border bg-card p-4 transition-colors hover:bg-secondary/50"
                    onClick={() => {
                      window.location.href = `/web/sessions/detail/?id=${s.id}`;
                    }}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">
                          {s.summary || `Session #${s.id}`}
                        </p>
                      </div>
                      <Badge variant="secondary" className="shrink-0 text-xs">
                        {s.messageCount ?? 0} msgs
                      </Badge>
                    </div>
                    <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
                      <span>ID: {s.id}</span>
                      <span>{s.toolCount ?? 0} tools</span>
                      <span>{formatDateTime(s.createdAt)}</span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Summary</TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.messageCount)}
                    >
                      <span className="inline-flex items-center gap-1">Messages {renderSortIcon(SORTABLE_COLUMNS.messageCount)}</span>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.toolCount)}
                    >
                      <span className="inline-flex items-center gap-1">Tools {renderSortIcon(SORTABLE_COLUMNS.toolCount)}</span>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                    >
                      <span className="inline-flex items-center gap-1">Time {renderSortIcon(SORTABLE_COLUMNS.createdAt)}</span>
                    </TableHead>
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
                      <TableCell>{s.messageCount ?? 0}</TableCell>
                      <TableCell>{s.toolCount ?? 0}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDateTime(s.createdAt)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

              {/* Pagination */}
              <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
                <div className="hidden items-center gap-3 md:flex">
                  <DropdownMenu>
                    <DropdownMenuTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
                      <ListFilter className="size-3.5" />
                      {pageInfo.pageSize} / page
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start">
                      {[20, 50, 100, 200].map((size) => (
                        <DropdownMenuItem
                          key={size}
                          onClick={() => fetchSessions(1, size, timeRange, customStart, customEnd, sort)}
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
<p className="hidden text-sm text-muted-foreground md:block">
                      {pageInfo.total} session{pageInfo.total !== 1 ? "s" : ""} total
                    </p>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page <= 1}
                    onClick={() => refresh(pageInfo.page - 1)}
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
                          refresh(page);
                        }
                      }}
                      onBlur={() => {
                        let page = parseInt(pageInputValue, 10);
                        if (Number.isNaN(page)) page = 1;
                        page = Math.max(1, Math.min(page, totalPages));
                        refresh(page);
                      }}
                    />
                    <span className="text-muted-foreground">/ {totalPages}</span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page >= totalPages}
                    onClick={() => refresh(pageInfo.page + 1)}
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
