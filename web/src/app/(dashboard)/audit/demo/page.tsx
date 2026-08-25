"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { DemoAccessAuditItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Footprints } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { useI18n } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export default function DemoAccessAuditPage() {
  const { t, locale } = useI18n();
  const actionLabelMap: Record<string, string> = {
    login: t("demo_access_audit.action_login"),
    login_denied: t("demo_access_audit.action_login_denied"),
    module_access: t("demo_access_audit.action_module_access"),
    module_denied: t("demo_access_audit.action_module_denied"),
  };
  const reasonLabelMap: Record<string, string> = {
    login_disabled: t("demo_access_audit.reason_login_disabled"),
    no_demo_user: t("demo_access_audit.reason_no_demo_user"),
    module_closed: t("demo_access_audit.reason_module_closed"),
  };

  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.demoAccessAudit.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.demoAccessAudit.pageSize",
    20,
  );
  const [logs, setLogs] = useState<DemoAccessAuditItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");

  const fetchOptionsFor = useCallback(
    (field: "action" | "module") => async () => {
      const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.listDemoAccessAuditOptions({ field, startTime, endTime });
      return rsp.items ?? [];
    },
    [timeRange, customStart, customEnd],
  );

  const facets = useMemo<FacetDef[]>(
    () => [
      {
        key: "action",
        label: t("demo_access_audit.filter_action"),
        options: fetchOptionsFor("action"),
        formatValue: (v) => actionLabelMap[v] ?? v,
      },
      {
        key: "module",
        label: t("demo_access_audit.filter_module"),
        options: fetchOptionsFor("module"),
      },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, fetchOptionsFor],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.demoAccessAudit",
    facets,
    freeTextPlaceholder: t("demo_access_audit.search_placeholder"),
    optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
  });
  const { queryParams } = filterBar;

  interface DemoAuditQuery {
    page: number;
    pageSize: number;
    range: TimeRangeKey;
    cs: string;
    ce: string;
    qp: FilterBarQueryParams;
  }

  const fetchLogs = useCallback(
    async (q: DemoAuditQuery) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(q.range, q.cs, q.ce);
        const rsp = await api.listDemoAccessAudits({
          page: q.page,
          pageSize: q.pageSize,
          query: q.qp.freeText || undefined,
          sort: "desc",
          sortField: "created_at",
          startTime,
          endTime,
          filter: q.qp.filter,
        });
        if (rsp.error) {
          showErrorToast(rsp.error, { title: t("common.error") });
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        showErrorToast(err, { title: t("common.error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t],
  );

  const currentQuery = (): Omit<DemoAuditQuery, "page" | "pageSize"> => ({
    range: timeRange,
    cs: customStart,
    ce: customEnd,
    qp: queryParams,
  });

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- token 变化回到第 1 页查询；挂载时以持久化筛选发起首次查询 */
  useEffect(() => {
    fetchLogs({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery() });
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs({ page, pageSize: pageSize ?? pageInfo.pageSize, ...currentQuery() });

  const actionBadgeVariant = (action: string) => {
    switch (action) {
      case "login":
        return "secondary" as const;
      case "module_access":
        return "default" as const;
      case "login_denied":
      case "module_denied":
        return "destructive" as const;
      default:
        return "secondary" as const;
    }
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("demo_access_audit.page_title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {t("demo_access_audit.page_subtitle")}
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("demo_access_audit.logs_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            {/* Filters — faceted bar */}
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center">
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                  fetchLogs({
                    page: 1,
                    pageSize: pageInfo.pageSize,
                    range: key,
                    cs,
                    ce,
                    qp: queryParams,
                  });
                }}
              />
              <FilterBar
                {...filterBar}
                facets={facets}
                placeholder={t("demo_access_audit.search_placeholder")}
              />
            </div>

            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : logs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Footprints className="mb-3 size-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">{t("demo_access_audit.no_logs")}</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("demo_access_audit.time")}</TableHead>
                    <TableHead>{t("demo_access_audit.action")}</TableHead>
                    <TableHead>{t("demo_access_audit.module")}</TableHead>
                    <TableHead>{t("demo_access_audit.path")}</TableHead>
                    <TableHead>{t("demo_access_audit.ip")}</TableHead>
                    <TableHead>{t("demo_access_audit.user_agent")}</TableHead>
                    <TableHead>{t("demo_access_audit.reason")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow
                      key={log.id}
                      className={
                        log.action === "login_denied" || log.action === "module_denied"
                          ? "bg-destructive/5"
                          : ""
                      }
                    >
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatTime(log.createdAt)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={actionBadgeVariant(log.action)} className="text-xs">
                          {actionLabelMap[log.action] ?? log.action}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{log.module || "—"}</TableCell>
                      <TableCell className="max-w-[220px] font-mono text-xs">
                        <TooltipRoot>
                          <TooltipTrigger
                            render={
                              <span className="block truncate text-left">{log.path || "—"}</span>
                            }
                          />
                          <TooltipContent side="top" className="max-w-xs break-all">
                            {log.path || "—"}
                          </TooltipContent>
                        </TooltipRoot>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {log.ip || "—"}
                      </TableCell>
                      <TableCell className="max-w-[180px] text-xs text-muted-foreground">
                        <TooltipRoot>
                          <TooltipTrigger
                            render={
                              <span className="block truncate text-left">
                                {log.userAgent || "—"}
                              </span>
                            }
                          />
                          <TooltipContent side="top" className="max-w-xs break-all">
                            {log.userAgent || "—"}
                          </TooltipContent>
                        </TooltipRoot>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {log.reason ? (reasonLabelMap[log.reason] ?? log.reason) : "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <PaginationBar
              pageInfo={pageInfo}
              onChange={(page, pageSize) => refresh(page, pageSize)}
              totalLabel={t("pagination.logs")}
            />
          </CardContent>
        </Card>
      </div>
    </PermissionGuard>
  );
}
